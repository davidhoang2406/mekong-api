package model

type DigestEntry struct {
	Category   string  `json:"category"`
	Rank       int     `json:"rank"`
	Symbol     string  `json:"symbol"`
	Exchange   string  `json:"exchange"`
	AssetClass string  `json:"asset_class"`
	Open       float64 `json:"open"`
	Close      float64 `json:"close"`
	Volume     int64   `json:"volume"`
	PctChange  float64 `json:"pct_change"`
}

type DigestResponse struct {
	Date   string        `json:"date"`
	Digest []DigestEntry `json:"digest"`
}
