package model

type OHLCVBar struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

type OHLCVResponse struct {
	Symbol     string     `json:"symbol"`
	AssetClass string     `json:"asset_class"`
	Exchange   string     `json:"exchange"`
	Bars       []OHLCVBar `json:"bars"`
}
