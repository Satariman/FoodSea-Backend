package domain

import "context"

// VoiceProcessor определяет интерфейс для обработки голосовых запросов
type VoiceProcessor interface {
	ProcessText(ctx context.Context, text string) (*VoiceResponse, error)
}

// MLGateway определяет интерфейс для взаимодействия с ML-модулем
type MLGateway interface {
	ExtractProducts(ctx context.Context, text string) (*VoiceResponse, error)
}

