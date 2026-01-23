package tdx_integration

import (
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type TDXClient struct {
	client *tdx.Client
}

func NewTDXClient() (*TDXClient, error) {
	// DialDefault connects to a fast server automatically
	cli, err := tdx.DialDefault()
	if err != nil {
		return nil, err
	}
	return &TDXClient{client: cli}, nil
}

func (c *TDXClient) GetStockCodes() ([]string, error) {
	var ls []string
	for _, ex := range []protocol.Exchange{protocol.ExchangeSH, protocol.ExchangeSZ, protocol.ExchangeBJ} {
		resp, err := c.client.GetCodeAll(ex)
		if err != nil {
			return nil, err
		}
		for _, v := range resp.List {
			// library's IsStock expects 8-digit codes (prefix + 6 digits)
			// protocol.AddPrefix handles this correctly based on the code's first digit
			codeWithPrefix := protocol.AddPrefix(v.Code)
			if protocol.IsStock(codeWithPrefix) {
				ls = append(ls, codeWithPrefix)
			}
		}
	}
	return ls, nil
}

func (c *TDXClient) GetDailyKLines(code string) (*protocol.KlineResp, error) {
	// GetKlineDayAll is likely what we want for Daily K-lines
	return c.client.GetKlineDayAll(code)
}

func (c *TDXClient) GetRecentKLines(code string, count uint16) (*protocol.KlineResp, error) {
	return c.client.GetKlineDay(code, 0, count)
}

func (c *TDXClient) GetQuotes(codes []string) (protocol.QuotesResp, error) {
	var allQuotes protocol.QuotesResp
	batchSize := 80
	for i := 0; i < len(codes); i += batchSize {
		end := i + batchSize
		if end > len(codes) {
			end = len(codes)
		}
		quotes, err := c.client.GetQuote(codes[i:end]...)
		if err != nil {
			return nil, err
		}
		allQuotes = append(allQuotes, quotes...)
	}
	return allQuotes, nil
}

func (c *TDXClient) GetMinuteTradeAll(code string) (*protocol.TradeResp, error) {
	return c.client.GetMinuteTradeAll(code)
}

func (c *TDXClient) GetHistoryTradeDay(date, code string) (*protocol.TradeResp, error) {
	// date format: YYYYMMDD
	return c.client.GetHistoryTradeDay(date, code)
}

func (c *TDXClient) Close() {
	if c.client != nil {
		c.client.Close()
	}
}
