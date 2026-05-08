package domain

type VoiceItem struct {
	ProductID   string
	ProductName string
	Quantity    int32
	Unit        string
	Confidence  float64
	RawQuery    string
}
