package model

type IndicatorRow struct {
	Time       string   `json:"time"`
	Close      float64  `json:"close"`
	SMA20      *float64 `json:"sma20"`
	SMA50      *float64 `json:"sma50"`
	SMA200     *float64 `json:"sma200"`
	RSI14      *float64 `json:"rsi14"`
	MACD       *float64 `json:"macd"`
	MACDSignal *float64 `json:"macd_signal"`
	MACDHist   *float64 `json:"macd_hist"`
	BBUpper    *float64 `json:"bb_upper"`
	BBMid      *float64 `json:"bb_mid"`
	BBLower    *float64 `json:"bb_lower"`
}

type IndicatorsResponse struct {
	Symbol     string         `json:"symbol"`
	Indicators []IndicatorRow `json:"indicators"`
}
