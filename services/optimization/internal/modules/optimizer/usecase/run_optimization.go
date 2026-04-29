package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	analogsdomain "github.com/foodsea/optimization/internal/modules/analogs/domain"
	"github.com/foodsea/optimization/internal/modules/optimizer/algorithm"
	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	"github.com/foodsea/optimization/internal/platform/cache"
	pbcore "github.com/foodsea/proto/core"
)

// RunOptimization executes full optimization flow for a user.
type RunOptimization struct {
	cartClient     pbcore.CartServiceClient
	offerClient    pbcore.OfferServiceClient
	analogProvider analogsdomain.AnalogProvider
	algorithm      *algorithm.Optimizer
	repo           domain.ResultRepository
	events         domain.OptimizationEventPublisher
	cache          cache.Cache
	timeout        time.Duration
	log            *slog.Logger
}

func NewRunOptimization(
	cartClient pbcore.CartServiceClient,
	offerClient pbcore.OfferServiceClient,
	analogProvider analogsdomain.AnalogProvider,
	algo *algorithm.Optimizer,
	repo domain.ResultRepository,
	events domain.OptimizationEventPublisher,
	cache cache.Cache,
	timeout time.Duration,
	log *slog.Logger,
) *RunOptimization {
	return &RunOptimization{
		cartClient:     cartClient,
		offerClient:    offerClient,
		analogProvider: analogProvider,
		algorithm:      algo,
		repo:           repo,
		events:         events,
		cache:          cache,
		timeout:        timeout,
		log:            log,
	}
}

func (uc *RunOptimization) Execute(ctx context.Context, userID uuid.UUID) (*domain.OptimizationResult, error) {
	cartResp, err := uc.cartClient.GetCartItems(ctx, &pbcore.GetCartItemsRequest{UserId: userID.String()})
	if err != nil {
		return nil, fmt.Errorf("getting cart items: %w", err)
	}

	items := make([]domain.CartItem, 0, len(cartResp.GetItems()))
	productIDs := make([]uuid.UUID, 0, len(cartResp.GetItems()))
	for _, item := range cartResp.GetItems() {
		productID, parseErr := uuid.Parse(item.GetProductId())
		if parseErr != nil {
			uc.log.WarnContext(ctx, "skipping cart item with invalid product_id", "product_id", item.GetProductId(), "error", parseErr)
			continue
		}
		items = append(items, domain.CartItem{
			ProductID:   productID,
			ProductName: item.GetProductName(),
			Quantity:    int(item.GetQuantity()),
		})
		productIDs = append(productIDs, productID)
	}

	if len(items) == 0 {
		return nil, domain.ErrEmptyCart
	}

	offersResp, err := uc.offerClient.GetOffers(ctx, &pbcore.GetOffersRequest{ProductIds: uuidsToStrings(productIDs)})
	if err != nil {
		return nil, fmt.Errorf("getting offers: %w", err)
	}

	prices := make(map[domain.ProductID]map[domain.StoreID]int64)
	storeNames := make(map[domain.StoreID]string)
	storesSet := make(map[domain.StoreID]struct{})

	for _, offer := range offersResp.GetOffers() {
		if !offer.GetInStock() {
			continue
		}
		productID, pErr := uuid.Parse(offer.GetProductId())
		storeID, sErr := uuid.Parse(offer.GetStoreId())
		if pErr != nil || sErr != nil {
			uc.log.WarnContext(ctx, "skipping invalid offer ids", "product_id", offer.GetProductId(), "store_id", offer.GetStoreId())
			continue
		}
		if _, ok := prices[productID]; !ok {
			prices[productID] = make(map[domain.StoreID]int64)
		}
		prices[productID][storeID] = offer.GetPriceKopecks()
		storeNames[storeID] = offer.GetStoreName()
		storesSet[storeID] = struct{}{}
	}

	if len(storesSet) == 0 {
		return nil, domain.ErrNoOffers
	}

	storeIDs := make([]uuid.UUID, 0, len(storesSet))
	for storeID := range storesSet {
		storeIDs = append(storeIDs, storeID)
	}

	deliveryResp, err := uc.offerClient.GetDeliveryConditions(ctx, &pbcore.GetDeliveryConditionsRequest{StoreIds: uuidsToStrings(storeIDs)})
	if err != nil {
		return nil, fmt.Errorf("getting delivery conditions: %w", err)
	}

	delivery := make(map[domain.StoreID]domain.DeliveryCondition, len(deliveryResp.GetConditions()))
	for _, cond := range deliveryResp.GetConditions() {
		storeID, parseErr := uuid.Parse(cond.GetStoreId())
		if parseErr != nil {
			uc.log.WarnContext(ctx, "skipping invalid store id in delivery condition", "store_id", cond.GetStoreId(), "error", parseErr)
			continue
		}
		var freeFrom *int64
		if cond.FreeFromKopecks != nil {
			v := cond.GetFreeFromKopecks()
			freeFrom = &v
		}
		delivery[storeID] = domain.DeliveryCondition{
			MinOrderKopecks:     cond.GetMinOrderKopecks(),
			DeliveryCostKopecks: cond.GetDeliveryCostKopecks(),
			FreeFromKopecks:     freeFrom,
		}
	}
	if missing := missingDeliveryStoreIDs(storesSet, delivery); len(missing) > 0 {
		return nil, fmt.Errorf("%w: %v", domain.ErrDeliveryIncomplete, uuidsToStrings(missing))
	}

	cartHash := hashCart(items)
	cached, err := uc.repo.FindByCartHash(ctx, cartHash)
	if err != nil && !errors.Is(err, domain.ErrResultNotFound) {
		return nil, err
	}
	if errors.Is(err, domain.ErrResultNotFound) {
		cached = nil
	}
	if cached != nil && cached.UserID == userID {
		return cached, nil
	}

	batchAnalogs, err := uc.analogProvider.GetBatchAnalogsForStores(ctx, productIDs, 5, storeIDs)
	if err != nil {
		uc.log.WarnContext(ctx, "failed to fetch batch analogs", "error", err)
	}

	analogs := make(map[domain.ProductID][]domain.Analog, len(batchAnalogs))
	for productID, candidates := range batchAnalogs {
		mapped := make([]domain.Analog, 0, len(candidates))
		for _, candidate := range candidates {
			mapped = append(mapped, domain.Analog{
				ProductID:   candidate.ProductID,
				ProductName: candidate.ProductName,
				Score:       candidate.Score,
			})
		}
		analogs[productID] = mapped
	}

	analogProductIDs := collectAnalogProductIDs(batchAnalogs, prices)
	if len(analogProductIDs) > 0 {
		analogOffersResp, getOffersErr := uc.offerClient.GetOffers(ctx, &pbcore.GetOffersRequest{ProductIds: uuidsToStrings(analogProductIDs)})
		if getOffersErr != nil {
			uc.log.WarnContext(ctx, "failed to fetch offers for analog products", "error", getOffersErr, "count", len(analogProductIDs))
		} else {
			mergeOffers(ctx, uc.log, analogOffersResp.GetOffers(), prices, storeNames, storesSet)
		}
	}

	input := domain.Input{
		UserID:     userID,
		Items:      items,
		Stores:     storeIDs,
		StoreNames: storeNames,
		Prices:     prices,
		Delivery:   delivery,
		Analogs:    analogs,
	}

	runCtx, cancel := context.WithTimeout(ctx, uc.timeout)
	defer cancel()

	algoResult, err := uc.algorithm.Optimize(runCtx, &input)
	if err != nil {
		return nil, err
	}

	optResult := &domain.OptimizationResult{
		ID:              uuid.New(),
		UserID:          userID,
		CartHash:        cartHash,
		TotalKopecks:    algoResult.TotalKopecks,
		DeliveryKopecks: algoResult.DeliveryKopecks,
		SavingsKopecks:  algoResult.SavingsKopecks,
		Status:          "active",
		IsApproximate:   algoResult.IsApproximate,
		Items:           algoResult.Assignments,
		Substitutions:   algoResult.Substitutions,
	}

	if err := uc.repo.Save(ctx, optResult); err != nil {
		return nil, err
	}

	if err = uc.events.ResultCreated(ctx, optResult); err != nil {
		uc.log.WarnContext(ctx, "failed to publish result created event", "result_id", optResult.ID, "error", err)
	}

	return optResult, nil
}

func missingDeliveryStoreIDs(stores map[domain.StoreID]struct{}, delivery map[domain.StoreID]domain.DeliveryCondition) []uuid.UUID {
	missing := make([]uuid.UUID, 0)
	for storeID := range stores {
		if _, ok := delivery[storeID]; ok {
			continue
		}
		missing = append(missing, storeID)
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i].String() < missing[j].String() })
	return missing
}

func collectAnalogProductIDs(
	batchAnalogs map[uuid.UUID][]analogsdomain.Analog,
	prices map[domain.ProductID]map[domain.StoreID]int64,
) []uuid.UUID {
	set := make(map[uuid.UUID]struct{})
	for _, candidates := range batchAnalogs {
		for _, candidate := range candidates {
			if _, alreadyLoaded := prices[candidate.ProductID]; alreadyLoaded {
				continue
			}
			set[candidate.ProductID] = struct{}{}
		}
	}

	out := make([]uuid.UUID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func mergeOffers(
	ctx context.Context,
	log *slog.Logger,
	offers []*pbcore.OfferProto,
	prices map[domain.ProductID]map[domain.StoreID]int64,
	storeNames map[domain.StoreID]string,
	storesSet map[domain.StoreID]struct{},
) {
	for _, offer := range offers {
		if !offer.GetInStock() {
			continue
		}
		productID, pErr := uuid.Parse(offer.GetProductId())
		storeID, sErr := uuid.Parse(offer.GetStoreId())
		if pErr != nil || sErr != nil {
			log.WarnContext(ctx, "skipping invalid offer ids", "product_id", offer.GetProductId(), "store_id", offer.GetStoreId())
			continue
		}
		if _, ok := prices[productID]; !ok {
			prices[productID] = make(map[domain.StoreID]int64)
		}
		prices[productID][storeID] = offer.GetPriceKopecks()
		storeNames[storeID] = offer.GetStoreName()
		storesSet[storeID] = struct{}{}
	}
}

func hashCart(items []domain.CartItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s:%d", item.ProductID.String(), item.Quantity))
	}
	sort.Strings(parts)
	payload := strings.Join(parts, "|")
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i := range ids {
		out[i] = ids[i].String()
	}
	return out
}
