package web_ui

//
//import (
//	"net/http"
//	"time"
//	"unified_quant_system/selector"
//	"unified_quant_system/data_manager"
//	//"unified_quant_system/backtest" // 新增回测模块
//
//	"github.com/gin-gonic/gin"
//)
//
//type WebUIHandlers struct {
//	stockSelector *selector.StockSelector
//	downloader    *data_manager.Downloader
//	//backtestEngine *backtest.BacktestEngine
//}
//
//func NewWebUIHandlers(stockSelector *selector.StockSelector, downloader *data_manager.Downloader, backtestEngine *backtest.BacktestEngine) *WebUIHandlers {
//	return &WebUIHandlers{
//		stockSelector: stockSelector,
//		downloader:    downloader,
//		backtestEngine: backtestEngine,
//	}
//}
//
//// GetAvailableDates returns available trading dates
//func (h *WebUIHandlers) GetAvailableDates(c *gin.Context) {
//	// Get trading dates from selector
//	tradingDates, err := h.stockSelector.GetTradingDates()
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//		return
//	}
//
//	c.JSON(http.StatusOK, gin.H{
//		"dates": tradingDates,
//	})
//}
//
//// RunStrategy runs a selected strategy
//func (h *WebUIHandlers) RunStrategy(c *gin.Context) {
//	var req struct {
//		Date     string `json:"date" binding:"required"`
//		Strategy string `json:"strategy" binding:"required"`
//	}
//
//	if err := c.ShouldBindJSON(&req); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//		return
//	}
//
//	// Reload market caps before selection
//	if err := h.stockSelector.LoadMarketCaps(); err != nil {
//		// Don't fail if market caps file doesn't exist yet, but log it
//		// We'll continue execution anyway
//	}
//
//	results, err := h.stockSelector.Run(req.Date)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "selection failed", "details": err.Error()})
//		return
//	}
//
//	c.JSON(http.StatusOK, gin.H{
//		"date":     req.Date,
//		"strategy": req.Strategy,
//		"count":    len(results),
//		"stocks":   results,
//	})
//}
//
//// GetSystemStatus returns current system status
//func (h *WebUIHandlers) GetSystemStatus(c *gin.Context) {
//	c.JSON(http.StatusOK, gin.H{
//		"status":      "running",
//		"timestamp":   time.Now().Unix(),
//		"version":     "1.0.0",
//		"last_update": time.Now().Format("2006-01-02 15:04:05"),
//	})
//}
//
//// RunBacktest 运行回测
//func (h *WebUIHandlers) RunBacktest(c *gin.Context) {
//	var req struct {
//		StartDate      string  `json:"start_date" binding:"required"`
//		EndDate        string  `json:"end_date" binding:"required"`
//		InitialCapital float64 `json:"initial_capital" binding:"required"`
//		Strategy       string  `json:"strategy" binding:"required"`
//	}
//
//	if err := c.ShouldBindJSON(&req); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//		return
//	}
//
//	// 定义策略函数
//	strategyFunc := func(date string) ([]selector.SelectedStock, error) {
//		// 重新加载市值数据
//		if err := h.stockSelector.LoadMarketCaps(); err != nil {
//			// 如果加载失败，继续执行
//		}
//
//		// 根据指定策略运行选股
//		results, err := h.stockSelector.Run(date)
//		if err != nil {
//			return nil, err
//		}
//
//		// 如果指定了特定策略，过滤结果
//		if req.Strategy != "all" {
//			filteredResults := make([]selector.SelectedStock, 0)
//			for _, stock := range results {
//				if stock.Strategy == req.Strategy {
//					filteredResults = append(filteredResults, stock)
//				}
//			}
//			results = filteredResults
//		}
//
//		return results, nil
//	}
//
//	// 运行回测
//	result, err := h.backtestEngine.RunBacktest(
//		req.StartDate,
//		req.EndDate,
//		req.InitialCapital,
//		strategyFunc,
//	)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "回测执行失败", "details": err.Error()})
//		return
//	}
//
//	c.JSON(http.StatusOK, result)
//}
//
//// GetBacktestMetrics 获取回测指标
//func (h *WebUIHandlers) GetBacktestMetrics(c *gin.Context) {
//	// 这里可以返回一些默认的指标描述
//	metrics := map[string]string{
//		"total_return": "总收益率",
//		"annual_return": "年化收益率",
//		"volatility": "波动率",
//		"max_drawdown": "最大回撤",
//		"sharpe_ratio": "夏普比率",
//		"win_rate": "胜率",
//	}
//
//	c.JSON(http.StatusOK, gin.H{"metrics": metrics})
//}
