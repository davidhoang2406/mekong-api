package model

type PriceSnapshot struct {
	Symbol     string  `json:"symbol"`
	Exchange   string  `json:"exchange"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	PctChange  float64 `json:"pct_change"`
	Volume     int64   `json:"volume"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Timestamp  string  `json:"timestamp"`
}
