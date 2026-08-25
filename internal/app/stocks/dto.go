package stocks

type StockListItemResponse struct {
	SKU             string  `json:"sku"`
	Name            string  `json:"name"`
	Quantity        int     `json:"quantity"`
	ValueLocked     float64 `json:"value_locked"`
	DeadstockStatus string  `json:"deadstock_status"`
	ItemsID         string  `json:"items_id"`
}

type ItemDetailResponse struct {
	SKU          string  `json:"sku"`
	Name         string  `json:"name"`
	Quantity     int     `json:"quantity"`
	ValueLocked  float64 `json:"value_locked"`
	CurrentPrice float64 `json:"current_price"`
	StocksID     string  `json:"stocks_id"`
}

type ItemDiagnoseResponse struct {
	DeadstockStatus string           `json:"deadstock_status"`
	OpportunityCost float64          `json:"opportunity_cost"`
	MarketAverage   float64          `json:"market_average"`
	Reasons         []DiagnoseReason `json:"reasons"`
}

type DiagnoseReason struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
}

type StockProjectionItemResponse struct {
	SKU                  string      `json:"sku"`
	Name                 string      `json:"name"`
	ProjectionPercentage float64     `json:"projection_percentage"`
	ProjectionPoints     [][]float64 `json:"projection_points"`
	Decision             string      `json:"decision"`
}


type ImportResponse struct {
	StockID       string `json:"stock_id"`
	ItemCount     int    `json:"item_count"`
	ForecastMonth string `json:"forecast_month"`
}
