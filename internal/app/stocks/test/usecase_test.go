package stocks_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"invise-backend/internal/app/stocks"
	"invise-backend/pkg/ai"
	pkgerr "invise-backend/pkg/errors"
)

type stubStockRepository struct {
	createStockWithItemsFn func(ctx context.Context, stock *stocks.Stock, items []stocks.Item) error
	listStocksByUserIDFn   func(ctx context.Context, userID string) ([]stocks.Stock, error)
	findStockByIDFn        func(ctx context.Context, userID, stockID string) (*stocks.Stock, error)
	findItemsByStockIDFn   func(ctx context.Context, userID, stockID string) ([]stocks.Item, error)
	findItemByIDFn         func(ctx context.Context, userID, itemID string) (*stocks.Item, error)
	findLatestStockFn      func(ctx context.Context, userID string) (*stocks.Stock, error)
}

func (s *stubStockRepository) CreateStockWithItems(ctx context.Context, stock *stocks.Stock, items []stocks.Item) error {
	if s.createStockWithItemsFn != nil {
		return s.createStockWithItemsFn(ctx, stock, items)
	}
	return nil
}

func (s *stubStockRepository) ListStocksByUserID(ctx context.Context, userID string) ([]stocks.Stock, error) {
	if s.listStocksByUserIDFn != nil {
		return s.listStocksByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (s *stubStockRepository) FindStockByID(ctx context.Context, userID, stockID string) (*stocks.Stock, error) {
	if s.findStockByIDFn != nil {
		return s.findStockByIDFn(ctx, userID, stockID)
	}
	return &stocks.Stock{ID: stockID, UserID: userID}, nil
}

func (s *stubStockRepository) FindItemsByStockID(ctx context.Context, userID, stockID string) ([]stocks.Item, error) {
	if s.findItemsByStockIDFn != nil {
		return s.findItemsByStockIDFn(ctx, userID, stockID)
	}
	return nil, nil
}

func (s *stubStockRepository) FindItemByID(ctx context.Context, userID, itemID string) (*stocks.Item, error) {
	if s.findItemByIDFn != nil {
		return s.findItemByIDFn(ctx, userID, itemID)
	}
	return nil, nil
}

func (s *stubStockRepository) FindLatestStockByUserID(ctx context.Context, userID string) (*stocks.Stock, error) {
	if s.findLatestStockFn != nil {
		return s.findLatestStockFn(ctx, userID)
	}
	return nil, nil
}

type stubAIClient struct {
	predictFn func(ctx context.Context, salesCSV io.Reader, filename string, includeSummary, includeFeatureImportance bool, topNFeatures int) (*ai.AIPredictResponse, error)
	healthFn  func(ctx context.Context) (*ai.AIHealthResponse, error)
	infoFn    func(ctx context.Context) (*ai.AIInfoResponse, error)
}

func (s *stubAIClient) Predict(ctx context.Context, salesCSV io.Reader, filename string, includeSummary, includeFeatureImportance bool, topNFeatures int) (*ai.AIPredictResponse, error) {
	if s.predictFn != nil {
		return s.predictFn(ctx, salesCSV, filename, includeSummary, includeFeatureImportance, topNFeatures)
	}
	return &ai.AIPredictResponse{
		Status:        "success",
		ForecastMonth: "2016-06",
		SeriesCount:   1,
		Summary: ai.AISummary{
			TotalForecast: 40.51,
			MeanForecast:  40.51,
		},
		Predictions: []ai.AIPredictionItem{
			{
				ItemID:                "FOODS_1_035",
				PredictedMonthlySales: 40.5085,
			},
		},
		FeatureImportance: []ai.AIFeatureImportance{
			{
				Rank:          1,
				Feature:       "monthly_sales",
				DisplayName:   "Riwayat Penjualan Bulanan",
				ImportancePct: 45.2,
			},
		},
	}, nil
}

func (s *stubAIClient) Health(ctx context.Context) (*ai.AIHealthResponse, error) {
	if s.healthFn != nil {
		return s.healthFn(ctx)
	}
	return &ai.AIHealthResponse{Status: "healthy", ModelLoaded: true}, nil
}

func (s *stubAIClient) Info(ctx context.Context) (*ai.AIInfoResponse, error) {
	if s.infoFn != nil {
		return s.infoFn(ctx)
	}
	return &ai.AIInfoResponse{Status: "success"}, nil
}

type stubULID struct {
	counter int
}

func (s *stubULID) Generate() (string, error) {
	s.counter++
	return "01ARZ3NDEKTSV4RRFFQ69G5FA" + string(rune('A'+s.counter)), nil
}

func sampleCSVs() (io.Reader, io.Reader, io.Reader) {
	sales := strings.NewReader("date_month,store_id,item_id,monthly_sales\n2016-04,CA_1,FOODS_1_035,10\n2016-05,CA_1,FOODS_1_035,15\n")
	cost := strings.NewReader("item_id,unit_cost\nFOODS_1_035,2.50\n")
	stock := strings.NewReader("item_id,quantity\nFOODS_1_035,30\n")
	return sales, cost, stock
}

func TestStockUsecase_Import(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		sales, cost, stock := sampleCSVs()
		var savedStock *stocks.Stock
		var savedItems []stocks.Item

		repo := &stubStockRepository{
			createStockWithItemsFn: func(ctx context.Context, stock *stocks.Stock, items []stocks.Item) error {
				savedStock = stock
				savedItems = items
				return nil
			},
		}
		aiClient := &stubAIClient{}
		ulidGen := &stubULID{}

		uc := stocks.NewStockUsecase(repo, aiClient, ulidGen)
		res, err := uc.Import(ctx, "user-123", sales, cost, stock, "sales.csv")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.NotEmpty(t, res.StockID)
		assert.Equal(t, 1, res.ItemCount)
		assert.Equal(t, "2016-06", res.ForecastMonth)

		require.NotNil(t, savedStock)
		assert.Equal(t, "user-123", savedStock.UserID)
		assert.Equal(t, 40.51, savedStock.TotalForecast)

		require.Len(t, savedItems, 1)
		item := savedItems[0]
		assert.Equal(t, "FOODS_1_035", item.SKU)
		assert.Equal(t, 30, item.Quantity)
		assert.Equal(t, 2.50, item.UnitCost)
		assert.Equal(t, 75.0, item.ValueLocked)
		assert.Equal(t, 40.5085, item.PredictedSales)
		assert.Equal(t, "RESTOCK", item.Decision)
		assert.Contains(t, item.ReasonsJSON, "Riwayat Penjualan Bulanan")
	})

	t.Run("Missing Files", func(t *testing.T) {
		repo := &stubStockRepository{}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})

		res, err := uc.Import(ctx, "user-123", nil, nil, nil, "")
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, stocks.ErrMissingImportFiles, err)
	})

	t.Run("Empty Sales CSV", func(t *testing.T) {
		sales := strings.NewReader("")
		cost := strings.NewReader("item_id,unit_cost\nFOODS_1_035,2.50\n")
		stock := strings.NewReader("item_id,quantity\nFOODS_1_035,30\n")

		repo := &stubStockRepository{}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})

		res, err := uc.Import(ctx, "user-123", sales, cost, stock, "sales.csv")
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, stocks.ErrEmptyCSV, err)
	})

	t.Run("Invalid CSV Header", func(t *testing.T) {
		sales := strings.NewReader("bad,header,only\n1,2,3\n")
		cost := strings.NewReader("item_id,unit_cost\nFOODS_1_035,2.50\n")
		stock := strings.NewReader("item_id,quantity\nFOODS_1_035,30\n")

		repo := &stubStockRepository{}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})

		res, err := uc.Import(ctx, "user-123", sales, cost, stock, "sales.csv")
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, stocks.ErrInvalidCSVFormat, err)
	})

	t.Run("AI Service Failure", func(t *testing.T) {
		sales, cost, stock := sampleCSVs()
		repo := &stubStockRepository{}
		aiClient := &stubAIClient{
			predictFn: func(ctx context.Context, salesCSV io.Reader, filename string, includeSummary, includeFeatureImportance bool, topNFeatures int) (*ai.AIPredictResponse, error) {
				return nil, pkgerr.BadGateway("AI_SERVICE_UNAVAILABLE", "ai container down")
			},
		}
		uc := stocks.NewStockUsecase(repo, aiClient, &stubULID{})

		res, err := uc.Import(ctx, "user-123", sales, cost, stock, "sales.csv")
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "ai container down")
	})

	t.Run("Database Save Error", func(t *testing.T) {
		sales, cost, stock := sampleCSVs()
		repo := &stubStockRepository{
			createStockWithItemsFn: func(ctx context.Context, stock *stocks.Stock, items []stocks.Item) error {
				return errors.New("db insert error")
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})

		res, err := uc.Import(ctx, "user-123", sales, cost, stock, "sales.csv")
		require.Error(t, err)
		assert.Nil(t, res)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "IMPORT_FAILED", appErr.Code)
	})
}

func TestStockUsecase_ListStocks(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		repo := &stubStockRepository{
			listStocksByUserIDFn: func(ctx context.Context, userID string) ([]stocks.Stock, error) {
				return []stocks.Stock{
					{ID: "stock-1"},
					{ID: "stock-2"},
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.ListStocks(ctx, "user-123")

		require.NoError(t, err)
		assert.Equal(t, []string{"stock-1", "stock-2"}, res)
	})

	t.Run("Database Error", func(t *testing.T) {
		repo := &stubStockRepository{
			listStocksByUserIDFn: func(ctx context.Context, userID string) ([]stocks.Stock, error) {
				return nil, errors.New("db query error")
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.ListStocks(ctx, "user-123")

		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestStockUsecase_GetStockItems(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		repo := &stubStockRepository{
			findItemsByStockIDFn: func(ctx context.Context, userID, stockID string) ([]stocks.Item, error) {
				return []stocks.Item{
					{
						ID:              "item-1",
						SKU:             "FOODS_1_035",
						Name:            "Item Foods #035",
						Quantity:        20,
						ValueLocked:     50.0,
						DeadstockStatus: "HEALTHY",
					},
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetStockItems(ctx, "user-123", "stock-1")

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "FOODS_1_035", res[0].SKU)
		assert.Equal(t, 20, res[0].Quantity)
	})

	t.Run("Stock Not Found", func(t *testing.T) {
		repo := &stubStockRepository{
			findItemsByStockIDFn: func(ctx context.Context, userID, stockID string) ([]stocks.Item, error) {
				return nil, stocks.ErrStockNotFound
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetStockItems(ctx, "user-123", "stock-999")

		require.Error(t, err)
		assert.Equal(t, stocks.ErrStockNotFound, err)
		assert.Nil(t, res)
	})
}

func TestStockUsecase_GetItemDetail(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		repo := &stubStockRepository{
			findItemByIDFn: func(ctx context.Context, userID, itemID string) (*stocks.Item, error) {
				return &stocks.Item{
					ID:           "item-1",
					StockID:      "stock-1",
					SKU:          "FOODS_1_035",
					Name:         "Item Foods #035",
					Quantity:     20,
					ValueLocked:  50.0,
					DaysInStock:  15,
					LastSaleDays: 5,
					CurrentPrice: 2.50,
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetItemDetail(ctx, "user-123", "item-1")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "FOODS_1_035", res.SKU)
		assert.Equal(t, 20, res.Quantity)
		assert.Equal(t, 2.50, res.CurrentPrice)
		assert.Equal(t, "stock-1", res.StocksID)
	})

	t.Run("Item Not Found", func(t *testing.T) {
		repo := &stubStockRepository{
			findItemByIDFn: func(ctx context.Context, userID, itemID string) (*stocks.Item, error) {
				return nil, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetItemDetail(ctx, "user-123", "nonexistent")

		require.Error(t, err)
		assert.Equal(t, stocks.ErrItemNotFound, err)
		assert.Nil(t, res)
	})
}

func TestStockUsecase_GetItemDiagnose(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		repo := &stubStockRepository{
			findItemByIDFn: func(ctx context.Context, userID, itemID string) (*stocks.Item, error) {
				return &stocks.Item{
					ID:              "item-1",
					DeadstockStatus: "DEADSTOCK",
					OpportunityCost: 15.50,
					MarketAverage:   2.50,
					ReasonsJSON:     `[{"name":"Riwayat Penjualan","percentage":45.2}]`,
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetItemDiagnose(ctx, "user-123", "item-1")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "DEADSTOCK", res.DeadstockStatus)
		assert.Equal(t, 15.50, res.OpportunityCost)
		require.Len(t, res.Reasons, 1)
		assert.Equal(t, "Riwayat Penjualan", res.Reasons[0].Name)
		assert.Equal(t, 45.2, res.Reasons[0].Percentage)
	})
}

func TestStockUsecase_GetStockProjection(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		repo := &stubStockRepository{
			findItemsByStockIDFn: func(ctx context.Context, userID, stockID string) ([]stocks.Item, error) {
				return []stocks.Item{
					{
						SKU:                  "FOODS_1_035",
						Name:                 "Item Foods #035",
						Quantity:             20,
						PredictedSales:       30,
						Decision:             "RESTOCK",
						ProjectionPointsJSON: `[[1,10],[2,20],[3,30]]`,
					},
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetStockProjection(ctx, "user-123", "stock-1", 1)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "FOODS_1_035", res[0].SKU)
		assert.Equal(t, 50.0, res[0].ProjectionPercentage)
		assert.Equal(t, "RESTOCK", res[0].Decision)
		assert.Len(t, res[0].ProjectionPoints, 3)
	})
}

func TestStockUsecase_GetMarketContext(t *testing.T) {
	ctx := context.Background()

	t.Run("Default Context when no stocks exist", func(t *testing.T) {
		repo := &stubStockRepository{
			findLatestStockFn: func(ctx context.Context, userID string) (*stocks.Stock, error) {
				return nil, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetMarketContext(ctx, "user-123")

		require.NoError(t, err)
		assert.NotEmpty(t, res.Context)
		assert.Equal(t, 0.85, res.Confidence)
	})

	t.Run("Context derived from healthy stock", func(t *testing.T) {
		repo := &stubStockRepository{
			findLatestStockFn: func(ctx context.Context, userID string) (*stocks.Stock, error) {
				return &stocks.Stock{
					ForecastMonth: "2016-06",
					Items: []stocks.Item{
						{DeadstockStatus: "HEALTHY"},
						{DeadstockStatus: "HEALTHY"},
					},
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetMarketContext(ctx, "user-123")

		require.NoError(t, err)
		assert.Contains(t, res.Context, "steady consumer traction")
	})

	t.Run("Context derived from high deadstock", func(t *testing.T) {
		repo := &stubStockRepository{
			findLatestStockFn: func(ctx context.Context, userID string) (*stocks.Stock, error) {
				return &stocks.Stock{
					ForecastMonth: "2016-06",
					Items: []stocks.Item{
						{DeadstockStatus: "DEADSTOCK"},
						{DeadstockStatus: "DEADSTOCK"},
					},
				}, nil
			},
		}
		uc := stocks.NewStockUsecase(repo, &stubAIClient{}, &stubULID{})
		res, err := uc.GetMarketContext(ctx, "user-123")

		require.NoError(t, err)
		assert.Contains(t, res.Context, "High inventory retention")
		assert.Equal(t, 0.92, res.Confidence)
	})
}
