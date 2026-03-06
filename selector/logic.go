package selector

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unified_quant_system/tdx_integration"

	"github.com/injoyai/tdx/protocol"
	tp "github.com/injoyai/tdx/protocol"
)

type StockSelector struct {
	dataDir    string
	tdxClient  *tdx_integration.TDXClient
	marketCaps map[string]StockMarketCap
	mu         sync.RWMutex
}

func NewStockSelector(dataDir string, tdxClient *tdx_integration.TDXClient) *StockSelector {
	return &StockSelector{
		dataDir:   dataDir,
		tdxClient: tdxClient,
	}
}

func (s *StockSelector) LoadMarketCaps() error {
	filePath := filepath.Join(s.dataDir, "market_caps.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var mcData MarketCapData
	if err := json.Unmarshal(data, &mcData); err != nil {
		return err
	}
	s.mu.Lock()
	s.marketCaps = mcData.MarketCaps
	s.mu.Unlock()
	return nil
}

func (s *StockSelector) Run(targetDate string) ([]SelectedStock, error) {
	poolDir := filepath.Join(s.dataDir, "pool_data")
	poolFile := filepath.Join(poolDir, fmt.Sprintf("pool_%s.json", targetDate))

	var poolData PoolData
	if data, err := os.ReadFile(poolFile); err == nil {
		if err := json.Unmarshal(data, &poolData); err == nil {
			fmt.Printf("Using cached pool for %s\n", targetDate)
			goto RUN_STRATEGIES
		}
	}

	// 1. Get Trading Dates
	{
		tradingDates, err := s.GetTradingDates()
		if err != nil {
			return nil, err
		}

		fmt.Println("Trading Dates: %v", tradingDates)

		// 查找目标日期或最接近的交易日
		// tradingDates 是按日期降序排列的（最新的日期在前）[2026-01-25, 2026-01-23, ...]
		var targetDateIndex = -1
		targetDateParsed, _ := time.Parse("2006-01-02", targetDate)

		// 遍历交易日期，找到目标日期在列表中的位置或最接近的位置
		for i, d := range tradingDates {
			dParsed, _ := time.Parse("2006-01-02", d)
			if dParsed.Equal(targetDateParsed) {
				// 目标日期正好是交易日
				targetDateIndex = i
				break
			} else if dParsed.Before(targetDateParsed) {
				// 如果当前交易日早于目标日期，这可能就是要找的交易日
				// 但我们应该继续查找，直到遇到晚于目标日期的交易日，
				// 或者找到最接近的早于目标日期的交易日
				targetDateIndex = i
				break // 找到第一个早于目标日期的交易日，这就是最接近的
			}
			// 如果当前交易日晚于目标日期，继续寻找
		}

		fmt.Println("Target Date: %s", targetDateIndex)
		var prevDate, prev2Date string
		if targetDateIndex != -1 {
			// prevDate 是最接近目标日期的前一个交易日
			if targetDateIndex >= 0 {
				prevDate = tradingDates[targetDateIndex+1]
			}
			// prev2Date 是前一个交易日的下一个交易日（即更早的交易日）
			if targetDateIndex+1 < len(tradingDates) {
				prev2Date = tradingDates[targetDateIndex+2]
			}
		}

		if prevDate == "" || prev2Date == "" {
			return nil, fmt.Errorf("could not find previous trading dates for %s", targetDate)
		}

		// 2. Identify Pools in Single Pass
		poolData = s.generatePoolData(targetDate, prevDate, prev2Date)

		// Save pool data
		os.MkdirAll(poolDir, 0755)
		poolBytes, _ := json.MarshalIndent(poolData, "", "  ")
		os.WriteFile(poolFile, poolBytes, 0644)
	}

RUN_STRATEGIES:
	isToday := targetDate == time.Now().Format("2006-01-02")

	resultsChan := make(chan SelectedStock, 100)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	limitUpYesterdayMap := make(map[string]bool)
	for _, c := range poolData.LimitUpStocks {
		limitUpYesterdayMap[c] = true
	}
	limitUp2DaysAgoMap := make(map[string]bool)
	for _, c := range poolData.LimitUp2DaysAgo {
		limitUp2DaysAgoMap[c] = true
	}

	// First Board (SBGK, SBDK)
	for _, code := range poolData.FirstBoardStocks {
		if !s.isEligible(code) {
			continue
		}
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			s.processFirstBoard(c, targetDate, poolData, limitUpYesterdayMap, limitUp2DaysAgoMap, resultsChan, 0, 0, isToday)
		}(code)
	}

	// Weak to Strong (RZQ)
	for _, code := range poolData.LimitUpNotClosedStocks {
		if !s.isEligible(code) {
			continue
		}
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			s.processWeakToStrong(c, targetDate, resultsChan, 0, 0, isToday)
		}(code)
	}

	go func() { wg.Wait(); close(resultsChan) }()
	var results []SelectedStock
	for res := range resultsChan {
		results = append(results, res)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Code < results[j].Code })
	return results, nil
}

func (s *StockSelector) isEligible(code string) bool {
	// 仿照 stock_selector.go 只过滤科创板(68), 北交所(4, 8)
	if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") || strings.HasPrefix(code, "68") {
		return false
	}
	return true
}

func (s *StockSelector) GetTradingDates() ([]string, error) {
	dailyDir := filepath.Join(s.dataDir, "daily_data")
	files, _ := os.ReadDir(dailyDir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".csv") {
			records, _ := s.readStockCSV(strings.TrimSuffix(f.Name(), ".csv"))
			if len(records) > 0 {
				var dates []string
				for _, r := range records {
					dates = append(dates, r.Date.Format("2006-01-02"))
				}
				sort.Slice(dates, func(i, j int) bool { return dates[i] > dates[j] })
				// 过滤掉非交易日（周末）
				var tradingDates []string
				for _, dateStr := range dates {
					date, err := time.Parse("2006-01-02", dateStr)
					if err != nil {
						continue
					}
					// 跳过周六(6)和周日(0)
					if date.Weekday() != time.Saturday && date.Weekday() != time.Sunday {
						tradingDates = append(tradingDates, dateStr)
					}
				}
				return tradingDates, nil
			}
		}
	}
	return nil, fmt.Errorf("no data")
}

func (s *StockSelector) generatePoolData(target, prev, prev2 string) PoolData {
	fmt.Println("Generating pool data for %s...", target)
	fmt.Println("Previous trading date: %s", prev)
	fmt.Println("Previous 2 trading date: %s", prev2)
	dailyDir := filepath.Join(s.dataDir, "daily_data")
	files, _ := os.ReadDir(dailyDir)

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 50)

	var limitUpYesterday, limitUp2DaysAgo, zhaBanYesterday []string

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".csv") {
			continue
		}
		wg.Add(1)
		go func(fname string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			code := strings.TrimSuffix(fname, ".csv")
			records, _ := s.readStockCSV(code)
			if len(records) < 2 {
				return
			}

			var recPrev, recPrev2, recPrevPrev, recPrevPrev2 *StockRecord
			for i := range records {
				ds := records[i].Date.Format("2006-01-02")
				if ds == prev {
					recPrev = &records[i]
					if i > 0 {
						recPrevPrev = &records[i-1]
					}
				}
				if ds == prev2 {
					recPrev2 = &records[i]
					if i > 0 {
						recPrevPrev2 = &records[i-1]
					}
				}
			}

			mu.Lock()
			defer mu.Unlock()
			if recPrev != nil && recPrevPrev != nil {
				if s.isLimitUp(code, recPrev.Close, recPrevPrev.Close) {
					limitUpYesterday = append(limitUpYesterday, code)
				}
				limitPrice := s.getLimitPrice(code, recPrevPrev.Close)
				if recPrev.High >= limitPrice && recPrev.Close < limitPrice {
					zhaBanYesterday = append(zhaBanYesterday, code)
				}
			}
			if recPrev2 != nil && recPrevPrev2 != nil {
				if s.isLimitUp(code, recPrev2.Close, recPrevPrev2.Close) {
					limitUp2DaysAgo = append(limitUp2DaysAgo, code)
				}
			}
		}(f.Name())
	}
	wg.Wait()

	limit2Map := make(map[string]bool)
	for _, c := range limitUp2DaysAgo {
		limit2Map[c] = true
	}
	var firstBoard []string
	for _, c := range limitUpYesterday {
		if !limit2Map[c] {
			firstBoard = append(firstBoard, c)
		}
	}

	return PoolData{
		TargetDate: target, PrevTradingDate: prev, Prev2TradingDate: prev2,
		LimitUpStocks: limitUpYesterday, LimitUp2DaysAgo: limitUp2DaysAgo,
		FirstBoardStocks: firstBoard, LimitUpNotClosedStocks: zhaBanYesterday,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (s *StockSelector) isLimitUp(code string, close, prevClose float64) bool {
	limitPrice := s.getLimitPrice(code, prevClose)
	return math.Abs(close-limitPrice) <= 0.01
}

func (s *StockSelector) getLimitPrice(code string, prevClose float64) float64 {
	ratio := 0.1
	if strings.HasPrefix(code, "30") || strings.HasPrefix(code, "68") {
		ratio = 0.2
	}
	s.mu.RLock()
	mc, ok := s.marketCaps[code]
	s.mu.RUnlock()
	if ok && (strings.Contains(mc.Name, "ST") || strings.Contains(mc.Name, "st")) {
		ratio = 0.05
	}
	return math.Round(prevClose*(1+ratio)*100) / 100
}

func (s *StockSelector) readStockCSV(code string) ([]StockRecord, error) {
	filePath := filepath.Join(s.dataDir, "daily_data", code+".csv")
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	headers, _ := reader.Read()

	// 仿照 stock_selector.go 的列映射逻辑
	columnMappings := map[string]string{
		"date": "date", "pct_change": "pct_change", "pctChange": "pct_change",
		"open": "open", "close": "close", "high": "high", "low": "low",
		"volume": "volume", "amount": "amount",
		"日期": "date", "成交量": "volume", "成交额": "amount", "开盘": "open", "收盘": "close", "最高": "high", "最低": "low", "涨跌幅": "pct_change",
	}

	hMap := make(map[string]int)
	for i, h := range headers {
		h = strings.TrimSpace(h)
		// 移除各种空格（包括全角）进行匹配
		normH := strings.ReplaceAll(strings.ReplaceAll(h, " ", ""), "　", "")
		for key, target := range columnMappings {
			if strings.Contains(normH, key) || normH == key {
				hMap[target] = i
				break
			}
		}
	}

	var records []StockRecord
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}

		getF := func(key string) float64 {
			if idx, ok := hMap[key]; ok && idx < len(row) {
				f, _ := strconv.ParseFloat(strings.TrimSpace(row[idx]), 64)
				return f
			}
			return 0
		}

		dateStr := strings.TrimSpace(row[hMap["date"]])
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			d, _ = time.Parse("2006/01/02", dateStr)
		}
		if d.IsZero() {
			continue
		}
		r := StockRecord{
			Date:      d,
			Open:      getF("open"),
			Close:     getF("close"),
			High:      getF("high"),
			Low:       getF("low"),
			Amount:    getF("amount"),
			PctChange: getF("pct_change"),
		}
		r.Volume = int64(math.Round(getF("volume")))
		records = append(records, r)
	}
	return records, nil
}

func (s *StockSelector) adjustVolume(rawVolume int64, amount, close float64) float64 {
	v := float64(rawVolume)
	if v == 0 {
		return 1
	}
	// 仿照 selection/stock_selector.go 的逻辑：
	// 检查原始平均成交价。如果远高于收盘价，说明 Volume 单位是“手”，需要 * 100 变成“股”。
	rawAvgPrice := amount / v
	if rawAvgPrice > close*5 {
		return v * 100
	}
	return v
}

func (s *StockSelector) getAuctionData(code, date string, isToday bool) (float64, float64) {

	var resp *protocol.TradeResp
	var err error

	if isToday {
		for i := 0; i < 3; i++ {
			resp, err = s.tdxClient.GetMinuteTradeAll(code)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	} else {
		formattedDate := strings.ReplaceAll(date, "-", "")
		for i := 0; i < 3; i++ {
			resp, err = s.tdxClient.GetHistoryTradeDay(formattedDate, code)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	}

	if err != nil {
		fmt.Printf("API请求失败 (尝试3次)，股票 %s 在 %s (isToday:%t): %v\n", code, date, isToday, err)
		return 0, 0
	}

	if len(resp.List) == 0 {
		return 0, 0
	}

	// 仿照 stock_selector.go，寻找 09:25-09:29 之间最晚的一条记录
	var latestAuction *tp.Trade
	for _, tradeData := range resp.List {
		ts := tradeData.Time.String()
		// 获取 09:25:00 这种格式，或者更模糊的匹配
		if strings.Contains(ts, "09:25") || strings.Contains(ts, "09:26") ||
			strings.Contains(ts, "09:27") || strings.Contains(ts, "09:28") ||
			strings.Contains(ts, "09:29") {
			latestAuction = tradeData
		}
	}

	// 如果没找到竞价数据，退而求其次寻找 09:15 之后 09:30 之前的数据
	if latestAuction == nil {
		for _, tradeData := range resp.List {
			ts := tradeData.Time.String()
			if strings.Contains(ts, "09:1") || strings.Contains(ts, "09:2") {
				latestAuction = tradeData
			}
		}
	}

	if latestAuction == nil {
		// 最后的备份：取第一条（如果是当日分笔，第一条通常是开盘竞价）
		if isToday && len(resp.List) > 0 {
			latestAuction = resp.List[0]
		} else if len(resp.List) > 0 {
			latestAuction = resp.List[len(resp.List)-1]
		}
	}

	if latestAuction == nil {
		fmt.Printf("DEBUG: 无法从交易列表中识别竞价数据 %s (list len: %d)\n", code, len(resp.List))
		return 0, 0
	}

	// 单位换算：github.com/injoyai/tdx 库的 Float64() 已经处理了 0.001 的缩放
	price := latestAuction.Price.Float64()
	// 原始 Volume 通常是以“手”为单位，所以 * 100
	rawVol := float64(latestAuction.Volume)
	if rawVol == 0 {
		rawVol = latestAuction.Amount().Float64() / price / 100.0
	}
	convertedVolume := rawVol * 100

	fmt.Printf("成功获取竞价数据 %s: 价格=%.2f, 成交量=%.0f股 (时间:%s, isToday:%t)\n", code, price, convertedVolume, latestAuction.Time.String(), isToday)
	return price, convertedVolume
}

func (s *StockSelector) processFirstBoard(code, targetDate string, poolData PoolData, limitUpYesterdayMap, limitUp2DaysAgoMap map[string]bool, resultsChan chan<- SelectedStock, preAuctionPrice, preAuctionVolume float64, isToday bool) {
	records, err := s.readStockCSV(code)
	if err != nil {
		return
	}
	var todayRec, prevRec *StockRecord
	var prevIdx int
	targetTime, _ := time.Parse("2006-01-02", targetDate)

	for i := range records {
		if records[i].Date.Equal(targetTime) {
			todayRec = &records[i]
		}
		if records[i].Date.Before(targetTime) {
			prevRec = &records[i]
			prevIdx = i
		}
	}

	if prevRec == nil {
		return
	}

	s.mu.RLock()
	mc, ok := s.marketCaps[code]
	s.mu.RUnlock()
	if !ok {
		return
	}

	isLimitUpYesterday := limitUpYesterdayMap[code]
	wasLimitUp2DaysAgo := limitUp2DaysAgoMap[code]
	isFirstBoard := isLimitUpYesterday && !wasLimitUp2DaysAgo

	if isFirstBoard {
		var auctionPrice, auctionVolume float64
		if isToday && preAuctionPrice > 0 {
			auctionPrice, auctionVolume = preAuctionPrice, preAuctionVolume
		} else {
			auctionPrice, auctionVolume = s.getAuctionData(code, targetDate, isToday)
			if auctionPrice == 0 && todayRec != nil {
				auctionPrice = todayRec.Open
			}
		}

		if auctionPrice > 0 {
			prevClose := prevRec.Close
			prevVolume := s.adjustVolume(prevRec.Volume, prevRec.Amount, prevClose)

			volumeRatio := auctionVolume / prevVolume
			currentRatio := auctionPrice / prevClose

			avgPrice := prevRec.Amount / prevVolume
			avgPriceIncrease := (avgPrice / prevClose * 1.1) - 1

			// 市值计算 (仿照 stock_selector.go)
			latestPrice := prevClose
			if todayRec != nil && todayRec.Close > 0 {
				latestPrice = todayRec.Close
			} else if auctionPrice > 0 {
				latestPrice = auctionPrice
			}
			ratio := latestPrice / mc.CurrentPrice
			totalMC := mc.TotalMarketCap * ratio
			circMC := mc.CirculatingMarketCap * ratio

			leftPressure := s.checkLeftPressure(records, prevIdx, prevVolume)
			cond1_p1 := avgPriceIncrease >= 0.07
			cond1_p2 := prevRec.Amount >= 5.5e8 && prevRec.Amount <= 20e8
			cond2_p1 := volumeRatio >= 0.03
			cond2_p2 := (currentRatio > 1.0 && currentRatio < 1.06)
			condMC := totalMC >= 70 && circMC <= 520

			// 详细输出每个条件的检查结果 (同步 select_2026_01_12.py)
			fmt.Printf("股票 %s: 均价获利=%.3f(≥0.07?%t), 成交额=%.2f亿(in[5.5,20]?%t), 市值总=%.2f亿≥70?%t, 市值流=%.2f亿≤520?%t, 竞价量比=%.3f(≥0.03?%t), 开盘比=%.3f(1.0<%.3f<1.06?%t), 左压?%t\n",
				code, avgPriceIncrease, cond1_p1,
				prevRec.Amount/1e8, cond1_p2,
				totalMC, totalMC >= 70, circMC, circMC <= 520,
				volumeRatio, cond2_p1,
				currentRatio, currentRatio, cond2_p2,
				leftPressure)

			if cond1_p1 && cond1_p2 && condMC && cond2_p1 && cond2_p2 && leftPressure {
				resultsChan <- SelectedStock{Code: code, Name: mc.Name, Date: targetDate, Strategy: "First Board High Open"}
				fmt.Printf("HIT: 首板高开 %s 选中!\n", code)
			}
		}

		// 2. SBDK Strategy
		if todayRec != nil {
			prevClose := prevRec.Close
			openRatio := todayRec.Open / prevClose
			if openRatio >= 0.955 && openRatio <= 0.97 && prevIdx >= 60 {
				hist60 := records[prevIdx-60 : prevIdx+1]
				low60, high60 := hist60[0].Low, hist60[0].High
				for _, r := range hist60 {
					if r.Low < low60 {
						low60 = r.Low
					}
					if r.High > high60 {
						high60 = r.High
					}
				}
				// 详细输出每个条件的检查结果 (同步 SBDK 逻辑)
				ratio := todayRec.Close / mc.CurrentPrice
				totalMC := mc.TotalMarketCap * ratio
				rp := (prevClose - low60) / (high60 - low60)

				fmt.Printf("股票 %s(SBDK): 开盘比=%.3f(in[0.955, 0.97]?%t), 60日位置=%.3f(≤0.5?%t), 成交额=%.2f亿(≥1.0?%t), 市值总=%.2f\n",
					code, openRatio, (openRatio >= 0.955 && openRatio <= 0.97),
					rp, rp <= 0.5,
					prevRec.Amount/1e8, prevRec.Amount >= 1e8,
					totalMC)

				if high60 > low60 && rp <= 0.5 && prevRec.Amount >= 1e8 {
					resultsChan <- SelectedStock{Code: code, Name: mc.Name, Date: targetDate, Strategy: "First Board Low Open"}
					fmt.Printf("HIT: 首板低开 %s 选中!\n", code)
				}
			}
		}
	}
}

func (s *StockSelector) processWeakToStrong(code, targetDate string, resultsChan chan<- SelectedStock, preAuctionPrice, preAuctionVolume float64, isToday bool) {
	records, err := s.readStockCSV(code)
	if err != nil {
		return
	}
	var todayRec, prevRec *StockRecord
	var prevIdx int
	targetTime, _ := time.Parse("2006-01-02", targetDate)

	for i := range records {
		if records[i].Date.Equal(targetTime) {
			todayRec = &records[i]
		}
		if records[i].Date.Before(targetTime) {
			prevRec = &records[i]
			prevIdx = i
		}
	}

	if prevRec == nil || prevIdx < 4 {
		return
	}

	s.mu.RLock()
	mc, ok := s.marketCaps[code]
	s.mu.RUnlock()
	if !ok {
		return
	}

	past4 := records[prevIdx-3 : prevIdx+1]
	increaseRatio := (past4[3].Close - past4[0].Close) / past4[0].Close
	if increaseRatio > 0.28 {
		return
	}
	if (prevRec.Close-prevRec.Open)/prevRec.Open < -0.05 {
		return
	}

	var auctionPrice, auctionVolume float64
	if isToday && preAuctionPrice > 0 {
		auctionPrice, auctionVolume = preAuctionPrice, preAuctionVolume
	} else {
		auctionPrice, auctionVolume = s.getAuctionData(code, targetDate, isToday)
		if auctionPrice == 0 && todayRec != nil {
			auctionPrice = todayRec.Open
		}
	}

	if auctionPrice > 0 {
		prevClose := prevRec.Close
		prevVolume := s.adjustVolume(prevRec.Volume, prevRec.Amount, prevClose)

		limitRatio := 0.1
		if strings.HasPrefix(code, "30") {
			limitRatio = 0.2
		}
		limitPrice := math.Round(prevClose*(1+limitRatio)*100) / 100

		currentRatioToClose := auctionPrice / (limitPrice / 1.1)
		if currentRatioToClose >= 0.98 && currentRatioToClose <= 1.09 {
			volumeRatioWTS := auctionVolume / prevVolume
			if volumeRatioWTS >= 0.03 {
				avgPrice := prevRec.Amount / prevVolume
				avgPriceIncrease := avgPrice/prevClose - 1

				if avgPriceIncrease >= -0.04 && (prevRec.Amount >= 3e8 && prevRec.Amount <= 19e8) {
					latestPrice := prevClose
					if todayRec != nil && todayRec.Close > 0 {
						latestPrice = todayRec.Close
					} else if auctionPrice > 0 {
						latestPrice = auctionPrice
					}
					ratio := latestPrice / mc.CurrentPrice
					totalMC := mc.TotalMarketCap * ratio
					circMC := mc.CirculatingMarketCap * ratio

					if totalMC >= 70 && circMC <= 520 {
						if s.checkLeftPressure(records, prevIdx, prevVolume) {
							fmt.Printf("HIT: 弱转强 %s 选中!\n", code)
							resultsChan <- SelectedStock{Code: code, Name: mc.Name, Date: targetDate, Strategy: "Weak to Strong"}
						}
					}
				}
			}
		}
	}
}

func (s *StockSelector) checkLeftPressure(records []StockRecord, prevIdx int, prevVolume float64) bool {
	hst := records[:prevIdx+1]
	if len(hst) < 2 {
		return true
	}
	var hstTail []StockRecord
	if len(hst) > 101 {
		hstTail = hst[len(hst)-101:]
	} else {
		hstTail = hst
	}
	prevHigh := hstTail[len(hstTail)-1].High
	zyts0 := 100
	for i := len(hstTail) - 3; i >= 0; i-- {
		if hstTail[i].High >= prevHigh {
			zyts0 = len(hstTail) - 1 - i - 1
			break
		}
	}
	startIdx := len(hstTail) - (zyts0 + 5)
	if startIdx < 0 {
		startIdx = 0
	}
	maxPrevVol := 0.0
	for i := startIdx; i < len(hstTail)-1; i++ {
		// 历史成交量单位转换 (手 -> 股, 1手=100股)
		convertedVolume := float64(hstTail[i].Volume) * 100
		if convertedVolume > maxPrevVol {
			maxPrevVol = convertedVolume
		}
	}
	result := maxPrevVol == 0 || prevVolume > maxPrevVol*0.9
	return result
}
