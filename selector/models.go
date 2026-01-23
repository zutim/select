package selector

import "time"

type PoolData struct {
	TargetDate             string   `json:"target_date"`
	PrevTradingDate        string   `json:"prev_trading_date"`
	Prev2TradingDate       string   `json:"prev_2_trading_date"`
	LimitUpStocks          []string `json:"limit_up_stocks"`
	LimitUp2DaysAgo        []string `json:"limit_up_2_days_ago"`
	FirstBoardStocks       []string `json:"first_board_stocks"`
	LimitUpNotClosedStocks []string `json:"limit_up_not_closed_stocks"`
	GeneratedAt            string   `json:"generated_at"`
}

type MarketCapData struct {
	LastUpdated string                    `json:"last_updated"`
	MarketCaps  map[string]StockMarketCap `json:"market_caps"`
}

type StockMarketCap struct {
	Name                 string  `json:"name"`
	CurrentPrice         float64 `json:"current_price"`
	TotalMarketCap       float64 `json:"total_market_cap"`
	CirculatingMarketCap float64 `json:"circulating_market_cap"`
	LastUpdated          string  `json:"last_updated"`
}

type AuctionData struct {
	Time      string  `json:"time"`
	Price     float64 `json:"price"`
	Volume    float64 `json:"volume"`
	Direction string  `json:"direction"`
	Order     int     `json:"order"`
}

type StockRecord struct {
	Date      time.Time `json:"date"`
	Open      float64   `json:"open"`
	Close     float64   `json:"close"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
	PctChange float64   `json:"pctChange"`
}

type SelectedStock struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Date     string `json:"date"`
	Strategy string `json:"strategy"`
}
