package stocks

import (
	"mime/multipart"
	"strconv"

	"github.com/gofiber/fiber/v3"

	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/response"
)

type StockHandler struct {
	usecase StockUsecaseI
}

func NewStockHandler(usecase StockUsecaseI) *StockHandler {
	return &StockHandler{
		usecase: usecase,
	}
}

func (h *StockHandler) getUserID(c fiber.Ctx) (string, error) {
	val := c.Locals("user_id")
	if val == nil {
		return "", pkgerr.Unauthorized("UNAUTHORIZED", "unauthorized")
	}
	userID, ok := val.(string)
	if !ok || userID == "" {
		return "", pkgerr.Unauthorized("UNAUTHORIZED", "invalid user context")
	}
	return userID, nil
}

func getFormFile(c fiber.Ctx, fieldNames ...string) (*multipart.FileHeader, error) {
	for _, name := range fieldNames {
		if header, err := c.FormFile(name); err == nil && header != nil {
			return header, nil
		}
	}
	return nil, ErrMissingImportFiles
}

func (h *StockHandler) Import(c fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	salesHeader, err := getFormFile(c, "monthly_sales_data", "sales_data", "sales", "file", "monthly_sales")
	if err != nil {
		return ErrMissingImportFiles
	}
	costHeader, err := getFormFile(c, "unit_cost_data", "cost_data", "item_modal", "modal", "unit_cost", "cost")
	if err != nil {
		return ErrMissingImportFiles
	}
	stockLevelHeader, err := getFormFile(c, "stock_level_data", "inventory_data", "item_inventory", "inventory", "stock_level", "stock")
	if err != nil {
		return ErrMissingImportFiles
	}

	salesFile, err := salesHeader.Open()
	if err != nil {
		return pkgerr.BadRequest("FILE_OPEN_ERROR", "failed to open monthly_sales_data file")
	}
	defer salesFile.Close()

	costFile, err := costHeader.Open()
	if err != nil {
		return pkgerr.BadRequest("FILE_OPEN_ERROR", "failed to open unit_cost_data file")
	}
	defer costFile.Close()

	stockLevelFile, err := stockLevelHeader.Open()
	if err != nil {
		return pkgerr.BadRequest("FILE_OPEN_ERROR", "failed to open stock_level_data file")
	}
	defer stockLevelFile.Close()

	res, err := h.usecase.Import(c.Context(), userID, salesFile, costFile, stockLevelFile, salesHeader.Filename)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(dto.Response[*ImportResponse]{
		Message: "stock data imported and analyzed successfully",
		Data:    res,
	})
}

func (h *StockHandler) ListStocks(c fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	stocks, err := h.usecase.ListStocks(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(dto.Response[[]string]{
		Message: "stocks retrieved successfully",
		Data:    stocks,
	})
}

func (h *StockHandler) GetStockItems(c fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	stockID := c.Params("stock_id")
	if stockID == "" {
		return pkgerr.BadRequest("MISSING_STOCK_ID", "stock_id path parameter is required")
	}

	items, err := h.usecase.GetStockItems(c.Context(), userID, stockID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(dto.Response[[]StockListItemResponse]{
		Message: "stock items retrieved successfully",
		Data:    items,
	})
}

func (h *StockHandler) GetItemDetail(c fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	itemID := c.Params("items_id")
	if itemID == "" {
		itemID = c.Params("item_id")
	}
	if itemID == "" {
		return pkgerr.BadRequest("MISSING_ITEM_ID", "items_id path parameter is required")
	}

	item, err := h.usecase.GetItemDetail(c.Context(), userID, itemID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(dto.Response[*ItemDetailResponse]{
		Message: "item details retrieved successfully",
		Data:    item,
	})
}

func (h *StockHandler) GetItemDiagnose(c fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	itemID := c.Params("items_id")
	if itemID == "" {
		itemID = c.Params("item_id")
	}
	if itemID == "" {
		return pkgerr.BadRequest("MISSING_ITEM_ID", "items_id path parameter is required")
	}

	diagnose, err := h.usecase.GetItemDiagnose(c.Context(), userID, itemID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(dto.Response[*ItemDiagnoseResponse]{
		Message: "item diagnose retrieved successfully",
		Data:    diagnose,
	})
}

func (h *StockHandler) GetStockProjection(c fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	stockID := c.Params("stock_id")
	if stockID == "" {
		stockID = c.Params("stocks_id")
	}
	if stockID == "" {
		return pkgerr.BadRequest("MISSING_STOCK_ID", "stock_id path parameter is required")
	}

	rangeVal, _ := strconv.Atoi(c.Query("range", "1"))

	projections, err := h.usecase.GetStockProjection(c.Context(), userID, stockID, rangeVal)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(dto.Response[[]StockProjectionItemResponse]{
		Message: "stock projections retrieved successfully",
		Data:    projections,
	})
}


