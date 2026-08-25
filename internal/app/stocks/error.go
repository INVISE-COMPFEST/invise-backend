package stocks

import pkgerr "invise-backend/pkg/errors"

var (
	ErrStockNotFound      = pkgerr.NotFound("STOCK_NOT_FOUND", "stock not found")
	ErrItemNotFound       = pkgerr.NotFound("ITEM_NOT_FOUND", "item not found")
	ErrInvalidCSVFormat   = pkgerr.BadRequest("INVALID_CSV_FORMAT", "invalid or missing required CSV headers")
	ErrEmptyCSV           = pkgerr.BadRequest("EMPTY_CSV", "uploaded CSV file is empty")
	ErrMissingImportFiles = pkgerr.BadRequest("MISSING_FILES", "monthly_sales_data, unit_cost_data, and stock_level_data files are required")
)
