package repository

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/foodsea/optimization/ent"
	entoptitem "github.com/foodsea/optimization/ent/optimizationitem"
	entoptres "github.com/foodsea/optimization/ent/optimizationresult"
	entsub "github.com/foodsea/optimization/ent/substitution"
	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

// ResultRepo is an Ent-backed repository for optimization results.
type ResultRepo struct {
	client *ent.Client
	log    *slog.Logger
}

func NewResultRepo(client *ent.Client, log *slog.Logger) *ResultRepo {
	return &ResultRepo{client: client, log: log}
}

func (r *ResultRepo) Save(ctx context.Context, result *domain.OptimizationResult) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if result.ID == uuid.Nil {
		result.ID = uuid.New()
	}

	created, err := tx.OptimizationResult.Create().
		SetID(result.ID).
		SetUserID(result.UserID).
		SetCartHash(result.CartHash).
		SetTotalKopecks(result.TotalKopecks).
		SetDeliveryKopecks(result.DeliveryKopecks).
		SetSavingsKopecks(result.SavingsKopecks).
		SetStatus(result.Status).
		SetIsApproximate(result.IsApproximate).
		Save(ctx)
	if err != nil {
		return err
	}
	result.CreatedAt = created.CreatedAt
	result.UpdatedAt = created.UpdatedAt

	if len(result.Items) > 0 {
		bulkItems := make([]*ent.OptimizationItemCreate, 0, len(result.Items))
		for _, item := range result.Items {
			if item.Quantity > math.MaxInt16 {
				return fmt.Errorf("optimization item quantity %d exceeds int16", item.Quantity)
			}
			bulkItems = append(bulkItems, tx.OptimizationItem.Create().
				SetResultID(result.ID).
				SetProductID(item.ProductID).
				SetProductName(item.ProductName).
				SetStoreID(item.StoreID).
				SetStoreName(item.StoreName).
				SetQuantity(int16(item.Quantity)).
				SetPriceKopecks(item.Price))
		}
		if _, err = tx.OptimizationItem.CreateBulk(bulkItems...).Save(ctx); err != nil {
			return err
		}
	}

	if len(result.Substitutions) > 0 {
		bulkSubs := make([]*ent.SubstitutionCreate, 0, len(result.Substitutions))
		for i := range result.Substitutions {
			sub := &result.Substitutions[i]
			bulkSubs = append(bulkSubs, tx.Substitution.Create().
				SetResultID(result.ID).
				SetOriginalProductID(sub.OriginalID).
				SetOriginalProductName(sub.OriginalProductName).
				SetAnalogProductID(sub.AnalogID).
				SetAnalogProductName(sub.AnalogProductName).
				SetOriginalStoreID(sub.OriginalStoreID).
				SetNewStoreID(sub.NewStoreID).
				SetNewStoreName(sub.NewStoreName).
				SetOldPriceKopecks(sub.OldPriceKopecks).
				SetNewPriceKopecks(sub.NewPriceKopecks).
				SetPriceDeltaKopecks(sub.PriceDeltaKopecks).
				SetDeliveryDeltaKopecks(sub.DeliveryDeltaKopecks).
				SetTotalSavingKopecks(sub.TotalSavingKopecks).
				SetScore(sub.Score).
				SetIsCrossStore(sub.IsCrossStore))
		}
		if _, err = tx.Substitution.CreateBulk(bulkSubs...).Save(ctx); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (r *ResultRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.OptimizationResult, error) {
	row, err := r.client.OptimizationResult.Query().
		Where(entoptres.IDEQ(id)).
		WithItems().
		WithSubstitutions().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrResultNotFound
		}
		return nil, err
	}

	return entResultToDomain(row), nil
}

func (r *ResultRepo) FindByCartHash(ctx context.Context, cartHash string) (*domain.OptimizationResult, error) {
	row, err := r.client.OptimizationResult.Query().
		Where(entoptres.CartHash(cartHash), entoptres.StatusEQ("active")).
		Order(entoptres.ByCreatedAt(entsql.OrderDesc())).
		WithItems().
		WithSubstitutions().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrResultNotFound
		}
		return nil, err
	}
	return entResultToDomain(row), nil
}

func (r *ResultRepo) Lock(ctx context.Context, id uuid.UUID) error {
	affected, err := r.client.OptimizationResult.Update().
		Where(entoptres.IDEQ(id), entoptres.StatusEQ("active")).
		SetStatus("locked").
		Save(ctx)
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}

	row, getErr := r.client.OptimizationResult.Query().Where(entoptres.IDEQ(id)).Only(ctx)
	if getErr != nil {
		if ent.IsNotFound(getErr) {
			return domain.ErrResultNotFound
		}
		return getErr
	}
	if row.Status == "locked" {
		return domain.ErrResultLocked
	}
	return domain.ErrResultNotActive
}

func (r *ResultRepo) Unlock(ctx context.Context, id uuid.UUID) error {
	affected, err := r.client.OptimizationResult.Update().
		Where(entoptres.IDEQ(id), entoptres.StatusEQ("locked")).
		SetStatus("active").
		Save(ctx)
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}

	row, getErr := r.client.OptimizationResult.Query().Where(entoptres.IDEQ(id)).Only(ctx)
	if getErr != nil {
		if ent.IsNotFound(getErr) {
			return domain.ErrResultNotFound
		}
		return getErr
	}
	if row.Status == "active" {
		return domain.ErrResultNotLocked
	}
	return domain.ErrResultNotActive
}

func (r *ResultRepo) ExpireOld(ctx context.Context, olderThan time.Time) (int, error) {
	affected, err := r.client.OptimizationResult.Update().
		Where(entoptres.StatusEQ("active"), entoptres.CreatedAtLT(olderThan)).
		SetStatus("expired").
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *ResultRepo) DeleteByUserCartHash(ctx context.Context, userID uuid.UUID, cartHash string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	ids, err := tx.OptimizationResult.Query().
		Where(entoptres.UserIDEQ(userID), entoptres.CartHash(cartHash)).
		IDs(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	if _, err = tx.Substitution.Delete().Where(entsub.ResultIDIn(ids...)).Exec(ctx); err != nil {
		return err
	}
	if _, err = tx.OptimizationItem.Delete().Where(entoptitem.ResultIDIn(ids...)).Exec(ctx); err != nil {
		return err
	}
	if _, err = tx.OptimizationResult.Delete().Where(entoptres.IDIn(ids...)).Exec(ctx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true

	r.log.InfoContext(ctx, "optimization cache invalidated", "user_id", userID, "cart_hash", cartHash, "deleted", len(ids))
	return nil
}

func (r *ResultRepo) DeleteActiveByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	ids, err := tx.OptimizationResult.Query().
		Where(entoptres.UserIDEQ(userID), entoptres.StatusEQ("active")).
		IDs(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if _, err = tx.Substitution.Delete().Where(entsub.ResultIDIn(ids...)).Exec(ctx); err != nil {
		return 0, err
	}
	if _, err = tx.OptimizationItem.Delete().Where(entoptitem.ResultIDIn(ids...)).Exec(ctx); err != nil {
		return 0, err
	}
	if _, err = tx.OptimizationResult.Delete().Where(entoptres.IDIn(ids...)).Exec(ctx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return len(ids), nil
}

func entResultToDomain(row *ent.OptimizationResult) *domain.OptimizationResult {
	items := make([]domain.Assignment, len(row.Edges.Items))
	for i, item := range row.Edges.Items {
		items[i] = domain.Assignment{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			StoreID:     item.StoreID,
			StoreName:   item.StoreName,
			Price:       item.PriceKopecks,
			Quantity:    int(item.Quantity),
		}
	}

	subs := make([]domain.Substitution, len(row.Edges.Substitutions))
	for i, sub := range row.Edges.Substitutions {
		subs[i] = domain.Substitution{
			OriginalID:           sub.OriginalProductID,
			OriginalProductName:  sub.OriginalProductName,
			AnalogID:             sub.AnalogProductID,
			AnalogProductName:    sub.AnalogProductName,
			OriginalStoreID:      sub.OriginalStoreID,
			NewStoreID:           sub.NewStoreID,
			NewStoreName:         sub.NewStoreName,
			OldPriceKopecks:      sub.OldPriceKopecks,
			NewPriceKopecks:      sub.NewPriceKopecks,
			PriceDeltaKopecks:    sub.PriceDeltaKopecks,
			DeliveryDeltaKopecks: sub.DeliveryDeltaKopecks,
			TotalSavingKopecks:   sub.TotalSavingKopecks,
			Score:                sub.Score,
			IsCrossStore:         sub.IsCrossStore,
		}
	}

	return &domain.OptimizationResult{
		ID:              row.ID,
		UserID:          row.UserID,
		CartHash:        row.CartHash,
		TotalKopecks:    row.TotalKopecks,
		DeliveryKopecks: row.DeliveryKopecks,
		SavingsKopecks:  row.SavingsKopecks,
		Status:          row.Status,
		IsApproximate:   row.IsApproximate,
		Items:           items,
		Substitutions:   subs,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
