-- +goose Up
ALTER TABLE items
    DROP COLUMN IF EXISTS days_in_stock,
    DROP COLUMN IF EXISTS last_sale_days;

-- +goose Down
ALTER TABLE items
    ADD COLUMN days_in_stock  INT NOT NULL DEFAULT 0,
    ADD COLUMN last_sale_days INT NOT NULL DEFAULT 0;
