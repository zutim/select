package data_manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sync"
	"unified_quant_system/tdx_integration"
)

type StockPoolManager struct {
	tdxClient *tdx_integration.TDXClient
	dataDir   string
}

func NewStockPoolManager(tdxClient *tdx_integration.TDXClient, dataDir string) *StockPoolManager {
	return &StockPoolManager{
		tdxClient: tdxClient,
		dataDir:   dataDir,
	}
}

// UpdateCSI300Stocks 获取沪深300成分股，过滤ST和停牌股，并保存为JSON
func (spm *StockPoolManager) UpdateCSI300Stocks() error {
	log.Println("Updating CSI300 stock list...")

	// Get CSI300 stock codes (placeholder - needs actual TDX integration for CSI300 list)
	// For now, let's assume GetStockCodes returns all A-shares and we will filter later if needed.
	// A more precise implementation would involve a TDX function to get index components.
	allStockCodes, err := spm.tdxClient.GetStockCodes() // This might return all A-shares
	if err != nil {
		return fmt.Errorf("failed to get all stock codes from TDX: %w", err)
	}

	var csi300Stocks []string
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // Concurrency limit
	mu := &sync.Mutex{}

	log.Printf("Checking %d stocks for ST and paused status...", len(allStockCodes))

	for _, code := range allStockCodes {
		wg.Add(1)
		go func(stockCode string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			isST, err := spm.tdxClient.IsST(stockCode)
			if err != nil {
				// Log error but don't fail entire process
				log.Printf("Failed to check ST status for %s: %v", stockCode, err)
				return
			}
			if isST {
				// log.Printf("%s is ST stock, skipping.", stockCode)
				return
			}

			isPaused, err := spm.tdxClient.IsPaused(stockCode)
			if err != nil {
				log.Printf("Failed to check paused status for %s: %v", stockCode, err)
				return
			}
			if isPaused {
				// log.Printf("%s is paused stock, skipping.", stockCode)
				return
			}

			// Placeholder for checking if it's actually a CSI300 component
			// In a real scenario, tdx_integration would provide a function like GetCSI300Components()
			// For now, assume all non-ST, non-paused A-shares are candidates.
			mu.Lock()
			csi300Stocks = append(csi300Stocks, stockCode)
			mu.Unlock()

		}(code)
	}
	wg.Wait()

	log.Printf("Finished checking stocks. Found %d eligible stocks.", len(csi300Stocks))

	// Save to JSON
	jsonFilePath := filepath.Join(spm.dataDir, "csi300_stocks.json")
	data, err := json.MarshalIndent(csi300Stocks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CSI300 stocks to JSON: %w", err)
	}

	err = ioutil.WriteFile(jsonFilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write CSI300 stocks JSON file: %w", err)
	}

	log.Printf("CSI300 stock list updated and saved to %s", jsonFilePath)
	return nil
}

// ReadCSI300Stocks 从JSON文件读取沪深300成分股列表
func (spm *StockPoolManager) ReadCSI300Stocks() ([]string, error) {
	jsonFilePath := filepath.Join(spm.dataDir, "csi300_stocks.json")
	data, err := ioutil.ReadFile(jsonFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSI300 stocks JSON file: %w", err)
	}

	var stocks []string
	err = json.Unmarshal(data, &stocks)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CSI300 stocks JSON: %w", err)
	}

	return stocks, nil
}
