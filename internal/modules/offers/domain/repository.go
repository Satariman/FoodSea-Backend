package domain

import "context"

// OfferRepository определяет интерфейс для работы с предложениями
type OfferRepository interface {
	GetByProductID(ctx context.Context, productID int64) ([]*Offer, error)
	GetByPartnerID(ctx context.Context, partnerID int64) ([]*Offer, error)
}

// PartnerRepository определяет интерфейс для работы с партнерами
type PartnerRepository interface {
	GetAll(ctx context.Context) ([]*Partner, error)
	GetByID(ctx context.Context, id int64) (*Partner, error)
}

