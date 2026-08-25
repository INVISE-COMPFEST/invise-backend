package stocks

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"invise-backend/pkg/ai"
	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/ulid"
)

type StockUsecaseI interface {
	Import(ctx context.Context, userID string, salesFile, costFile, stockLevelFile io.Reader, salesFilename string) (*ImportResponse, error)
	ListStocks(ctx context.Context, userID string) ([]string, error)
	GetStockItems(ctx context.Context, userID, stockID string) ([]StockListItemResponse, error)
	GetItemDetail(ctx context.Context, userID, itemID string) (*ItemDetailResponse, error)
	GetItemDiagnose(ctx context.Context, userID, itemID string) (*ItemDiagnoseResponse, error)
	GetStockProjection(ctx context.Context, userID, stockID string, rangeVal int) ([]StockProjectionItemResponse, error)
}

type stockUsecase struct {
	repo     StockRepositoryI
	aiClient ai.AIClientI
	ulid     ulid.GeneratorI
}

func NewStockUsecase(repo StockRepositoryI, aiClient ai.AIClientI, ulid ulid.GeneratorI) StockUsecaseI {
	return &stockUsecase{
		repo:     repo,
		aiClient: aiClient,
		ulid:     ulid,
	}
}

type salesRecord struct {
	DateMonth    string
	ID           string
	ItemID       string
	StoreID      string
	DeptID       string
	CatID        string
	StateID      string
	MonthlySales float64
}

type itemCostInfo struct {
	Cost        float64
	ProductName string
	StoreID     string
	Month       string
}

type itemStockInfo struct {
	Quantity    int
	ProductName string
	StoreID     string
	Month       string
}

func (u *stockUsecase) Import(
	ctx context.Context,
	userID string,
	salesFile, costFile, stockLevelFile io.Reader,
	salesFilename string,
) (*ImportResponse, error) {
	if salesFile == nil || costFile == nil || stockLevelFile == nil {
		return nil, ErrMissingImportFiles
	}

	// 1. Read sales file into memory buffer for dual use: validation & AI forward
	var salesBuf bytes.Buffer
	if _, err := io.Copy(&salesBuf, salesFile); err != nil {
		return nil, pkgerr.BadRequest("INVALID_SALES_FILE", "failed to read monthly sales file")
	}
	if salesBuf.Len() == 0 {
		return nil, ErrEmptyCSV
	}

	// Validate and parse sales CSV
	salesRecords, err := parseSalesCSV(bytes.NewReader(salesBuf.Bytes()))
	if err != nil {
		return nil, err
	}

	// Parse unit cost / modal CSV
	costMap, err := parseCostCSV(costFile)
	if err != nil {
		return nil, err
	}

	// Parse stock level / inventory CSV
	stockMap, err := parseStockLevelCSV(stockLevelFile)
	if err != nil {
		return nil, err
	}

	// 2. Dispatch sales CSV to AI forecasting service
	aiResp, err := u.aiClient.Predict(ctx, bytes.NewReader(salesBuf.Bytes()), salesFilename, true, true, 10)
	if err != nil {
		return nil, err
	}

	// 3. Map predictions by item ID, ID, and series ID
	predictionMap := make(map[string]float64)
	for _, pred := range aiResp.Predictions {
		if pred.ItemID != "" {
			predictionMap[pred.ItemID] = pred.PredictedMonthlySales
		}
		if pred.ID != "" {
			predictionMap[pred.ID] = pred.PredictedMonthlySales
		}
		if pred.SeriesID != "" {
			predictionMap[pred.SeriesID] = pred.PredictedMonthlySales
		}
	}

	// Format feature importance reasons
	var reasons []DiagnoseReason
	for _, fi := range aiResp.FeatureImportance {
		name := fi.DisplayName
		if name == "" {
			name = fi.Feature
		}
		reasons = append(reasons, DiagnoseReason{
			Name:       name,
			Percentage: fi.ImportancePct,
		})
	}
	reasonsBytes, _ := json.Marshal(reasons)
	reasonsJSON := string(reasonsBytes)

	// 4. Build canonical item mappings and group historical sales points
	// aliasMap maps any identifier (item_id, id, series_id) to the canonical SKU key
	aliasMap := make(map[string]string)
	idToItem := make(map[string]string)
	itemHistory := make(map[string][]float64)
	itemStores := make(map[string]string)

	for _, rec := range salesRecords {
		canonicalKey := rec.ItemID
		if canonicalKey == "" {
			canonicalKey = rec.ID
		}
		if canonicalKey == "" {
			continue
		}

		if rec.ID != "" {
			aliasMap[rec.ID] = canonicalKey
			idToItem[canonicalKey] = rec.ID
		}
		if rec.ItemID != "" {
			aliasMap[rec.ItemID] = canonicalKey
		}

		itemHistory[canonicalKey] = append(itemHistory[canonicalKey], rec.MonthlySales)
		if rec.StoreID != "" {
			itemStores[canonicalKey] = rec.StoreID
		}
	}

	// Helper to resolve canonical SKU key
	resolveCanonical := func(key string) string {
		if can, ok := aliasMap[key]; ok && can != "" {
			return can
		}
		return key
	}

	// 5. Generate Stock ID
	stockID, err := u.ulid.Generate()
	if err != nil {
		return nil, pkgerr.InternalServerError("ID_GENERATION_FAILED", "could not generate stock ID")
	}

	forecastMonth := aiResp.ForecastMonth
	if forecastMonth == "" {
		forecastMonth = time.Now().Format("2006-01")
	}

	// Collect all unique canonical items from sales, cost, or stock records
	allItems := make(map[string]bool)
	for itemID := range itemHistory {
		allItems[itemID] = true
	}
	for key := range costMap {
		allItems[resolveCanonical(key)] = true
	}
	for key := range stockMap {
		allItems[resolveCanonical(key)] = true
	}

	// Helper to look up cost info for an item key
	getCostInfo := func(key string) itemCostInfo {
		if info, ok := costMap[key]; ok {
			return info
		}
		if altID, ok := idToItem[key]; ok {
			if info, ok := costMap[altID]; ok {
				return info
			}
		}
		for k, info := range costMap {
			if resolveCanonical(k) == key {
				return info
			}
		}
		return itemCostInfo{}
	}

	// Helper to look up stock info for an item key
	getStockInfo := func(key string) itemStockInfo {
		if info, ok := stockMap[key]; ok {
			return info
		}
		if altID, ok := idToItem[key]; ok {
			if info, ok := stockMap[altID]; ok {
				return info
			}
		}
		for k, info := range stockMap {
			if resolveCanonical(k) == key {
				return info
			}
		}
		return itemStockInfo{}
	}

	// Helper to look up predicted sales for an item key
	getPredictedSales := func(key string) float64 {
		if p, ok := predictionMap[key]; ok {
			return p
		}
		if altID, ok := idToItem[key]; ok {
			if p, ok := predictionMap[altID]; ok {
				return p
			}
		}
		for k, p := range predictionMap {
			if resolveCanonical(k) == key {
				return p
			}
		}
		return 0.0
	}

	// Calculate overall average price across items
	var totalPrice float64
	var priceCount int
	for itemID := range allItems {
		costInfo := getCostInfo(itemID)
		if costInfo.Cost > 0 {
			totalPrice += costInfo.Cost
			priceCount++
		}
	}
	marketAvg := 0.0
	if priceCount > 0 {
		marketAvg = math.Round((totalPrice/float64(priceCount))*100) / 100
	}

	// Sort item keys deterministically
	var sortedItemKeys []string
	for itemID := range allItems {
		sortedItemKeys = append(sortedItemKeys, itemID)
	}
	sort.Strings(sortedItemKeys)

	var items []Item
	var sumPredictedSales float64

	for _, itemID := range sortedItemKeys {
		itemIDVal, err := u.ulid.Generate()
		if err != nil {
			return nil, pkgerr.InternalServerError("ID_GENERATION_FAILED", "could not generate item ID")
		}

		costInfo := getCostInfo(itemID)
		stockInfo := getStockInfo(itemID)

		qty := stockInfo.Quantity
		unitCost := costInfo.Cost
		valueLocked := math.Round((float64(qty)*unitCost)*100) / 100
		predictedSales := getPredictedSales(itemID)
		sumPredictedSales += predictedSales

		history := itemHistory[itemID]

		// Deadstock Status determination
		deadstockStatus := "HEALTHY"
		if qty > 0 && predictedSales <= 0.1*float64(qty) {
			deadstockStatus = "DEADSTOCK"
		} else if qty > 0 && predictedSales < 0.5*float64(qty) {
			deadstockStatus = "SLOW_MOVING"
		}

		// Decision determination
		decision := "HOLD"
		if deadstockStatus == "DEADSTOCK" {
			decision = "LIQUIDATE"
		} else if deadstockStatus == "SLOW_MOVING" {
			decision = "DISCOUNT"
		} else if predictedSales > float64(qty) {
			decision = "RESTOCK"
		}

		opportunityCost := math.Round((valueLocked*0.15)*100) / 100

		// Build projection points: [ [month_index, sales_value], ... ]
		var points [][]float64
		for idx, h := range history {
			points = append(points, []float64{float64(idx + 1), math.Round(h*100) / 100})
		}
		// Append forecast point
		points = append(points, []float64{float64(len(history) + 1), math.Round(predictedSales*100) / 100})
		pointsBytes, _ := json.Marshal(points)

		// Determine product name: prefer CSV product_name, then fallback to formatItemName
		name := costInfo.ProductName
		if name == "" {
			name = stockInfo.ProductName
		}
		if name == "" {
			name = formatItemName(itemID)
		}

		// Determine store ID
		storeID := itemStores[itemID]
		if storeID == "" {
			storeID = stockInfo.StoreID
		}
		if storeID == "" {
			storeID = costInfo.StoreID
		}

		items = append(items, Item{
			ID:                   itemIDVal,
			StockID:              stockID,
			SKU:                  itemID,
			Name:                 name,
			StoreID:              storeID,
			Quantity:             qty,
			UnitCost:             unitCost,
			ValueLocked:          valueLocked,
			CurrentPrice:         unitCost,
			DeadstockStatus:      deadstockStatus,
			OpportunityCost:      opportunityCost,
			MarketAverage:        marketAvg,
			PredictedSales:       predictedSales,
			Decision:             decision,
			ReasonsJSON:          reasonsJSON,
			ProjectionPointsJSON: string(pointsBytes),
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		})
	}

	totalForecast := aiResp.Summary.TotalForecast
	meanForecast := aiResp.Summary.MeanForecast
	if totalForecast == 0 && len(items) > 0 {
		totalForecast = math.Round(sumPredictedSales*100) / 100
		meanForecast = math.Round((sumPredictedSales/float64(len(items)))*100) / 100
	}

	stock := &Stock{
		ID:            stockID,
		UserID:        userID,
		Name:          fmt.Sprintf("Stock Batch %s", time.Now().Format("2006-01-02 15:04")),
		ForecastMonth: forecastMonth,
		TotalForecast: totalForecast,
		MeanForecast:  meanForecast,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 6. Save in database
	if err := u.repo.CreateStockWithItems(ctx, stock, items); err != nil {
		return nil, pkgerr.InternalServerError("IMPORT_FAILED", "failed to save imported stock and items")
	}

	return &ImportResponse{
		StockID:       stockID,
		ItemCount:     len(items),
		ForecastMonth: forecastMonth,
	}, nil
}

func (u *stockUsecase) ListStocks(ctx context.Context, userID string) ([]string, error) {
	stocks, err := u.repo.ListStocksByUserID(ctx, userID)
	if err != nil {
		return nil, pkgerr.InternalServerError("DATABASE_ERROR", "failed to list stocks")
	}

	result := make([]string, len(stocks))
	for i, s := range stocks {
		result[i] = s.ID
	}
	return result, nil
}

func (u *stockUsecase) GetStockItems(ctx context.Context, userID, stockID string) ([]StockListItemResponse, error) {
	items, err := u.repo.FindItemsByStockID(ctx, userID, stockID)
	if err != nil {
		if errorsIs(err, ErrStockNotFound) {
			return nil, ErrStockNotFound
		}
		return nil, pkgerr.InternalServerError("DATABASE_ERROR", "failed to retrieve stock items")
	}

	res := make([]StockListItemResponse, len(items))
	for i, it := range items {
		res[i] = StockListItemResponse{
			SKU:             it.SKU,
			Name:            it.Name,
			Quantity:        it.Quantity,
			ValueLocked:     it.ValueLocked,
			DeadstockStatus: it.DeadstockStatus,
			ItemsID:         it.ID,
		}
	}
	return res, nil
}

func (u *stockUsecase) GetItemDetail(ctx context.Context, userID, itemID string) (*ItemDetailResponse, error) {
	item, err := u.repo.FindItemByID(ctx, userID, itemID)
	if err != nil {
		return nil, pkgerr.InternalServerError("DATABASE_ERROR", "failed to retrieve item details")
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	return &ItemDetailResponse{
		SKU:          item.SKU,
		Name:         item.Name,
		Quantity:     item.Quantity,
		ValueLocked:  item.ValueLocked,
		CurrentPrice: item.CurrentPrice,
		StocksID:     item.StockID,
	}, nil
}

func (u *stockUsecase) GetItemDiagnose(ctx context.Context, userID, itemID string) (*ItemDiagnoseResponse, error) {
	item, err := u.repo.FindItemByID(ctx, userID, itemID)
	if err != nil {
		return nil, pkgerr.InternalServerError("DATABASE_ERROR", "failed to retrieve item diagnose")
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	var reasons []DiagnoseReason
	if item.ReasonsJSON != "" {
		_ = json.Unmarshal([]byte(item.ReasonsJSON), &reasons)
	}

	return &ItemDiagnoseResponse{
		DeadstockStatus: item.DeadstockStatus,
		OpportunityCost: item.OpportunityCost,
		MarketAverage:   item.MarketAverage,
		Reasons:         reasons,
	}, nil
}

func (u *stockUsecase) GetStockProjection(ctx context.Context, userID, stockID string, rangeVal int) ([]StockProjectionItemResponse, error) {
	items, err := u.repo.FindItemsByStockID(ctx, userID, stockID)
	if err != nil {
		if errorsIs(err, ErrStockNotFound) {
			return nil, ErrStockNotFound
		}
		return nil, pkgerr.InternalServerError("DATABASE_ERROR", "failed to retrieve stock projection")
	}

	res := make([]StockProjectionItemResponse, len(items))
	for i, it := range items {
		var points [][]float64
		if it.ProjectionPointsJSON != "" {
			_ = json.Unmarshal([]byte(it.ProjectionPointsJSON), &points)
		}

		// Calculate projection percentage
		var projPct float64
		if it.Quantity > 0 {
			projPct = math.Round(((it.PredictedSales-float64(it.Quantity))/float64(it.Quantity))*10000) / 100
		} else if it.PredictedSales > 0 {
			projPct = 100.0
		}

		res[i] = StockProjectionItemResponse{
			SKU:                  it.SKU,
			Name:                 it.Name,
			ProjectionPercentage: projPct,
			ProjectionPoints:     points,
			Decision:             it.Decision,
		}
	}
	return res, nil
}


// Helpers

func parseSalesCSV(r io.Reader) ([]salesRecord, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, ErrInvalidCSVFormat
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		clean := strings.ToLower(strings.TrimSpace(h))
		headerMap[clean] = i
	}

	dateMonthIdx, hasDate := headerMap["date_month"]
	if !hasDate {
		dateMonthIdx, hasDate = headerMap["month"]
	}
	if !hasDate {
		dateMonthIdx, hasDate = headerMap["date"]
	}
	if !hasDate {
		dateMonthIdx, hasDate = headerMap["period"]
	}

	idIdx, hasID := headerMap["id"]
	if !hasID {
		idIdx, hasID = headerMap["series_id"]
	}

	itemIDIdx, hasItem := headerMap["item_id"]
	if !hasItem {
		itemIDIdx, hasItem = headerMap["sku"]
	}
	if !hasItem {
		itemIDIdx, hasItem = headerMap["product_id"]
	}
	if !hasItem && hasID {
		itemIDIdx = idIdx
		hasItem = true
	}
	if !hasID && hasItem {
		idIdx = itemIDIdx
		hasID = true
	}

	storeIDIdx, hasStore := headerMap["store_id"]
	if !hasStore {
		storeIDIdx, hasStore = headerMap["store"]
	}

	monthlySalesIdx, hasSales := headerMap["monthly_sales"]
	if !hasSales {
		monthlySalesIdx, hasSales = headerMap["sales"]
	}
	if !hasSales {
		monthlySalesIdx, hasSales = headerMap["quantity"]
	}
	if !hasSales {
		monthlySalesIdx, hasSales = headerMap["qty"]
	}

	deptIDIdx, hasDept := headerMap["dept_id"]
	if !hasDept {
		deptIDIdx, hasDept = headerMap["dept"]
	}

	catIDIdx, hasCat := headerMap["cat_id"]
	if !hasCat {
		catIDIdx, hasCat = headerMap["category"]
	}
	if !hasCat {
		catIDIdx, hasCat = headerMap["cat"]
	}

	stateIDIdx, hasState := headerMap["state_id"]
	if !hasState {
		stateIDIdx, hasState = headerMap["state"]
	}

	if !hasDate || !hasItem || !hasSales {
		return nil, ErrInvalidCSVFormat
	}

	var records []salesRecord
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidCSVFormat
		}
		if len(row) <= dateMonthIdx || len(row) <= itemIDIdx || len(row) <= monthlySalesIdx {
			continue
		}

		sales, err := strconv.ParseFloat(strings.TrimSpace(row[monthlySalesIdx]), 64)
		if err != nil {
			sales = 0
		}

		var idVal, itemIDVal, storeIDVal, deptIDVal, catIDVal, stateIDVal string
		if hasID && len(row) > idIdx {
			idVal = strings.TrimSpace(row[idIdx])
		}
		if hasItem && len(row) > itemIDIdx {
			itemIDVal = strings.TrimSpace(row[itemIDIdx])
		}
		if hasStore && len(row) > storeIDIdx {
			storeIDVal = strings.TrimSpace(row[storeIDIdx])
		}
		if hasDept && len(row) > deptIDIdx {
			deptIDVal = strings.TrimSpace(row[deptIDIdx])
		}
		if hasCat && len(row) > catIDIdx {
			catIDVal = strings.TrimSpace(row[catIDIdx])
		}
		if hasState && len(row) > stateIDIdx {
			stateIDVal = strings.TrimSpace(row[stateIDIdx])
		}

		records = append(records, salesRecord{
			DateMonth:    strings.TrimSpace(row[dateMonthIdx]),
			ID:           idVal,
			ItemID:       itemIDVal,
			StoreID:      storeIDVal,
			DeptID:       deptIDVal,
			CatID:        catIDVal,
			StateID:      stateIDVal,
			MonthlySales: sales,
		})
	}

	if len(records) == 0 {
		return nil, ErrEmptyCSV
	}

	return records, nil
}

func parseCostCSV(r io.Reader) (map[string]itemCostInfo, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, ErrInvalidCSVFormat
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	idIdx, hasID := headerMap["id"]
	itemIdx, hasItem := headerMap["item_id"]
	if !hasItem {
		itemIdx, hasItem = headerMap["sku"]
	}
	if !hasItem {
		itemIdx, hasItem = headerMap["product_id"]
	}
	if !hasItem && hasID {
		itemIdx = idIdx
		hasItem = true
	}
	if !hasID && hasItem {
		idIdx = itemIdx
		hasID = true
	}

	costIdx, hasCost := headerMap["modal"]
	if !hasCost {
		costIdx, hasCost = headerMap["unit_cost"]
	}
	if !hasCost {
		costIdx, hasCost = headerMap["cost"]
	}
	if !hasCost {
		costIdx, hasCost = headerMap["sell_price"]
	}
	if !hasCost {
		costIdx, hasCost = headerMap["price"]
	}
	if !hasCost {
		costIdx, hasCost = headerMap["unit_price"]
	}

	monthIdx, hasMonth := headerMap["month"]
	if !hasMonth {
		monthIdx, hasMonth = headerMap["date_month"]
	}
	if !hasMonth {
		monthIdx, hasMonth = headerMap["date"]
	}

	nameIdx, hasName := headerMap["product_name"]
	if !hasName {
		nameIdx, hasName = headerMap["name"]
	}
	if !hasName {
		nameIdx, hasName = headerMap["item_name"]
	}
	if !hasName {
		nameIdx, hasName = headerMap["title"]
	}

	storeIdx, hasStore := headerMap["store"]
	if !hasStore {
		storeIdx, hasStore = headerMap["store_id"]
	}

	if !hasItem || !hasCost {
		return nil, ErrInvalidCSVFormat
	}

	costMap := make(map[string]itemCostInfo)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidCSVFormat
		}
		if len(row) <= itemIdx || len(row) <= costIdx {
			continue
		}

		key := strings.TrimSpace(row[itemIdx])
		if key == "" && hasID && len(row) > idIdx {
			key = strings.TrimSpace(row[idIdx])
		}
		if key == "" {
			continue
		}

		cost, _ := strconv.ParseFloat(strings.TrimSpace(row[costIdx]), 64)

		var monthVal, nameVal, storeVal string
		if hasMonth && len(row) > monthIdx {
			monthVal = strings.TrimSpace(row[monthIdx])
		}
		if hasName && len(row) > nameIdx {
			nameVal = strings.TrimSpace(row[nameIdx])
		}
		if hasStore && len(row) > storeIdx {
			storeVal = strings.TrimSpace(row[storeIdx])
		}

		existing, exists := costMap[key]
		if !exists || monthVal == "" || monthVal >= existing.Month {
			costMap[key] = itemCostInfo{
				Cost:        cost,
				ProductName: nameVal,
				StoreID:     storeVal,
				Month:       monthVal,
			}
		}
		// Also record by idKey if distinct
		if hasID && len(row) > idIdx {
			idKey := strings.TrimSpace(row[idIdx])
			if idKey != "" && idKey != key {
				existingID, existsID := costMap[idKey]
				if !existsID || monthVal == "" || monthVal >= existingID.Month {
					costMap[idKey] = itemCostInfo{
						Cost:        cost,
						ProductName: nameVal,
						StoreID:     storeVal,
						Month:       monthVal,
					}
				}
			}
		}
	}
	return costMap, nil
}

func parseStockLevelCSV(r io.Reader) (map[string]itemStockInfo, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, ErrInvalidCSVFormat
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	idIdx, hasID := headerMap["id"]
	itemIdx, hasItem := headerMap["item_id"]
	if !hasItem {
		itemIdx, hasItem = headerMap["sku"]
	}
	if !hasItem {
		itemIdx, hasItem = headerMap["product_id"]
	}
	if !hasItem && hasID {
		itemIdx = idIdx
		hasItem = true
	}
	if !hasID && hasItem {
		idIdx = itemIdx
		hasID = true
	}

	qtyIdx, hasQty := headerMap["inventory"]
	if !hasQty {
		qtyIdx, hasQty = headerMap["quantity"]
	}
	if !hasQty {
		qtyIdx, hasQty = headerMap["stock_level"]
	}
	if !hasQty {
		qtyIdx, hasQty = headerMap["stock"]
	}
	if !hasQty {
		qtyIdx, hasQty = headerMap["qty"]
	}
	if !hasQty {
		qtyIdx, hasQty = headerMap["stock_quantity"]
	}

	monthIdx, hasMonth := headerMap["month"]
	if !hasMonth {
		monthIdx, hasMonth = headerMap["date_month"]
	}
	if !hasMonth {
		monthIdx, hasMonth = headerMap["date"]
	}

	nameIdx, hasName := headerMap["product_name"]
	if !hasName {
		nameIdx, hasName = headerMap["name"]
	}
	if !hasName {
		nameIdx, hasName = headerMap["item_name"]
	}
	if !hasName {
		nameIdx, hasName = headerMap["title"]
	}

	storeIdx, hasStore := headerMap["store"]
	if !hasStore {
		storeIdx, hasStore = headerMap["store_id"]
	}

	if !hasItem || !hasQty {
		return nil, ErrInvalidCSVFormat
	}

	stockMap := make(map[string]itemStockInfo)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidCSVFormat
		}
		if len(row) <= itemIdx || len(row) <= qtyIdx {
			continue
		}

		key := strings.TrimSpace(row[itemIdx])
		if key == "" && hasID && len(row) > idIdx {
			key = strings.TrimSpace(row[idIdx])
		}
		if key == "" {
			continue
		}

		qty, _ := strconv.Atoi(strings.TrimSpace(row[qtyIdx]))

		var monthVal, nameVal, storeVal string
		if hasMonth && len(row) > monthIdx {
			monthVal = strings.TrimSpace(row[monthIdx])
		}
		if hasName && len(row) > nameIdx {
			nameVal = strings.TrimSpace(row[nameIdx])
		}
		if hasStore && len(row) > storeIdx {
			storeVal = strings.TrimSpace(row[storeIdx])
		}

		existing, exists := stockMap[key]
		if !exists || monthVal == "" || monthVal >= existing.Month {
			stockMap[key] = itemStockInfo{
				Quantity:    qty,
				ProductName: nameVal,
				StoreID:     storeVal,
				Month:       monthVal,
			}
		}
		// Also record by idKey if distinct
		if hasID && len(row) > idIdx {
			idKey := strings.TrimSpace(row[idIdx])
			if idKey != "" && idKey != key {
				existingID, existsID := stockMap[idKey]
				if !existsID || monthVal == "" || monthVal >= existingID.Month {
					stockMap[idKey] = itemStockInfo{
						Quantity:    qty,
						ProductName: nameVal,
						StoreID:     storeVal,
						Month:       monthVal,
					}
				}
			}
		}
	}
	return stockMap, nil
}

func formatItemName(sku string) string {
	parts := strings.Split(sku, "_")
	if len(parts) >= 2 {
		return fmt.Sprintf("Item %s #%s", strings.Title(strings.ToLower(parts[0])), parts[len(parts)-1])
	}
	return fmt.Sprintf("Item %s", sku)
}

func errorsIs(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}
	return err.Error() == target.Error()
}

