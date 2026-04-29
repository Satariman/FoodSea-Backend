package algorithm

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

func TestOptimizer_BasicCase(t *testing.T) {
	o := New()

	p1 := mustUUID("11111111-1111-1111-1111-111111111111")
	p2 := mustUUID("22222222-2222-2222-2222-222222222222")
	p3 := mustUUID("33333333-3333-3333-3333-333333333333")
	s1 := mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	s2 := mustUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	input := domain.Input{
		Items: []domain.CartItem{
			{ProductID: p1, ProductName: "P1", Quantity: 1},
			{ProductID: p2, ProductName: "P2", Quantity: 1},
			{ProductID: p3, ProductName: "P3", Quantity: 2},
		},
		Prices: map[domain.ProductID]map[domain.StoreID]int64{
			p1: {s1: 100, s2: 120},
			p2: {s1: 250, s2: 200},
			p3: {s1: 90, s2: 110},
		},
		Delivery: map[domain.StoreID]domain.DeliveryCondition{
			s1: {MinOrderKopecks: 0, DeliveryCostKopecks: 0},
			s2: {MinOrderKopecks: 0, DeliveryCostKopecks: 0},
		},
		StoreNames: map[domain.StoreID]string{s1: "S1", s2: "S2"},
	}

	res, err := o.Optimize(context.Background(), &input)
	require.NoError(t, err)
	require.Len(t, res.Assignments, 3)
	require.Equal(t, int64(480), res.TotalKopecks)

	assignmentByProduct := map[uuid.UUID]domain.Assignment{}
	for _, a := range res.Assignments {
		assignmentByProduct[a.ProductID] = a
	}
	require.Equal(t, s1, assignmentByProduct[p1].StoreID)
	require.Equal(t, s2, assignmentByProduct[p2].StoreID)
	require.Equal(t, s1, assignmentByProduct[p3].StoreID)
}

func TestOptimizer_OneStore(t *testing.T) {
	o := New()
	p := mustUUID("44444444-4444-4444-4444-444444444444")
	s := mustUUID("cccccccc-cccc-cccc-cccc-cccccccccccc")

	input := domain.Input{
		Items:  []domain.CartItem{{ProductID: p, ProductName: "P", Quantity: 3}},
		Prices: map[domain.ProductID]map[domain.StoreID]int64{p: {s: 99}},
		Delivery: map[domain.StoreID]domain.DeliveryCondition{
			s: {MinOrderKopecks: 0, DeliveryCostKopecks: 50},
		},
		StoreNames: map[domain.StoreID]string{s: "Solo"},
	}
	res, err := o.Optimize(context.Background(), &input)

	require.NoError(t, err)
	require.Len(t, res.Assignments, 1)
	require.Equal(t, s, res.Assignments[0].StoreID)
	require.Equal(t, int64(347), res.TotalKopecks)
}

func TestConsolidateMultiMove_SingleMoveImprovement(t *testing.T) {
	pA := mustUUID("55555555-5555-5555-5555-555555555555")
	pB := mustUUID("66666666-6666-6666-6666-666666666666")
	x := mustUUID("dddddddd-dddd-dddd-dddd-dddddddddddd")
	y := mustUUID("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	free := int64(500)

	input := domain.Input{
		Items: []domain.CartItem{
			{ProductID: pA, ProductName: "A", Quantity: 1},
			{ProductID: pB, ProductName: "B", Quantity: 1},
		},
		Prices: map[domain.ProductID]map[domain.StoreID]int64{
			pA: {x: 100, y: 130},
			pB: {x: 470, y: 450},
		},
		Delivery: map[domain.StoreID]domain.DeliveryCondition{
			x: {MinOrderKopecks: 0, DeliveryCostKopecks: 300},
			y: {MinOrderKopecks: 0, DeliveryCostKopecks: 300, FreeFromKopecks: &free},
		},
		StoreNames: map[domain.StoreID]string{x: "X", y: "Y"},
	}

	assignments := []domain.Assignment{
		{ProductID: pA, StoreID: x, StoreName: "X", Price: 100, Quantity: 1},
		{ProductID: pB, StoreID: y, StoreName: "Y", Price: 450, Quantity: 1},
	}
	storeTotals := map[domain.StoreID]int64{x: 100, y: 450}

	newAssignments, newTotals := consolidateMultiMove(assignments, storeTotals, &input, []domain.StoreID{x, y})
	require.Equal(t, int64(580), newTotals[y])
	require.NotContains(t, newTotals, x)
	require.Equal(t, y, newAssignments[0].StoreID)
}

func TestConsolidateMultiMove_MultiMoveRequired(t *testing.T) {
	pA := mustUUID("77777777-7777-7777-7777-777777777777")
	pB := mustUUID("88888888-8888-8888-8888-888888888888")
	pC := mustUUID("99999999-9999-9999-9999-999999999999")
	x := mustUUID("ffffffff-ffff-ffff-ffff-ffffffffffff")
	y := mustUUID("12121212-1212-1212-1212-121212121212")
	free := int64(500)

	input := domain.Input{
		Prices: map[domain.ProductID]map[domain.StoreID]int64{
			pA: {x: 50, y: 100},
			pB: {x: 400, y: 300},
			pC: {x: 50, y: 100},
		},
		Delivery: map[domain.StoreID]domain.DeliveryCondition{
			x: {MinOrderKopecks: 0, DeliveryCostKopecks: 200},
			y: {MinOrderKopecks: 0, DeliveryCostKopecks: 200, FreeFromKopecks: &free},
		},
		StoreNames: map[domain.StoreID]string{x: "X", y: "Y"},
	}

	assignments := []domain.Assignment{
		{ProductID: pA, StoreID: x, StoreName: "X", Price: 50, Quantity: 1},
		{ProductID: pB, StoreID: y, StoreName: "Y", Price: 300, Quantity: 1},
		{ProductID: pC, StoreID: x, StoreName: "X", Price: 50, Quantity: 1},
	}
	storeTotals := map[domain.StoreID]int64{x: 100, y: 300}
	oldDelivery := calculateDelivery(storeTotals, input.Delivery)

	newAssignments, newTotals := consolidateMultiMove(assignments, storeTotals, &input, []domain.StoreID{x, y})
	newDelivery := calculateDelivery(newTotals, input.Delivery)
	require.Less(t, newDelivery, oldDelivery)
	require.Equal(t, y, newAssignments[0].StoreID)
	require.Equal(t, y, newAssignments[2].StoreID)
}

func TestOptimizer_AnalogsCrossStoreAndNoAssignmentMutation(t *testing.T) {
	o := New()
	p := mustUUID("13131313-1313-1313-1313-131313131313")
	q := mustUUID("14141414-1414-1414-1414-141414141414")
	analog := mustUUID("15151515-1515-1515-1515-151515151515")
	a := mustUUID("16161616-1616-1616-1616-161616161616")
	b := mustUUID("17171717-1717-1717-1717-171717171717")
	free := int64(500)

	input := domain.Input{
		Items: []domain.CartItem{
			{ProductID: p, ProductName: "Original", Quantity: 1},
			{ProductID: q, ProductName: "Q", Quantity: 1},
		},
		Prices: map[domain.ProductID]map[domain.StoreID]int64{
			p:      {a: 500},
			q:      {b: 150},
			analog: {b: 480},
		},
		Delivery: map[domain.StoreID]domain.DeliveryCondition{
			a: {MinOrderKopecks: 0, DeliveryCostKopecks: 300, FreeFromKopecks: &free},
			b: {MinOrderKopecks: 0, DeliveryCostKopecks: 300, FreeFromKopecks: &free},
		},
		Analogs: map[domain.ProductID][]domain.Analog{
			p: {{ProductID: analog, ProductName: "Analog", Score: 0.92}},
		},
		StoreNames: map[domain.StoreID]string{a: "A", b: "B"},
	}

	res, err := o.Optimize(context.Background(), &input)
	require.NoError(t, err)
	require.Len(t, res.Substitutions, 1)
	require.True(t, res.Substitutions[0].IsCrossStore)
	require.Greater(t, res.Substitutions[0].TotalSavingKopecks, int64(0))

	assignmentByProduct := map[uuid.UUID]domain.Assignment{}
	for _, as := range res.Assignments {
		assignmentByProduct[as.ProductID] = as
	}
	require.Equal(t, a, assignmentByProduct[p].StoreID)
}

func TestOptimizer_EmptyCart(t *testing.T) {
	o := New()
	_, err := o.Optimize(context.Background(), &domain.Input{})
	require.ErrorIs(t, err, domain.ErrEmptyCart)
}

func TestOptimizer_NoOffers(t *testing.T) {
	o := New()
	_, err := o.Optimize(context.Background(), &domain.Input{Items: []domain.CartItem{{ProductID: uuid.New(), Quantity: 1}}})
	require.ErrorIs(t, err, domain.ErrNoOffers)
}

func TestOptimizer_TimeoutReturnsApproximate(t *testing.T) {
	o := New()
	product := mustUUID("18181818-1818-1818-1818-181818181818")
	prices := make(map[domain.StoreID]int64)
	delivery := make(map[domain.StoreID]domain.DeliveryCondition)
	storeNames := make(map[domain.StoreID]string)
	for i := 0; i < 15; i++ {
		id := uuid.New()
		prices[id] = int64(100 + i)
		delivery[id] = domain.DeliveryCondition{MinOrderKopecks: 0, DeliveryCostKopecks: 100}
		storeNames[id] = id.String()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(time.Microsecond)
		cancel()
	}()

	input := domain.Input{
		Items:      []domain.CartItem{{ProductID: product, ProductName: "P", Quantity: 1}},
		Prices:     map[domain.ProductID]map[domain.StoreID]int64{product: prices},
		Delivery:   delivery,
		StoreNames: storeNames,
	}
	res, err := o.Optimize(ctx, &input)

	require.NoError(t, err)
	require.True(t, res.IsApproximate)
}

func mustUUID(raw string) uuid.UUID {
	return uuid.MustParse(raw)
}
