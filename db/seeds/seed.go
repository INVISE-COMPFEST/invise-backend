package seeds

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User is a minimal representation of the users table for seeding.
type User struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Email        string    `gorm:"column:email;uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash"`
	Verified     bool      `gorm:"column:verified;default:false"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }

// Stock is a minimal representation of the stocks table for seeding.
type Stock struct {
	ID            string    `gorm:"column:id;primaryKey"`
	UserID        string    `gorm:"column:user_id;not null"`
	Name          string    `gorm:"column:name;not null"`
	ForecastMonth string    `gorm:"column:forecast_month;not null"`
	TotalForecast float64   `gorm:"column:total_forecast;default:0"`
	MeanForecast  float64   `gorm:"column:mean_forecast;default:0"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (Stock) TableName() string { return "stocks" }

// Item is a minimal representation of the items table for seeding.
type Item struct {
	ID                   string    `gorm:"column:id;primaryKey"`
	StockID              string    `gorm:"column:stock_id;not null"`
	SKU                  string    `gorm:"column:sku;not null"`
	Name                 string    `gorm:"column:name;not null"`
	StoreID              string    `gorm:"column:store_id"`
	Quantity             int       `gorm:"column:quantity;default:0"`
	UnitCost             float64   `gorm:"column:unit_cost;default:0"`
	ValueLocked          float64   `gorm:"column:value_locked;default:0"`
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

// Run seeds the database with a demo user, one stock, and a set of items.
// email and password are read from SeederConfig; if either is empty the seeder
// returns an error rather than inserting unusable data.
func Run(db *gorm.DB, email, password string) error {
	if email == "" || password == "" {
		return fmt.Errorf("SEEDER_EMAIL and SEEDER_PASSWORD must be set in the environment")
	}

	ctx := context.Background()

	// ── 1. Demo user ──────────────────────────────────────────────────────────
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}

	user := User{
		ID:           ulid.Make().String(),
		Email:        email,
		PasswordHash: string(hash),
		Verified:     true,
	}

	if err := db.WithContext(ctx).
		Where(User{Email: email}).
		FirstOrCreate(&user).Error; err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	log.Printf("  user:  %s (%s)", user.Email, user.ID)

	// ── 2. Demo stock ─────────────────────────────────────────────────────────
	stock := Stock{
		ID:            ulid.Make().String(),
		UserID:        user.ID,
		Name:          "Demo Inventory",
		ForecastMonth: time.Now().Format("2006-01"),
		TotalForecast: 15_000_000,
		MeanForecast:  1_500_000,
	}

	if err := db.WithContext(ctx).
		Where(Stock{UserID: user.ID, Name: stock.Name}).
		FirstOrCreate(&stock).Error; err != nil {
		return fmt.Errorf("upsert stock: %w", err)
	}
	log.Printf("  stock: %s (%s)", stock.Name, stock.ID)

	// ── 3. Demo items ─────────────────────────────────────────────────────────
	seedItems := []Item{
		{
			ID:              ulid.Make().String(),
			StockID:         stock.ID,
			SKU:             "SKU-001",
			Name:            "Wireless Headphones",
			StoreID:         "STORE-A",
			Quantity:        120,
			UnitCost:        350_000,
			ValueLocked:     42_000_000,
			CurrentPrice:    499_000,
			DeadstockStatus: "HEALTHY",
			Decision:        "HOLD",
			ReasonsJSON:     `[]`,
		},
		{
			ID:              ulid.Make().String(),
			StockID:         stock.ID,
			SKU:             "SKU-002",
			Name:            "USB-C Hub",
			StoreID:         "STORE-A",
			Quantity:        45,
			UnitCost:        180_000,
			ValueLocked:     8_100_000,
			CurrentPrice:    249_000,
			DeadstockStatus: "WARNING",
			Decision:        "DISCOUNT",
			ReasonsJSON:     `["Low turnover","High days in stock"]`,
		},
		{
			ID:              ulid.Make().String(),
			StockID:         stock.ID,
			SKU:             "SKU-003",
			Name:            "Mechanical Keyboard",
			StoreID:         "STORE-B",
			Quantity:        8,
			UnitCost:        620_000,
			ValueLocked:     4_960_000,
			CurrentPrice:    850_000,
			DeadstockStatus: "CRITICAL",
			Decision:        "LIQUIDATE",
			ReasonsJSON:     `["No sales in 90 days","Overstocked relative to demand"]`,
		},
		{
			ID:              ulid.Make().String(),
			StockID:         stock.ID,
			SKU:             "SKU-004",
			Name:            "Laptop Stand",
			StoreID:         "STORE-B",
			Quantity:        200,
			UnitCost:        95_000,
			ValueLocked:     19_000_000,
			CurrentPrice:    149_000,
			DeadstockStatus: "HEALTHY",
			Decision:        "HOLD",
			ReasonsJSON:     `[]`,
		},
		{
			ID:              ulid.Make().String(),
			StockID:         stock.ID,
			SKU:             "SKU-005",
			Name:            "Webcam 1080p",
			StoreID:         "STORE-A",
			Quantity:        60,
			UnitCost:        275_000,
			ValueLocked:     16_500_000,
			CurrentPrice:    379_000,
			DeadstockStatus: "WARNING",
			Decision:        "PROMOTE",
			ReasonsJSON:     `["Moderate turnover","Competitive market"]`,
		},
	}

	created := 0
	for i := range seedItems {
		res := db.WithContext(ctx).
			Where(Item{StockID: stock.ID, SKU: seedItems[i].SKU}).
			FirstOrCreate(&seedItems[i])
		if res.Error != nil {
			return fmt.Errorf("upsert item %s: %w", seedItems[i].SKU, res.Error)
		}
		if res.RowsAffected > 0 {
			created++
		}
		log.Printf("  item:  %-12s %s", seedItems[i].SKU, seedItems[i].Name)
	}
	log.Printf("  %d/%d items created (rest already existed)", created, len(seedItems))

	return nil
}
