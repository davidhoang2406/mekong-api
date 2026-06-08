package model

type ScreenerResult struct {
	Symbol       string   `json:"symbol"`
	PERatio      *float64 `json:"pe_ratio"`
	PBRatio      *float64 `json:"pb_ratio"`
	ROE          *float64 `json:"roe"`
	DERatio      *float64 `json:"de_ratio"`
	CurrentRatio *float64 `json:"current_ratio"`
}

type ScreenerResponse struct {
	Year    string           `json:"year"`
	Week    string           `json:"week"`
	Results []ScreenerResult `json:"results"`
}
