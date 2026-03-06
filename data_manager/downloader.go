package data_manager

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"unified_quant_system/tdx_integration"

	"github.com/injoyai/tdx/protocol"
)

type Downloader struct {
	tdxClient *tdx_integration.TDXClient
	dataDir   string
}

type Record struct {
	Date      string // Format: YYYY-MM-DD
	Open      float64
	Close     float64
	High      float64
	Low       float64
	Volume    int64
	Amount    float64
	PctChange float64
}

func NewDownloader(tdxClient *tdx_integration.TDXClient, dataDir string) *Downloader {
	return &Downloader{
		tdxClient: tdxClient,
		dataDir:   dataDir,
	}
}

func (d *Downloader) UpdateAllStocks() error {
	codes, err := d.tdxClient.GetStockCodes()
	if err != nil {
		return err
	}

	dailyDataDir := filepath.Join(d.dataDir, "daily_data")
	if err := os.MkdirAll(dailyDataDir, 0755); err != nil {
		return err
	}

	fmt.Printf("Starting full K-line update for %d stocks...\n", len(codes))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)
	total := len(codes)
	processed := 0
	var mu sync.Mutex

	for _, code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			cleanCode := c
			if len(c) > 6 {
				cleanCode = c[len(c)-6:]
			}
			fileName := filepath.Join(dailyDataDir, cleanCode+".csv")
			d.updateSingleStockFull(c, fileName)

			mu.Lock()
			processed++
			if processed%100 == 0 || processed == total {
				fmt.Printf("Full Progress: %d/%d (%.2f%%)\n", processed, total, float64(processed)/float64(total)*100)
			}
			mu.Unlock()
		}(code)
	}

	wg.Wait()
	fmt.Println("Full K-line update completed.")
	return nil
}

func (d *Downloader) UpdateStocksIncremental() error {
	codes, err := d.tdxClient.GetStockCodes()
	if err != nil {
		return err
	}

	dailyDataDir := filepath.Join(d.dataDir, "daily_data")
	fmt.Printf("Starting incremental K-line update for %d stocks (Smart Merge)...\n", len(codes))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)
	total := len(codes)
	processed := 0
	var mu sync.Mutex

	for _, code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			cleanCode := c
			if len(c) > 6 {
				cleanCode = c[len(c)-6:]
			}
			fileName := filepath.Join(dailyDataDir, cleanCode+".csv")

			d.updateSingleStockSmart(c, fileName)

			mu.Lock()
			processed++
			if processed%100 == 0 || processed == total {
				fmt.Printf("Incremental Progress: %d/%d (%.2f%%)\n", processed, total, float64(processed)/float64(total)*100)
			}
			mu.Unlock()
		}(code)
	}

	wg.Wait()
	fmt.Println("Incremental K-line update completed.")
	return nil
}

func (d *Downloader) updateSingleStockFull(code, fileName string) {
	resp, err := d.tdxClient.GetDailyKLines(code)
	if err != nil || len(resp.List) == 0 {
		return
	}
	// Convert to Records
	var records []Record
	for _, k := range resp.List {
		records = append(records, d.klineToRecord(k))
	}
	// Recalculate PctChange
	d.recalculatePctChange(records)
	d.saveRecords(fileName, records)
}

func (d *Downloader) updateSingleStockSmart(code, fileName string) {
	// 1. Read existing records
	existingRecords, err := d.readRecords(fileName)
	if err != nil {
		// If file doesn't exist or error, fallback to full update
		d.updateSingleStockFull(code, fileName)
		return
	}

	// 2. Fetch recent KLines (e.g. last 80 days to cover gaps)
	resp, err := d.tdxClient.GetRecentKLines(code, 80)
	if err != nil || len(resp.List) == 0 {
		return
	}

	// 3. Merge: Map date -> Record
	recordMap := make(map[string]Record)
	for _, r := range existingRecords {
		recordMap[r.Date] = r
	}

	// Upsert new records
	for _, k := range resp.List {
		newRec := d.klineToRecord(k)
		recordMap[newRec.Date] = newRec // Overwrite existing date
	}

	// 4. Convert back to slice and Sort
	var failures int
	var merged []Record
	for _, r := range recordMap {
		merged = append(merged, r)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Date < merged[j].Date
	})

	// 5. Recalculate PctChange
	d.recalculatePctChange(merged)

	// 6. Write back
	d.saveRecords(fileName, merged)
	_ = failures // unused
}

func (d *Downloader) klineToRecord(k *protocol.Kline) Record {
	return Record{
		Date:   k.Time.Format("2006-01-02"),
		Open:   k.Open.Float64(),
		Close:  k.Close.Float64(),
		High:   k.High.Float64(),
		Low:    k.Low.Float64(),
		Volume: k.Volume,
		Amount: k.Amount.Float64(),
	}
}

func (d *Downloader) recalculatePctChange(records []Record) {
	for i := 0; i < len(records); i++ {
		if i == 0 {
			records[i].PctChange = 0
			continue
		}
		prevClose := records[i-1].Close
		if prevClose > 0 {
			records[i].PctChange = (records[i].Close - prevClose) / prevClose
		} else {
			records[i].PctChange = 0
		}
	}
}

func (d *Downloader) readRecords(fileName string) ([]Record, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("empty csv")
	}

	// Map headers
	headers := rows[0]
	hMap := make(map[string]int)
	for i, h := range headers {
		hMap[h] = i
	}

	var records []Record
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rec := Record{}
		// Helper to safely parse
		getFloat := func(col string) float64 {
			if idx, ok := hMap[col]; ok && idx < len(row) {
				f, _ := strconv.ParseFloat(row[idx], 64)
				return f
			}
			return 0
		}
		getInt := func(col string) int64 {
			if idx, ok := hMap[col]; ok && idx < len(row) {
				i, _ := strconv.ParseInt(row[idx], 10, 64)
				return i
			}
			return 0
		}
		getStr := func(col string) string {
			if idx, ok := hMap[col]; ok && idx < len(row) {
				return row[idx]
			}
			return ""
		}

		rec.Date = getStr("date")
		if rec.Date == "" {
			continue
		}
		rec.Open = getFloat("open")
		rec.Close = getFloat("close")
		rec.High = getFloat("high")
		rec.Low = getFloat("low")
		rec.Volume = getInt("volume")
		rec.Amount = getFloat("amount")
		rec.PctChange = getFloat("pctChange")
		records = append(records, rec)
	}
	return records, nil
}

func (d *Downloader) saveRecords(fileName string, records []Record) {
	file, err := os.Create(fileName) // Truncate and rewrite
	if err != nil {
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	// Write Header
	writer.Write([]string{"date", "open", "close", "high", "low", "volume", "amount", "pctChange"})

	for _, r := range records {
		writer.Write([]string{
			r.Date,
			strconv.FormatFloat(r.Open, 'f', 2, 64),
			strconv.FormatFloat(r.Close, 'f', 2, 64),
			strconv.FormatFloat(r.High, 'f', 2, 64),
			strconv.FormatFloat(r.Low, 'f', 2, 64),
			strconv.FormatInt(r.Volume, 10),
			strconv.FormatFloat(r.Amount, 'f', 2, 64),
			strconv.FormatFloat(r.PctChange, 'f', 4, 64),
		})
	}
	writer.Flush()
}

// ReadStockCSV 读取股票CSV数据
func (d *Downloader) ReadStockCSV(code string) ([]Record, error) {
	fileName := filepath.Join(d.dataDir, "daily_data", code+".csv")
	return d.readRecords(fileName)
}
