package web_ui

import (
	"net/http"
	"path/filepath"
	"unified_quant_system/data_manager"
	"unified_quant_system/selector"
	//"unified_quant_system/backtest" // 新增回测模块

	"github.com/gin-gonic/gin"
)

// StartWebServer 启动Web服务器
func StartWebServer(r *gin.Engine, stockSelector *selector.StockSelector, downloader *data_manager.Downloader) {
	// 创建回测引擎
	//backtestEngine := backtest.NewBacktestEngine(stockSelector, downloader, "./data")
	//
	//handlers := NewWebUIHandlers(stockSelector, downloader, backtestEngine)

	// 设置静态文件服务
	staticDir := "./web_ui/static"
	r.Static("/static", staticDir)

	// 设置模板目录
	templateDir := "./web_ui/templates"
	r.LoadHTMLGlob(filepath.Join(templateDir, "*.html"))

	// 主页路由
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	// API 路由组
	//api := r.Group("/api")
	{
		//api.GET("/dates", handlers.GetAvailableDates)
		//api.POST("/run_strategy", handlers.RunStrategy)
		//api.GET("/status", handlers.GetSystemStatus)
		//api.POST("/backtest", handlers.RunBacktest)         // 新增回测API
		//api.GET("/backtest/metrics", handlers.GetBacktestMetrics) // 新增指标API
	}
}
