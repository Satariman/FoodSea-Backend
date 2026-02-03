package domain

// RecognizedProduct представляет распознанный товар из голосового запроса
type RecognizedProduct struct {
	ProductID   int64
	ProductName string
	Quantity    int
	Confidence  float64
}

// VoiceRequest представляет запрос на обработку голосового ввода
type VoiceRequest struct {
	Text string
}

// VoiceResponse представляет результат обработки голосового запроса
type VoiceResponse struct {
	Products []*RecognizedProduct
}

