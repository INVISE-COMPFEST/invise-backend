package stocks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"invise-backend/internal/app/stocks"
	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/response"
)

type stubStockUsecase struct {
	importFn             func(ctx context.Context, userID string, salesFile, costFile, stockLevelFile io.Reader, salesFilename string) (*stocks.ImportResponse, error)
	listStocksFn         func(ctx context.Context, userID string) ([]string, error)
	getStockItemsFn      func(ctx context.Context, userID, stockID string) ([]stocks.StockListItemResponse, error)
	getItemDetailFn      func(ctx context.Context, userID, itemID string) (*stocks.ItemDetailResponse, error)
	getItemDiagnoseFn    func(ctx context.Context, userID, itemID string) (*stocks.ItemDiagnoseResponse, error)
	getStockProjectionFn func(ctx context.Context, userID, stockID string, rangeVal int) ([]stocks.StockProjectionItemResponse, error)
}

func (s *stubStockUsecase) Import(ctx context.Context, userID string, salesFile, costFile, stockLevelFile io.Reader, salesFilename string) (*stocks.ImportResponse, error) {
	if s.importFn != nil {
		return s.importFn(ctx, userID, salesFile, costFile, stockLevelFile, salesFilename)
	}
	return &stocks.ImportResponse{StockID: "stock-123", ItemCount: 1, ForecastMonth: "2016-06"}, nil
}

func (s *stubStockUsecase) ListStocks(ctx context.Context, userID string) ([]string, error) {
	if s.listStocksFn != nil {
		return s.listStocksFn(ctx, userID)
	}
	return []string{"stock-123"}, nil
}

func (s *stubStockUsecase) GetStockItems(ctx context.Context, userID, stockID string) ([]stocks.StockListItemResponse, error) {
	if s.getStockItemsFn != nil {
		return s.getStockItemsFn(ctx, userID, stockID)
	}
	return []stocks.StockListItemResponse{
		{
			SKU:             "FOODS_1_035",
			Name:            "Item Foods #035",
			Quantity:        20,
			ValueLocked:     50.0,
			DeadstockStatus: "HEALTHY",
			ItemsID:         "item-123",
		},
	}, nil
}

func (s *stubStockUsecase) GetItemDetail(ctx context.Context, userID, itemID string) (*stocks.ItemDetailResponse, error) {
	if s.getItemDetailFn != nil {
		return s.getItemDetailFn(ctx, userID, itemID)
	}
	return &stocks.ItemDetailResponse{
		SKU:          "FOODS_1_035",
		Name:         "Item Foods #035",
		Quantity:     20,
		ValueLocked:  50.0,
		DaysInStock:  15,
		LastSaleDays: 5,
		CurrentPrice: 2.50,
		StocksID:     "stock-123",
	}, nil
}

func (s *stubStockUsecase) GetItemDiagnose(ctx context.Context, userID, itemID string) (*stocks.ItemDiagnoseResponse, error) {
	if s.getItemDiagnoseFn != nil {
		return s.getItemDiagnoseFn(ctx, userID, itemID)
	}
	return &stocks.ItemDiagnoseResponse{
		DeadstockStatus: "HEALTHY",
		OpportunityCost: 7.50,
		MarketAverage:   2.50,
		Reasons: []stocks.DiagnoseReason{
			{Name: "Riwayat Penjualan", Percentage: 45.0},
		},
	}, nil
}

func (s *stubStockUsecase) GetStockProjection(ctx context.Context, userID, stockID string, rangeVal int) ([]stocks.StockProjectionItemResponse, error) {
	if s.getStockProjectionFn != nil {
		return s.getStockProjectionFn(ctx, userID, stockID, rangeVal)
	}
	return []stocks.StockProjectionItemResponse{
		{
			SKU:                  "FOODS_1_035",
			Name:                 "Item Foods #035",
			ProjectionPercentage: 15.0,
			ProjectionPoints:     [][]float64{{1, 10}, {2, 20}},
			Decision:             "RESTOCK",
		},
	}, nil
}


func testErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "internal server error"

	var appErr *pkgerr.AppError
	if errors.As(err, &appErr) {
		code = appErr.StatusCode
		message = appErr.Message
	}

	return c.Status(code).JSON(dto.Response[any]{
		Message: message,
	})
}

func setupStockTestApp(h *stocks.StockHandler, mockUserID string) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: testErrorHandler,
	})

	app.Use(func(c fiber.Ctx) error {
		if mockUserID != "" {
			c.Locals("user_id", mockUserID)
		}
		return c.Next()
	})

	app.Post("/api/v1/stocks/import", h.Import)
	app.Get("/api/v1/stocks", h.ListStocks)
	app.Get("/api/v1/stocks/items/:items_id", h.GetItemDetail)
	app.Get("/api/v1/stocks/items/:items_id/diagnose", h.GetItemDiagnose)
	app.Get("/api/v1/stocks/:stock_id/projection", h.GetStockProjection)
	app.Get("/api/v1/stocks/:stock_id", h.GetStockItems)

	return app
}

func TestStockHandler_Import(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		uc := &stubStockUsecase{}
		handler := stocks.NewStockHandler(uc)
		app := setupStockTestApp(handler, "user-123")

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		p1, _ := writer.CreateFormFile("monthly_sales_data", "sales.csv")
		_, _ = p1.Write([]byte("date_month,store_id,item_id,monthly_sales\n2016-05,CA_1,FOODS_1_035,10\n"))

		p2, _ := writer.CreateFormFile("unit_cost_data", "cost.csv")
		_, _ = p2.Write([]byte("item_id,unit_cost\nFOODS_1_035,2.50\n"))

		p3, _ := writer.CreateFormFile("stock_level_data", "stock.csv")
		_, _ = p3.Write([]byte("item_id,quantity\nFOODS_1_035,20\n"))

		_ = writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/stocks/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

		var resBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "stock data imported and analyzed successfully", resBody["message"])
	})

	t.Run("Missing Files", func(t *testing.T) {
		uc := &stubStockUsecase{}
		handler := stocks.NewStockHandler(uc)
		app := setupStockTestApp(handler, "user-123")

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/stocks/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

func TestStockHandler_ListStocks(t *testing.T) {
	uc := &stubStockUsecase{}
	handler := stocks.NewStockHandler(uc)
	app := setupStockTestApp(handler, "user-123")

	req, _ := http.NewRequest("GET", "/api/v1/stocks", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody struct {
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.Equal(t, "stocks retrieved successfully", resBody.Message)
	assert.Equal(t, []string{"stock-123"}, resBody.Data)
}

func TestStockHandler_GetStockItems(t *testing.T) {
	uc := &stubStockUsecase{}
	handler := stocks.NewStockHandler(uc)
	app := setupStockTestApp(handler, "user-123")

	req, _ := http.NewRequest("GET", "/api/v1/stocks/stock-123", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody struct {
		Message string                         `json:"message"`
		Data    []stocks.StockListItemResponse `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.Equal(t, "stock items retrieved successfully", resBody.Message)
	assert.Len(t, resBody.Data, 1)
	assert.Equal(t, "FOODS_1_035", resBody.Data[0].SKU)
}

func TestStockHandler_GetItemDetail(t *testing.T) {
	uc := &stubStockUsecase{}
	handler := stocks.NewStockHandler(uc)
	app := setupStockTestApp(handler, "user-123")

	req, _ := http.NewRequest("GET", "/api/v1/stocks/items/item-123", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody struct {
		Message string                    `json:"message"`
		Data    stocks.ItemDetailResponse `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.Equal(t, "item details retrieved successfully", resBody.Message)
	assert.Equal(t, "FOODS_1_035", resBody.Data.SKU)
	assert.Equal(t, 2.50, resBody.Data.CurrentPrice)
}

func TestStockHandler_GetItemDiagnose(t *testing.T) {
	uc := &stubStockUsecase{}
	handler := stocks.NewStockHandler(uc)
	app := setupStockTestApp(handler, "user-123")

	req, _ := http.NewRequest("GET", "/api/v1/stocks/items/item-123/diagnose", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody struct {
		Message string                      `json:"message"`
		Data    stocks.ItemDiagnoseResponse `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.Equal(t, "item diagnose retrieved successfully", resBody.Message)
	assert.Equal(t, "HEALTHY", resBody.Data.DeadstockStatus)
	assert.Len(t, resBody.Data.Reasons, 1)
}

func TestStockHandler_GetStockProjection(t *testing.T) {
	uc := &stubStockUsecase{}
	handler := stocks.NewStockHandler(uc)
	app := setupStockTestApp(handler, "user-123")

	req, _ := http.NewRequest("GET", "/api/v1/stocks/stock-123/projection?range=3", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody struct {
		Message string                               `json:"message"`
		Data    []stocks.StockProjectionItemResponse `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.Equal(t, "stock projections retrieved successfully", resBody.Message)
	assert.Len(t, resBody.Data, 1)
	assert.Equal(t, "RESTOCK", resBody.Data[0].Decision)
}

