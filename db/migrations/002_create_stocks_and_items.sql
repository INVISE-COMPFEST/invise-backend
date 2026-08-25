-- +goose Up
CREATE TABLE IF NOT EXISTS stocks (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    forecast_month TEXT NOT NULL,
    total_forecast NUMERIC(12,2) NOT NULL DEFAULT 0,
    mean_forecast  NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stocks_user_id ON stocks (user_id);

CREATE TABLE IF NOT EXISTS items (
    id                     TEXT PRIMARY KEY,
    stock_id               TEXT NOT NULL REFERENCES stocks(id) ON DELETE CASCADE,
    sku                    TEXT NOT NULL,
    name                   TEXT NOT NULL,
    store_id               TEXT NOT NULL DEFAULT '',
    quantity               INT NOT NULL DEFAULT 0,
    unit_cost              NUMERIC(12,2) NOT NULL DEFAULT 0,
    value_locked           NUMERIC(12,2) NOT NULL DEFAULT 0,
    days_in_stock          INT NOT NULL DEFAULT 0,
    last_sale_days         INT NOT NULL DEFAULT 0,
    current_price          NUMERIC(12,2) NOT NULL DEFAULT 0,
    deadstock_status       TEXT NOT NULL DEFAULT 'HEALTHY',
    opportunity_cost       NUMERIC(12,2) NOT NULL DEFAULT 0,
    market_average         NUMERIC(12,2) NOT NULL DEFAULT 0,
    predicted_sales        NUMERIC(12,2) NOT NULL DEFAULT 0,
    decision               TEXT NOT NULL DEFAULT 'HOLD',
    reasons_json           JSONB NOT NULL DEFAULT '[]',
    projection_points_json JSONB NOT NULL DEFAULT '[]',
    created_at             TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_items_stock_id ON items (stock_id);
CREATE INDEX idx_items_sku ON items (sku);

-- +goose Down
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS stocks;
