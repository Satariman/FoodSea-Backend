//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/foodsea/ordering/internal/modules/orders"
	"github.com/foodsea/ordering/internal/modules/saga"
	"github.com/foodsea/ordering/internal/platform/httputil"
	"github.com/foodsea/ordering/internal/platform/kafka"
	pb_core "github.com/foodsea/proto/core"
	pb_opt "github.com/foodsea/proto/optimization"
)

type placeOrderReq struct {
	OptimizationResultID uuid.UUID `json:"optimization_result_id"`
}

type placeOrderResp struct {
	OrderID uuid.UUID `json:"order_id"`
	Status  string    `json:"status"`
}

func placeOrder(t *testing.T, userID uuid.UUID, optResult *pb_opt.OptimizationResultProto) (uuid.UUID, int) {
	t.Helper()
	resultID, err := uuid.Parse(optResult.Id)
	require.NoError(t, err)
	token := makeJWT(userID)

	resp, err := postJSONAuth(testBaseURL+"/api/v1/orders", token, placeOrderReq{
		OptimizationResultID: resultID,
	})
	require.NoError(t, err)
	statusCode := resp.StatusCode

	if statusCode == http.StatusCreated {
		var envelope httputil.Response
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&envelope))
		resp.Body.Close()
		dataBytes, _ := json.Marshal(envelope.Data)
		var body placeOrderResp
		require.NoError(t, json.Unmarshal(dataBytes, &body))
		return body.OrderID, statusCode
	}
	resp.Body.Close()
	return uuid.Nil, statusCode
}

func TestSaga_HappyPath(t *testing.T) {
	cartMock.reset()
	optMock.reset()

	userID := uuid.New()
	optResult := buildOptResult(userID)
	optMock.addResult(optResult)

	orderID, code := placeOrder(t, userID, optResult)
	assert.Equal(t, http.StatusCreated, code)
	assert.NotEqual(t, uuid.Nil, orderID)

	ctx := context.Background()

	// DB: order confirmed.
	ord, err := testEntClient.Order.Get(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", ord.Status)

	// DB: saga completed at step 4.
	sagas, err := testEntClient.SagaState.Query().All(ctx)
	require.NoError(t, err)
	var sagaFound bool
	for _, s := range sagas {
		if s.OrderID == orderID {
			assert.Equal(t, "completed", string(s.Status))
			assert.Equal(t, int8(4), s.CurrentStep)
			sagaFound = true
		}
	}
	assert.True(t, sagaFound, "saga state not found for order")

	// DB: 2 order items.
	items, err := testEntClient.OrderItem.Query().All(ctx)
	require.NoError(t, err)
	itemCount := 0
	for _, it := range items {
		if it.OrderID == orderID {
			itemCount++
		}
	}
	assert.Equal(t, 2, itemCount)

	// DB: 2 status history entries (created → confirmed).
	history, err := testEntClient.OrderStatusHistory.Query().All(ctx)
	require.NoError(t, err)
	histCount := 0
	for _, h := range history {
		if h.OrderID == orderID {
			histCount++
		}
	}
	assert.Equal(t, 2, histCount)

	// Mock: ClearCart called once, RestoreCart never.
	cartMock.mu.Lock()
	assert.Equal(t, 1, cartMock.clearCartCalls)
	assert.Equal(t, 0, cartMock.restoreCartCalls)
	cartMock.mu.Unlock()

	// Mock: LockResult once, UnlockResult never.
	optMock.mu.Lock()
	assert.Equal(t, 1, optMock.lockCalls)
	assert.Equal(t, 0, optMock.unlockCalls)
	optMock.mu.Unlock()
}

func TestSaga_CompensationClearCartFail(t *testing.T) {
	cartMock.reset()
	optMock.reset()

	userID := uuid.New()
	optResult := buildOptResult(userID)
	optMock.addResult(optResult)

	// Make ClearCart fail with codes.Internal.
	cartMock.setClearCartErr(status.Error(codes.Internal, "cart service error"))

	_, code := placeOrder(t, userID, optResult)
	assert.GreaterOrEqual(t, code, 400, "expected error response when ClearCart fails")

	ctx := context.Background()

	// DB: order cancelled.
	ords, err := testEntClient.Order.Query().All(ctx)
	require.NoError(t, err)
	var orderCancelled bool
	for _, o := range ords {
		if o.UserID == userID {
			assert.Equal(t, "cancelled", o.Status, "order should be cancelled")
			orderCancelled = true
		}
	}
	assert.True(t, orderCancelled, "no order found for user")

	// DB: saga failed.
	sagas, err := testEntClient.SagaState.Query().All(ctx)
	require.NoError(t, err)
	var sagaFailed bool
	for _, s := range sagas {
		if s.UserID == userID {
			assert.Equal(t, "failed", string(s.Status))
			sagaFailed = true
		}
	}
	assert.True(t, sagaFailed, "saga should be in failed state")

	// Mock: UnlockResult called for compensation.
	optMock.mu.Lock()
	assert.GreaterOrEqual(t, optMock.unlockCalls, 1, "UnlockResult should be called in compensation")
	optMock.mu.Unlock()

	// ClearCart failed without completing — RestoreCart should NOT be called.
	cartMock.mu.Lock()
	assert.GreaterOrEqual(t, cartMock.clearCartCalls, 1, "ClearCart should have been attempted")
	assert.Equal(t, 0, cartMock.restoreCartCalls, "RestoreCart should NOT be called")
	cartMock.mu.Unlock()
}

func TestSaga_Recovery(t *testing.T) {
	cartMock.reset()
	optMock.reset()

	ctx := context.Background()
	userID := uuid.New()
	optResult := buildOptResult(userID)
	optMock.addResult(optResult)
	optResultID, _ := uuid.Parse(optResult.Id)

	// Pre-create a pending order (step 2 completed).
	orderID := uuid.New()
	_, err := testEntClient.Order.Create().
		SetID(orderID).
		SetUserID(userID).
		SetOptimizationResultID(optResultID).
		SetStatus("created").
		SetTotalKopecks(300_000).
		SetDeliveryKopecks(15_000).
		Save(ctx)
	require.NoError(t, err)

	// Create order items (snapshot).
	for _, it := range optResult.Items {
		pid, _ := uuid.Parse(it.ProductId)
		sid, _ := uuid.Parse(it.StoreId)
		_, err = testEntClient.OrderItem.Create().
			SetOrderID(orderID).
			SetProductID(pid).
			SetProductName("").
			SetStoreID(sid).
			SetStoreName(it.StoreName).
			SetQuantity(int16(it.Quantity)).
			SetPriceKopecks(it.PriceKopecks).
			Save(ctx)
		require.NoError(t, err)
	}

	// Insert saga_state at step=2 (order created, ClearCart not yet done).
	sagaID := uuid.New()
	sagaPayload := map[string]any{
		"optimization_result_id": optResultID.String(),
		"order_id":               orderID.String(),
		"total_kopecks":          int64(300_000),
		"delivery_kopecks":       int64(15_000),
		"items":                  []any{},
	}
	_, err = testEntClient.SagaState.Create().
		SetID(sagaID).
		SetOrderID(orderID).
		SetUserID(userID).
		SetCurrentStep(2).
		SetStatus("pending").
		SetPayload(sagaPayload).
		Save(ctx)
	require.NoError(t, err)

	// Build a fresh saga module for recovery (mirrors what RecoverPending does at startup).
	brokers := []string{testKafkaBroker}
	rOrderProd := kafka.NewProducer(brokers, "order.events", testLog)
	defer rOrderProd.Close()
	rCmdProd := kafka.NewProducer(brokers, "saga.commands", testLog)
	defer rCmdProd.Close()
	rReplyProd := kafka.NewProducer(brokers, "saga.replies", testLog)
	defer rReplyProd.Close()
	rReplyCons := kafka.NewConsumer(brokers, "saga.replies", "ordering-saga-recovery-"+sagaID.String(), testLog)
	defer rReplyCons.Close()

	ordMod := orders.NewModule(orders.Deps{Ent: testEntClient, Producer: rOrderProd, Log: testLog})
	recoveryMod := saga.NewModule(saga.Deps{
		Ent:             testEntClient,
		OrdersFacade:    ordMod.OrderFacade(),
		CartClient:      pb_core.NewCartServiceClient(testCartConn),
		OptClient:       pb_opt.NewOptimizationServiceClient(testOptConn),
		CommandProducer: rCmdProd,
		ReplyProducer:   rReplyProd,
		ReplyConsumer:   rReplyCons,
		Log:             testLog,
		StepTimeout:     10 * time.Second,
		MaxCompAttempts: 3,
	})

	err = recoveryMod.RecoverPending(ctx)
	require.NoError(t, err)

	// Small wait for DB state to settle.
	time.Sleep(300 * time.Millisecond)

	// DB: saga completed, step 4.
	s, err := testEntClient.SagaState.Get(ctx, sagaID)
	require.NoError(t, err)
	assert.Equal(t, "completed", string(s.Status))
	assert.Equal(t, int8(4), s.CurrentStep)

	// DB: order confirmed.
	o, err := testEntClient.Order.Get(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", o.Status)

	// ClearCart called once (step 3), LockResult NOT called (step 1 already done).
	cartMock.mu.Lock()
	assert.Equal(t, 1, cartMock.clearCartCalls, "ClearCart should be called once during recovery")
	cartMock.mu.Unlock()

	optMock.mu.Lock()
	assert.Equal(t, 0, optMock.lockCalls, "LockResult should NOT be called during recovery")
	optMock.mu.Unlock()
}

func TestSaga_OrderListAndDetail(t *testing.T) {
	cartMock.reset()
	optMock.reset()

	userID := uuid.New()
	optResult := buildOptResult(userID)
	optMock.addResult(optResult)

	orderID, code := placeOrder(t, userID, optResult)
	require.Equal(t, http.StatusCreated, code)

	token := makeJWT(userID)

	// GET /api/v1/orders.
	listResp, err := getAuth(testBaseURL+"/api/v1/orders", token)
	require.NoError(t, err)
	defer listResp.Body.Close()
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var listEnv httputil.Response
	require.NoError(t, decodeJSON(listResp, &listEnv))
	require.NotNil(t, listEnv.Data)
	dataBytes, _ := json.Marshal(listEnv.Data)
	var orderList []map[string]any
	require.NoError(t, json.Unmarshal(dataBytes, &orderList))
	assert.GreaterOrEqual(t, len(orderList), 1)

	// GET /api/v1/orders/:id.
	detailResp, err := getAuth(testBaseURL+"/api/v1/orders/"+orderID.String(), token)
	require.NoError(t, err)
	defer detailResp.Body.Close()
	assert.Equal(t, http.StatusOK, detailResp.StatusCode)

	var detailEnv httputil.Response
	require.NoError(t, decodeJSON(detailResp, &detailEnv))
	require.NotNil(t, detailEnv.Data)
	dataBytes, _ = json.Marshal(detailEnv.Data)
	var detail map[string]any
	require.NoError(t, json.Unmarshal(dataBytes, &detail))
	assert.Equal(t, orderID.String(), detail["id"])

	items, ok := detail["items"].([]any)
	require.True(t, ok, "items should be an array")
	assert.Equal(t, 2, len(items))
}

func TestSaga_TransientRetry(t *testing.T) {
	cartMock.reset()
	optMock.reset()

	userID := uuid.New()
	optResult := buildOptResult(userID)
	optMock.addResult(optResult)

	// LockResult returns Unavailable for first 2 calls, then succeeds.
	optMock.mu.Lock()
	optMock.lockErr = status.Error(codes.Unavailable, "temporary unavailable")
	optMock.mu.Unlock()

	go func() {
		for {
			optMock.mu.Lock()
			calls := optMock.lockCalls
			optMock.mu.Unlock()
			if calls >= 2 {
				optMock.mu.Lock()
				optMock.lockErr = nil
				optMock.mu.Unlock()
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	start := time.Now()
	_, code := placeOrder(t, userID, optResult)
	assert.Equal(t, http.StatusCreated, code, "saga should succeed after retries")
	assert.Less(t, time.Since(start), 30*time.Second, "saga should complete within timeout")
}
