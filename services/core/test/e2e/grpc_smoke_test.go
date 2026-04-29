//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/foodsea/proto/core"
)

// dialGRPC opens an insecure gRPC connection to testGRPCAddr.
func dialGRPC(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(testGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

// registerUser registers a user and returns the access token.
func registerUser(t *testing.T, email, password string) string {
	t.Helper()
	resp, err := postJSON(testBaseURL+"/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data struct {
			Access string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, decodeJSON(resp, &body))
	require.NotEmpty(t, body.Data.Access)
	return body.Data.Access
}

func TestGRPCSmoke(t *testing.T) {
	ctx := context.Background()
	conn := dialGRPC(t)
	cartClient := pb.NewCartServiceClient(conn)
	offerClient := pb.NewOfferServiceClient(conn)

	// Register a user via HTTP so they have a known ID.
	access := registerUser(t, "grpc-smoke@foodsea.test", "SuperSecret1!")

	// Get the user ID from /users/me.
	meResp, err := getAuth(testBaseURL+"/api/v1/users/me", access)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, meResp.StatusCode)

	var me struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, decodeJSON(meResp, &me))
	userID := me.Data.ID

	// Add an item via HTTP.
	addResp, err := postJSONAuth(testBaseURL+"/api/v1/cart/items", access, map[string]any{
		"product_id": seededProductID,
		"quantity":   3,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, addResp.StatusCode)
	addResp.Body.Close()

	t.Run("get_cart_items", func(t *testing.T) {
		resp, err := cartClient.GetCartItems(ctx, &pb.GetCartItemsRequest{UserId: userID})
		require.NoError(t, err)
		require.NotEmpty(t, resp.Items)
		assert.Equal(t, seededProductID, resp.Items[0].ProductId)
		assert.EqualValues(t, 3, resp.Items[0].Quantity)
	})

	t.Run("clear_cart", func(t *testing.T) {
		resp, err := cartClient.ClearCart(ctx, &pb.ClearCartRequest{UserId: userID})
		require.NoError(t, err)
		assert.True(t, resp.Success)

		items, err := cartClient.GetCartItems(ctx, &pb.GetCartItemsRequest{UserId: userID})
		require.NoError(t, err)
		assert.Empty(t, items.Items)
	})

	t.Run("get_offers", func(t *testing.T) {
		resp, err := offerClient.GetOffers(ctx, &pb.GetOffersRequest{
			ProductIds: []string{seededProductID},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Offers)
	})

	t.Run("get_delivery_conditions", func(t *testing.T) {
		resp, err := offerClient.GetDeliveryConditions(ctx, &pb.GetDeliveryConditionsRequest{
			StoreIds: []string{seededStoreID},
		})
		require.NoError(t, err)
		// May be empty if no delivery conditions are seeded; just assert no error.
		assert.NotNil(t, resp)
	})
}
