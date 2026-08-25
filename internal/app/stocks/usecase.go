package stocks

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	GetMarketContext(ctx context.Context, userID string) (*MarketContextResponse, error)
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
	StoreID      string
	ItemID       string
	MonthlySales float64
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

	// Parse unit cost CSV
	costMap, err := parseCostCSV(costFile)
	if err != nil {
		return nil, err
	}

	// Parse stock level CSV
	stockMap, err := parseStockLevelCSV(stockLevelFile)
	if err != nil {
		return nil, err
	}

	// 2. Dispatch sales CSV to AI forecasting service
	aiResp, err := u.aiClient.Predict(ctx, bytes.NewReader(salesBuf.Bytes()), salesFilename, true, true, 10)
	if err != nil {
		return nil, err
	}

	// 3. Map predictions by item ID
	predictionMap := make(map[string]float64)
	for _, pred := range aiResp.Predictions {
		predictionMap[pred.ItemID] = pred.PredictedMonthlySales
		if pred.ID != "" {
			predictionMap[pred.ID] = pred.PredictedMonthlySales
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

	// 4. Group historical sales points by item
	itemHistory := make(map[string][]float64)
	itemStores := make(map[string]string)
	for _, rec := range salesRecords {
		itemHistory[rec.ItemID] = append(itemHistory[rec.ItemID], rec.MonthlySales)
		if rec.StoreID != "" {
			itemStores[rec.ItemID] = rec.StoreID
		}
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

	stock := &Stock{
		ID:            stockID,
		UserID:        userID,
		Name:          fmt.Sprintf("Stock Batch %s", time.Now().Format("2006-01-02 15:04")),
		ForecastMonth: forecastMonth,
		TotalForecast: aiResp.Summary.TotalForecast,
		MeanForecast:  aiResp.Summary.MeanForecast,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Collect unique items from sales, cost, or stock records
	allItems := make(map[string]bool)
	for itemID := range itemHistory {
		allItems[itemID] = true
	}
	for itemID := range costMap {
		allItems[itemID] = true
	}
	for itemID := range stockMap {
		allItems[itemID] = true
	}

	// Calculate overall average price across items
	var totalPrice float64
	var priceCount int
	for _, price := range costMap {
		totalPrice += price
		priceCount++
	}
	marketAvg := 0.0
	if priceCount > 0 {
		marketAvg = math.Round((totalPrice/float64(priceCount))*100) / 100
	}

	var items []Item
	for itemID := range allItems {
		itemIDVal, err := u.ulid.Generate()
		if err != nil {
			return nil, pkgerr.InternalServerError("ID_GENERATION_FAILED", "could not generate item ID")
		}

		qty := stockMap[itemID]
		unitCost := costMap[itemID]
		valueLocked := math.Round((float64(qty)*unitCost)*100) / 100
		predictedSales := predictionMap[itemID]

		history := itemHistory[itemID]
		var avgSales float64
		if len(history) > 0 {
			var sumSales float64
			for _, s := range history {
				sumSales += s
			}
			avgSales = sumSales / float64(len(history))
		}

		// Estimate days in stock & last sale days
		daysInStock := 30
		if avgSales > 0 && qty > 0 {
			daysInStock = int((float64(qty) / avgSales) * 30)
		} else if qty > 0 {
			daysInStock = 90
		}

		lastSaleDays := 15
		if len(history) > 0 && history[len(history)-1] == 0 {
			lastSaleDays = 45
		}

		// Deadstock Status determination
		deadstockStatus := "HEALTHY"
		if qty > 0 && (predictedSales <= 0.1*float64(qty) || daysInStock > 90 || lastSaleDays > 60) {
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

		name := formatItemName(itemID)
		storeID := itemStores[itemID]

		items = append(items, Item{
			ID:                   itemIDVal,
			StockID:              stockID,
			SKU:                  itemID,
			Name:                 name,
			StoreID:              storeID,
			Quantity:             qty,
			UnitCost:             unitCost,
			ValueLocked:          valueLocked,
			DaysInStock:          daysInStock,
			LastSaleDays:         lastSaleDays,
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
		DaysInStock:  item.DaysInStock,
		LastSaleDays: item.LastSaleDays,
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

func (u *stockUsecase) GetMarketContext(ctx context.Context, userID string) (*MarketContextResponse, error) {
	stock, err := u.repo.FindLatestStockByUserID(ctx, userID)
	if err != nil {
		return nil, pkgerr.InternalServerError("DATABASE_ERROR", "failed to retrieve market context")
	}

	if stock == nil || len(stock.Items) == 0 {
		return &MarketContextResponse{
			Context:    "Overall market inventory trends remain balanced with healthy turnover expectations across standard categories.",
			Confidence: 0.85,
		}, nil
	}

	var deadstockCount, totalItems int
	for _, it := range stock.Items {
		totalItems++
		if it.DeadstockStatus == "DEADSTOCK" || it.DeadstockStatus == "SLOW_MOVING" {
			deadstockCount++
		}
	}

	deadstockRatio := float64(deadstockCount) / float64(totalItems)
	var narrative string
	confidence := 0.88

	if deadstockRatio > 0.4 {
		narrative = fmt.Sprintf("High inventory retention observed in %d%% of catalog items. Market demand signals recommend targeted discount campaigns and liquidation to mitigate carrying costs.", int(deadstockRatio*100))
		confidence = 0.92
	} else {
		narrative = fmt.Sprintf("Demand forecasting for %s indicates steady consumer traction with %d healthy performing items. Recommended actions focus on timely restocking of core product lines.", stock.ForecastMonth, totalItems-deadstockCount)
	}

	return &MarketContextResponse{
		Context:    narrative,
		Confidence: confidence,
	}, nil
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

	dateMonthIdx, ok1 := headerMap["date_month"]
	storeIDIdx, ok2 := headerMap["store_id"]
	itemIDIdx, ok3 := headerMap["item_id"]
	monthlySalesIdx, ok4 := headerMap["monthly_sales"]

	if !ok1 || !ok2 || !ok3 || !ok4 {
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
		if len(row) <= monthlySalesIdx || len(row) <= itemIDIdx || len(row) <= dateMonthIdx || len(row) <= storeIDIdx {
			continue
		}

		sales, _ := strconv.ParseFloat(strings.TrimSpace(row[monthlySalesIdx]), 64)
		records = append(records, salesRecord{
			DateMonth:    strings.TrimSpace(row[dateMonthIdx]),
			StoreID:      strings.TrimSpace(row[storeIDIdx]),
			ItemID:       strings.TrimSpace(row[itemIDIdx]),
			MonthlySales: sales,
		})
	}

	if len(records) == 0 {
		return nil, ErrEmptyCSV
	}

	return records, nil
}

func parseCostCSV(r io.Reader) (map[string]float64, error) {
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

	itemIdx, hasItem := headerMap["item_id"]
	if !hasItem {
		itemIdx, hasItem = headerMap["sku"]
	}

	costIdx, hasCost := headerMap["unit_cost"]
	if !hasCost {
		costIdx, hasCost = headerMap["sell_price"]
	}
	if !hasCost {
		costIdx, hasCost = headerMap["cost"]
	}
	if !hasCost {
		costIdx, hasCost = headerMap["price"]
	}

	if !hasItem || !hasCost {
		return nil, ErrInvalidCSVFormat
	}

	costMap := make(map[string]float64)
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
		cost, _ := strconv.ParseFloat(strings.TrimSpace(row[costIdx]), 64)
		costMap[strings.TrimSpace(row[itemIdx])] = cost
	}
	return costMap, nil
}

func parseStockLevelCSV(r io.Reader) (map[string]int, error) {
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

	itemIdx, hasItem := headerMap["item_id"]
	if !hasItem {
		itemIdx, hasItem = headerMap["sku"]
	}

	qtyIdx, hasQty := headerMap["quantity"]
	if !hasQty {
		qtyIdx, hasQty = headerMap["stock_level"]
	}
	if !hasQty {
		qtyIdx, hasQty = headerMap["stock"]
	}

	if !hasItem || !hasQty {
		return nil, ErrInvalidCSVFormat
	}

	stockMap := make(map[string]int)
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
		qty, _ := strconv.Atoi(strings.TrimSpace(row[qtyIdx]))
		stockMap[strings.TrimSpace(row[itemIdx])] = qty
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
