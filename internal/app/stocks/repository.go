package stocks

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Stock struct {
	ID            string    `gorm:"column:id;primaryKey"`
	UserID        string    `gorm:"column:user_id;not null"`
	Name          string    `gorm:"column:name;not null"`
	ForecastMonth string    `gorm:"column:forecast_month;not null"`
	TotalForecast float64   `gorm:"column:total_forecast;default:0"`
	MeanForecast  float64   `gorm:"column:mean_forecast;default:0"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
	Items         []Item    `gorm:"foreignKey:StockID;references:ID"`
}

func (Stock) TableName() string { return "stocks" }

type Item struct {
	ID                   string    `gorm:"column:id;primaryKey"`
	StockID              string    `gorm:"column:stock_id;not null"`
	SKU                  string    `gorm:"column:sku;not null"`
	Name                 string    `gorm:"column:name;not null"`
	StoreID              string    `gorm:"column:store_id"`
	Quantity             int       `gorm:"column:quantity;default:0"`
	UnitCost             float64   `gorm:"column:unit_cost;default:0"`
	ValueLocked          float64   `gorm:"column:value_locked;default:0"`
	DaysInStock          int       `gorm:"column:days_in_stock;default:0"`
	LastSaleDays         int       `gorm:"column:last_sale_days;default:0"`
	CurrentPrice         float64   `gorm:"column:current_price;default:0"`
	DeadstockStatus      string    `gorm:"column:deadstock_status;default:'HEALTHY'"`
	OpportunityCost      float64   `gorm:"column:opportunity_cost;default:0"`
	MarketAverage        float64   `gorm:"column:market_average;default:0"`
	PredictedSales       float64   `gorm:"column:predicted_sales;default:0"`
	Decision             string    `gorm:"column:decision;default:'HOLD'"`
	ReasonsJSON          string    `gorm:"column:reasons_json;default:'[]'"`
	ProjectionPointsJSON string    `gorm:"column:projection_points_json;default:'[]'"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (Item) TableName() string { return "items" }

type StockRepositoryI interface {
	CreateStockWithItems(ctx context.Context, stock *Stock, items []Item) error
	ListStocksByUserID(ctx context.Context, userID string) ([]Stock, error)
	FindStockByID(ctx context.Context, userID, stockID string) (*Stock, error)
	FindItemsByStockID(ctx context.Context, userID, stockID string) ([]Item, error)
	FindItemByID(ctx context.Context, userID, itemID string) (*Item, error)
}

type stockRepository struct {
	db *gorm.DB
}

func NewStockRepository(db *gorm.DB) StockRepositoryI {
	return &stockRepository{db: db}
}

func (r *stockRepository) CreateStockWithItems(ctx context.Context, stock *Stock, items []Item) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(stock).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *stockRepository) ListStocksByUserID(ctx context.Context, userID string) ([]Stock, error) {
	var stocks []Stock
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&stocks).Error
	return stocks, err
}

func (r *stockRepository) FindStockByID(ctx context.Context, userID, stockID string) (*Stock, error) {
	var stock Stock
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", stockID, userID).
		First(&stock).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &stock, nil
}

func (r *stockRepository) FindItemsByStockID(ctx context.Context, userID, stockID string) ([]Item, error) {
	// Ensure stock belongs to user
	stock, err := r.FindStockByID(ctx, userID, stockID)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, ErrStockNotFound
	}

	var items []Item
	err = r.db.WithContext(ctx).
		Where("stock_id = ?", stockID).
		Order("sku ASC").
		Find(&items).Error
	return items, err
}

func (r *stockRepository) FindItemByID(ctx context.Context, userID, itemID string) (*Item, error) {
	var item Item
	err := r.db.WithContext(ctx).
		Joins("JOIN stocks ON stocks.id = items.stock_id").
		Where("items.id = ? AND stocks.user_id = ?", itemID, userID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

