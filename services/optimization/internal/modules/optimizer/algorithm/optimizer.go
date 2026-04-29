package algorithm

import (
	"context"
	"math"
	"sort"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

var errProductNotCovered = domain.ErrNoOffers

// Optimizer runs cart price/delivery optimization.
type Optimizer struct{}

func New() *Optimizer { return &Optimizer{} }

func (o *Optimizer) Optimize(ctx context.Context, input *domain.Input) (*domain.Result, error) {
	if input == nil {
		return nil, domain.ErrEmptyCart
	}

	if len(input.Items) == 0 {
		return nil, domain.ErrEmptyCart
	}

	activeStores := filterStores(input)
	if len(activeStores) == 0 {
		return nil, domain.ErrNoOffers
	}

	if len(activeStores) > 15 {
		activeStores = topStoresByCoverage(activeStores, input, 15)
	}

	m := len(activeStores)
	var bestResult *domain.Result
	bestCost := int64(math.MaxInt64)

	for mask := 1; mask < (1 << m); mask++ {
		if mask&0xFF == 0 {
			select {
			case <-ctx.Done():
				if bestResult != nil {
					bestResult.IsApproximate = true
					return bestResult, nil
				}
				return nil, ctx.Err()
			default:
			}
		}

		subset := maskToStores(mask, activeStores)
		res, err := evaluateSubset(input, subset)
		if err != nil {
			continue
		}
		if res.TotalKopecks < bestCost {
			bestCost = res.TotalKopecks
			bestResult = res
		}
	}

	if bestResult == nil {
		return nil, domain.ErrNoFeasibleSolution
	}

	applyAnalogs(bestResult, input)
	calculateSavings(bestResult, input)

	return bestResult, nil
}

func filterStores(input *domain.Input) []domain.StoreID {
	storeHasItems := make(map[domain.StoreID]bool)
	for _, item := range input.Items {
		if stores, ok := input.Prices[item.ProductID]; ok {
			for storeID := range stores {
				storeHasItems[storeID] = true
			}
		}
	}
	result := make([]domain.StoreID, 0, len(storeHasItems))
	for storeID := range storeHasItems {
		result = append(result, storeID)
	}
	return result
}

func topStoresByCoverage(stores []domain.StoreID, input *domain.Input, k int) []domain.StoreID {
	type storeScore struct {
		id    domain.StoreID
		count int
	}

	scores := make([]storeScore, len(stores))
	for i, storeID := range stores {
		count := 0
		for _, item := range input.Items {
			if prices, ok := input.Prices[item.ProductID]; ok {
				if _, ok = prices[storeID]; ok {
					count++
				}
			}
		}
		scores[i] = storeScore{id: storeID, count: count}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].count > scores[j].count
	})

	if k > len(scores) {
		k = len(scores)
	}
	result := make([]domain.StoreID, k)
	for i := 0; i < k; i++ {
		result[i] = scores[i].id
	}
	return result
}

func maskToStores(mask int, stores []domain.StoreID) []domain.StoreID {
	subset := make([]domain.StoreID, 0, len(stores))
	for i, storeID := range stores {
		if mask&(1<<i) != 0 {
			subset = append(subset, storeID)
		}
	}
	return subset
}

func evaluateSubset(input *domain.Input, stores []domain.StoreID) (*domain.Result, error) {
	storeSet := make(map[domain.StoreID]bool, len(stores))
	for _, s := range stores {
		storeSet[s] = true
	}

	productNames := make(map[domain.ProductID]string, len(input.Items))
	for _, item := range input.Items {
		productNames[item.ProductID] = item.ProductName
	}

	assignments := make([]domain.Assignment, 0, len(input.Items))
	storeTotals := make(map[domain.StoreID]int64)

	for _, item := range input.Items {
		bestStore, bestPrice, found := cheapestInSubset(item.ProductID, storeSet, input.Prices)
		if !found {
			return nil, errProductNotCovered
		}

		totalPrice := bestPrice * int64(item.Quantity)
		assignments = append(assignments, domain.Assignment{
			ProductID:   item.ProductID,
			ProductName: productNames[item.ProductID],
			StoreID:     bestStore,
			StoreName:   input.StoreNames[bestStore],
			Price:       bestPrice,
			Quantity:    item.Quantity,
		})
		storeTotals[bestStore] += totalPrice
	}

	assignments, storeTotals = consolidateMultiMove(assignments, storeTotals, input, stores)

	deliveryCost := calculateDelivery(storeTotals, input.Delivery)
	itemsCost := int64(0)
	for _, total := range storeTotals {
		itemsCost += total
	}

	return &domain.Result{
		Assignments:     assignments,
		TotalKopecks:    itemsCost + deliveryCost,
		DeliveryKopecks: deliveryCost,
	}, nil
}

func cheapestInSubset(
	productID domain.ProductID,
	storeSet map[domain.StoreID]bool,
	prices map[domain.ProductID]map[domain.StoreID]int64,
) (bestStore domain.StoreID, bestPrice int64, found bool) {
	storePrices, ok := prices[productID]
	if !ok {
		return bestStore, 0, false
	}

	bestPrice = int64(math.MaxInt64)

	for storeID, price := range storePrices {
		if !storeSet[storeID] {
			continue
		}
		if price < bestPrice {
			bestPrice = price
			bestStore = storeID
			found = true
		}
	}

	return bestStore, bestPrice, found
}

func calculateDelivery(storeTotals map[domain.StoreID]int64, conditions map[domain.StoreID]domain.DeliveryCondition) int64 {
	var total int64
	for storeID, orderSum := range storeTotals {
		cond, ok := conditions[storeID]
		if !ok {
			continue
		}
		if orderSum < cond.MinOrderKopecks {
			total += cond.DeliveryCostKopecks
			continue
		}
		if cond.FreeFromKopecks != nil && orderSum >= *cond.FreeFromKopecks {
			continue
		}
		total += cond.DeliveryCostKopecks
	}
	return total
}

func consolidateMultiMove(
	assignments []domain.Assignment,
	storeTotals map[domain.StoreID]int64,
	input *domain.Input,
	stores []domain.StoreID,
) (updatedAssignments []domain.Assignment, updatedStoreTotals map[domain.StoreID]int64) {
	improved := true
	for improved {
		improved = false

		for _, targetStore := range stores {
			cond, ok := input.Delivery[targetStore]
			if !ok {
				continue
			}
			if cond.FreeFromKopecks == nil {
				continue
			}

			currentTotal := storeTotals[targetStore]
			threshold := *cond.FreeFromKopecks
			if currentTotal >= threshold {
				continue
			}
			gap := threshold - currentTotal

			type candidate struct {
				assignIdx  int
				fromStore  domain.StoreID
				priceDelta int64
				newPrice   int64
				addedValue int64
			}

			candidates := make([]candidate, 0)
			for i, a := range assignments {
				if a.StoreID == targetStore {
					continue
				}
				altPrice, ok := input.Prices[a.ProductID][targetStore]
				if !ok {
					continue
				}
				qty := int64(a.Quantity)
				delta := (altPrice - a.Price) * qty
				candidates = append(candidates, candidate{
					assignIdx:  i,
					fromStore:  a.StoreID,
					priceDelta: delta,
					newPrice:   altPrice,
					addedValue: altPrice * qty,
				})
			}

			if len(candidates) == 0 {
				continue
			}

			sort.Slice(candidates, func(i, j int) bool {
				if candidates[i].priceDelta <= 0 && candidates[j].priceDelta > 0 {
					return true
				}
				if candidates[i].priceDelta > 0 && candidates[j].priceDelta <= 0 {
					return false
				}
				return candidates[i].addedValue > candidates[j].addedValue
			})

			chosen := make([]candidate, 0, len(candidates))
			addedTotal := int64(0)
			totalPriceDelta := int64(0)

			for _, c := range candidates {
				chosen = append(chosen, c)
				addedTotal += c.addedValue
				totalPriceDelta += c.priceDelta
				if currentTotal+addedTotal >= threshold {
					break
				}
			}

			if currentTotal+addedTotal < threshold || addedTotal < gap {
				continue
			}

			oldDelivery := calculateDelivery(storeTotals, input.Delivery)
			newTotals := copyMap(storeTotals)
			for _, c := range chosen {
				oldItemTotal := assignments[c.assignIdx].Price * int64(assignments[c.assignIdx].Quantity)
				newTotals[c.fromStore] -= oldItemTotal
				if newTotals[c.fromStore] <= 0 {
					delete(newTotals, c.fromStore)
				}
			}
			newTotals[targetStore] = currentTotal + addedTotal

			newDelivery := calculateDelivery(newTotals, input.Delivery)
			deliverySaving := oldDelivery - newDelivery

			if deliverySaving > totalPriceDelta {
				for _, c := range chosen {
					assignments[c.assignIdx].StoreID = targetStore
					assignments[c.assignIdx].StoreName = input.StoreNames[targetStore]
					assignments[c.assignIdx].Price = c.newPrice
				}
				storeTotals = newTotals
				improved = true
				break
			}
		}
	}

	return assignments, storeTotals
}

func applyAnalogs(result *domain.Result, input *domain.Input) {
	storeTotals := buildStoreTotals(result.Assignments)
	currentDelivery := calculateDelivery(storeTotals, input.Delivery)

	for _, assignment := range result.Assignments {
		analogs, ok := input.Analogs[assignment.ProductID]
		if !ok {
			continue
		}

		var bestSub *domain.Substitution

		for _, analog := range analogs {
			analogPrices, ok := input.Prices[analog.ProductID]
			if !ok {
				continue
			}

			for candidateStore, analogPrice := range analogPrices {
				qty := int64(assignment.Quantity)
				oldItemCost := assignment.Price * qty
				newItemCost := analogPrice * qty
				priceDelta := newItemCost - oldItemCost

				deliveryDelta := int64(0)
				isCrossStore := candidateStore != assignment.StoreID
				if isCrossStore {
					simTotals := copyMap(storeTotals)
					simTotals[assignment.StoreID] -= oldItemCost
					if simTotals[assignment.StoreID] <= 0 {
						delete(simTotals, assignment.StoreID)
					}
					simTotals[candidateStore] += newItemCost
					simDelivery := calculateDelivery(simTotals, input.Delivery)
					deliveryDelta = simDelivery - currentDelivery
				}

				totalSaving := -(priceDelta + deliveryDelta)
				if totalSaving <= 0 {
					continue
				}

				sub := domain.Substitution{
					OriginalID:           assignment.ProductID,
					OriginalProductName:  assignment.ProductName,
					AnalogID:             analog.ProductID,
					AnalogProductName:    analog.ProductName,
					OriginalStoreID:      assignment.StoreID,
					NewStoreID:           candidateStore,
					NewStoreName:         input.StoreNames[candidateStore],
					OldPriceKopecks:      assignment.Price,
					NewPriceKopecks:      analogPrice,
					PriceDeltaKopecks:    priceDelta,
					DeliveryDeltaKopecks: deliveryDelta,
					TotalSavingKopecks:   totalSaving,
					Score:                analog.Score,
					IsCrossStore:         isCrossStore,
				}

				if bestSub == nil || totalSaving > bestSub.TotalSavingKopecks {
					bestSub = &sub
				}
			}
		}

		if bestSub != nil {
			result.Substitutions = append(result.Substitutions, *bestSub)
		}
	}

	sort.Slice(result.Substitutions, func(i, j int) bool {
		return result.Substitutions[i].TotalSavingKopecks > result.Substitutions[j].TotalSavingKopecks
	})
}

func buildStoreTotals(assignments []domain.Assignment) map[domain.StoreID]int64 {
	totals := make(map[domain.StoreID]int64)
	for _, assignment := range assignments {
		totals[assignment.StoreID] += assignment.Price * int64(assignment.Quantity)
	}
	return totals
}

func copyMap(m map[domain.StoreID]int64) map[domain.StoreID]int64 {
	cp := make(map[domain.StoreID]int64, len(m))
	for key, value := range m {
		cp[key] = value
	}
	return cp
}

func calculateSavings(result *domain.Result, input *domain.Input) {
	greedyTotal := int64(0)
	greedyStores := make(map[domain.StoreID]int64)

	for _, item := range input.Items {
		storePrices, ok := input.Prices[item.ProductID]
		if !ok || len(storePrices) == 0 {
			continue
		}

		bestPrice := int64(math.MaxInt64)
		bestStore := domain.StoreID{}
		found := false
		for storeID, price := range storePrices {
			if price < bestPrice {
				bestPrice = price
				bestStore = storeID
				found = true
			}
		}
		if !found {
			continue
		}

		lineTotal := bestPrice * int64(item.Quantity)
		greedyTotal += lineTotal
		greedyStores[bestStore] += lineTotal
	}

	greedyDelivery := calculateDelivery(greedyStores, input.Delivery)
	result.SavingsKopecks = (greedyTotal + greedyDelivery) - result.TotalKopecks
}
