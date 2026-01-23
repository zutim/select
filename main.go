package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"unified_quant_system/data_manager"
	"unified_quant_system/selector"
	"unified_quant_system/tdx_integration"

	"github.com/gin-gonic/gin"
)

func main() {
	// Configuration
	dataDir := "./data"
	pythonPath := "python3" // or path to venv python
	marketCapScript := "./scripts/get_market_caps.py"

	// Ensure directories exist
	os.MkdirAll(dataDir, 0755)

	// Initialize TDX Client
	tdxClient, err := tdx_integration.NewTDXClient()
	if err != nil {
		log.Fatalf("Failed to initialize TDX client: %v", err)
	}
	defer tdxClient.Close()

	// Initialize Components
	downloader := data_manager.NewDownloader(tdxClient, dataDir)
	pythonBridge := data_manager.NewPythonBridge(pythonPath, marketCapScript)
	stockSelector := selector.NewStockSelector(dataDir, tdxClient)

	// Set up Gin
	r := gin.Default()

	// Endpoints
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Specialized Endpoints
	r.POST("/api/v1/data/update/market-caps", func(c *gin.Context) {
		fmt.Println("Starting market cap update in background...")
		go func() {
			if err := pythonBridge.UpdateMarketCaps(); err != nil {
				fmt.Printf("Error: market cap update failed: %v\n", err)
				return
			}
			fmt.Println("Market cap update finished successfully.")
		}()
		c.JSON(http.StatusOK, gin.H{"message": "Market cap update started in background."})
	})

	r.POST("/api/v1/data/update/klines", func(c *gin.Context) {
		fmt.Println("Starting K-line full update in background...")
		go func() {
			if err := downloader.UpdateAllStocks(); err != nil {
				fmt.Printf("Error: k-line full update failed: %v\n", err)
				return
			}
			fmt.Println("K-line full update finished successfully.")
		}()
		c.JSON(http.StatusOK, gin.H{"message": "K-line full update started in background."})
	})

	r.POST("/api/v1/data/update/klines/incremental", func(c *gin.Context) {
		fmt.Println("Starting K-line incremental update in background...")
		go func() {
			if err := downloader.UpdateStocksIncremental(); err != nil {
				fmt.Printf("Error: k-line incremental update failed: %v\n", err)
				return
			}
			fmt.Println("K-line incremental update finished successfully.")
		}()
		c.JSON(http.StatusOK, gin.H{"message": "K-line incremental update started in background."})
	})

	// Legacy/Convenience Merged Endpoint
	r.POST("/api/v1/data/update", func(c *gin.Context) {
		fmt.Println("Starting full data update in background...")
		go func() {
			fmt.Println("Step 1: Updating Market Caps...")
			pythonBridge.UpdateMarketCaps()
			fmt.Println("Step 2: Updating K-lines...")
			downloader.UpdateAllStocks()
			fmt.Println("Full data update finished.")
		}()
		c.JSON(http.StatusOK, gin.H{"message": "Full data update started in background."})
	})

	r.GET("/api/v1/selection/run", func(c *gin.Context) {
		date := c.Query("date")
		if date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date parameter is required (YYYY-MM-DD)"})
			return
		}

		// Reload market caps before selection
		if err := stockSelector.LoadMarketCaps(); err != nil {
			// Don't fail if market caps file doesn't exist yet, but log it
			fmt.Printf("Warning: could not load market caps: %v\n", err)
		}

		results, err := stockSelector.Run(date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "selection failed", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"date":   date,
			"count":  len(results),
			"stocks": results,
		})
	})

	fmt.Println("Unified Quant System starting on :8081")
	r.Run(":8081")
}
