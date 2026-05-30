package model

type SymbolInfo struct {
	Symbol     string `json:"symbol"`
	AssetClass string `json:"asset_class"`
	Exchange   string `json:"exchange"`
	FirstDate  string `json:"first_date"`
	LastDate   string `json:"last_date"`
}

type SymbolsResponse struct {
	Symbols []SymbolInfo `json:"symbols"`
}
