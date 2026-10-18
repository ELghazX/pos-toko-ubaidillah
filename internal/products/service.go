package products

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductSyncItem struct {
	ID   int32  `json:"id"`
	SKU  string `json:"sku"`
	Name string `json:"name"`
}

type SyncProgressCallback func(item ProductSyncItem, index, total int, newStock int32)

type Service interface {
	SearchProducts(ctx context.Context, query string, categoryID int32, productType string, stockFilter string, sortBy string, limit, offset int32) ([]repo.SearchProductsRow, int64, error)
	CountLowStock(ctx context.Context) (int64, error)
	CreateProduct(ctx context.Context, req CreateProductReq) error
	GenerateSKU(ctx context.Context, categoryID int32) (string, error)
	GetProductForEdit(ctx context.Context, id int32) (repo.Product, []repo.ProductUnit, error)
	UpdateProduct(ctx context.Context, req UpdateProductReq) error
	DeleteProduct(ctx context.Context, id int32) error
	RecalculateStock(ctx context.Context) error
	RecalculateStockStream(ctx context.Context, onProgress SyncProgressCallback) error
	GetProductStockCard(ctx context.Context, productID int32) (*ProductStockCard, error)
	ListStockAdjustments(ctx context.Context, filter StockAdjustmentFilter) ([]repo.ListStockAdjustmentsRow, int64, error)
	CreateStockAdjustment(ctx context.Context, req CreateStockAdjustmentReq) (repo.StockAdjustment, error)
}

type svc struct {
	repo repo.Querier
	db   *pgxpool.Pool
}

func NewService(repo repo.Querier, db *pgxpool.Pool) Service {
	return &svc{
		repo: repo,
		db:   db,
	}
}

type UnitReq struct {
	UnitName      string
	ConversionQty int32
	Price         pgtype.Numeric
}

type CreateProductReq struct {
	CategoryID        int32
	SKU               string
	Name              string
	ProductType       string
	IsOpenPrice       bool
	CurrentStock      int32
	InitialStockNotes string
	MinStock          int32
	AverageCost       pgtype.Numeric
	Units             []UnitReq
}

func (s *svc) SearchProducts(ctx context.Context, query string, categoryID int32, productType string, stockFilter string, sortBy string, limit, offset int32) ([]repo.SearchProductsRow, int64, error) {
	searchTerm := "%" + query + "%"

	products, err := s.repo.SearchProducts(ctx, repo.SearchProductsParams{
		Name:    searchTerm,
		Column2: categoryID,
		Column3: productType,
		Column4: stockFilter,
		Column5: sortBy,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountProducts(ctx, repo.CountProductsParams{
		Name:    searchTerm,
		Column2: categoryID,
		Column3: productType,
		Column4: stockFilter,
	})
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (s *svc) CountLowStock(ctx context.Context) (int64, error) {
	return s.repo.CountLowStockProducts(ctx)
}

func (s *svc) CreateProduct(ctx context.Context, req CreateProductReq) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	avgCost := req.AverageCost
	if !avgCost.Valid {
		avgCost = utils.FloatToNumeric(0)
	}

	initialStock := req.CurrentStock
	if req.ProductType != "barang" || initialStock < 0 {
		initialStock = 0
	}

	newProductID, err := qtx.CreateProduct(ctx, repo.CreateProductParams{
		CategoryID:   req.CategoryID,
		Sku:          req.SKU,
		Name:         utils.ToPascalCase(req.Name),
		Type:         req.ProductType,
		IsOpenPrice:  req.IsOpenPrice,
		CurrentStock: initialStock,
		AverageCost:  avgCost,
		MinStock:     req.MinStock,
	})

	if err != nil {
		return err
	}

	for _, u := range req.Units {
		err = qtx.CreateProductUnit(ctx, repo.CreateProductUnitParams{
			ProductID:     newProductID,
			UnitName:      utils.ToPascalCase(u.UnitName),
			ConversionQty: u.ConversionQty,
			Price:         u.Price,
		})
		if err != nil {
			return err
		}
	}

	if req.ProductType == "barang" && initialStock > 0 {
		note := strings.TrimSpace(req.InitialStockNotes)
		if note == "" {
			note = "Stok awal produk baru"
		}
		err = qtx.CreateInventoryLedger(ctx, repo.CreateInventoryLedgerParams{
			ProductID:       newProductID,
			TransactionType: "opname",
			ReferenceID:     pgtype.Int4{Int32: newProductID, Valid: true},
			QtyChange:       initialStock,
			BalanceAfter:    initialStock,
			Notes:           note,
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *svc) GenerateSKU(ctx context.Context, categoryID int32) (string, error) {
	category, err := s.repo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return "", err
	}
	searchPattern := category.Code + "-%"
	lastSKU, err := s.repo.GetLastProductByCategory(ctx, repo.GetLastProductByCategoryParams{
		CategoryID: categoryID,
		Sku:        searchPattern,
	})

	if err != nil || lastSKU == "" {
		return fmt.Sprintf("%s-0001", category.Code), nil
	}

	parts := strings.Split(lastSKU, "-")
	if len(parts) == 2 {
		lastNumber, err := strconv.Atoi(parts[1])
		if err == nil {
			newNumber := lastNumber + 1
			return fmt.Sprintf("%s-%04d", category.Code, newNumber), nil
		}
	}
	return fmt.Sprintf("%s-0001", category.Code), nil
}

type UpdateProductReq struct {
	ID          int32
	CategoryID  int32
	SKU         string
	Name        string
	ProductType string
	IsOpenPrice bool
	MinStock    int32
	AverageCost pgtype.Numeric
	Units       []UnitReq
}

func (s *svc) GetProductForEdit(ctx context.Context, id int32) (repo.Product, []repo.ProductUnit, error) {
	product, err := s.repo.GetProductByID(ctx, id)
	if err != nil {
		return repo.Product{}, nil, err
	}
	units, err := s.repo.GetProductUnits(ctx, id)
	return product, units, err
}

func (s *svc) UpdateProduct(ctx context.Context, req UpdateProductReq) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	avgCost := req.AverageCost
	if !avgCost.Valid {
		avgCost = utils.FloatToNumeric(0)
	}

	err = qtx.UpdateProduct(ctx, repo.UpdateProductParams{
		ID:          req.ID,
		CategoryID:  req.CategoryID,
		Sku:         req.SKU,
		Name:        utils.ToPascalCase(req.Name),
		Type:        req.ProductType,
		IsOpenPrice: req.IsOpenPrice,
		MinStock:    req.MinStock,
		AverageCost: avgCost,
	})
	if err != nil {
		return err
	}

	if utils.NumericToFloat(avgCost) > 0 {
		_ = qtx.BackfillZeroCogsByProductID(ctx, repo.BackfillZeroCogsByProductIDParams{
			ProductID: req.ID,
			Column2:   avgCost,
		})
	}

	err = qtx.DeactivateProductUnitsByProductID(ctx, req.ID)
	if err != nil {
		return err
	}

	for _, u := range req.Units {
		formattedName := utils.ToPascalCase(u.UnitName)
		_, err = qtx.UpdateExistingProductUnit(ctx, repo.UpdateExistingProductUnitParams{
			ProductID:     req.ID,
			UnitName:      formattedName,
			ConversionQty: u.ConversionQty,
			Price:         u.Price,
		})
		if err != nil {
			err = qtx.CreateProductUnit(ctx, repo.CreateProductUnitParams{
				ProductID:     req.ID,
				UnitName:      formattedName,
				ConversionQty: u.ConversionQty,
				Price:         u.Price,
			})
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (s *svc) DeleteProduct(ctx context.Context, id int32) error {
	return s.repo.DeleteProduct(ctx, id)
}

func (s *svc) ensureProductInitialLedger(ctx context.Context, productID int32) error {
	_, _ = s.db.Exec(ctx, `
		UPDATE inventory_ledgers
		SET transaction_type = 'opname'
		WHERE product_id = $1
		  AND transaction_type = 'pembelian'
		  AND notes LIKE 'Stok awal produk baru%'
	`, productID)

	_, err := s.db.Exec(ctx, `
		INSERT INTO inventory_ledgers (product_id, date, transaction_type, reference_id, qty_change, balance_after, notes)
		SELECT p.id, COALESCE(p.created_at, CURRENT_TIMESTAMP), 'opname', p.id, p.current_stock, p.current_stock, 'Stok awal produk baru'
		FROM products p
		WHERE p.id = $1
		  AND p.type = 'barang'
		  AND p.current_stock > 0
		  AND NOT EXISTS (SELECT 1 FROM inventory_ledgers l WHERE l.product_id = p.id)
	`, productID)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `
		WITH earliest AS (
			SELECT product_id, date, qty_change, balance_after
			FROM inventory_ledgers
			WHERE product_id = $1
			ORDER BY date ASC, id ASC
			LIMIT 1
		)
		INSERT INTO inventory_ledgers (product_id, date, transaction_type, reference_id, qty_change, balance_after, notes)
		SELECT e.product_id, e.date - INTERVAL '1 second', 'opname', e.product_id, (e.balance_after - e.qty_change), (e.balance_after - e.qty_change), 'Stok awal produk baru'
		FROM earliest e
		JOIN products p ON p.id = e.product_id
		WHERE p.type = 'barang'
		  AND (e.balance_after - e.qty_change) > 0
	`, productID)
	return err
}

func (s *svc) RecalculateStock(ctx context.Context) error {
	return s.RecalculateStockStream(ctx, nil)
}

func (s *svc) RecalculateStockStream(ctx context.Context, onProgress SyncProgressCallback) error {
	rows, err := s.db.Query(ctx, "SELECT id, sku, name FROM products WHERE type = 'barang' ORDER BY id ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []ProductSyncItem
	for rows.Next() {
		var it ProductSyncItem
		if err := rows.Scan(&it.ID, &it.SKU, &it.Name); err != nil {
			return err
		}
		items = append(items, it)
	}
	rows.Close()

	total := len(items)
	for i, it := range items {
		_ = s.ensureProductInitialLedger(ctx, it.ID)

		var newStock int32
		err := s.db.QueryRow(ctx, `
			UPDATE products
			SET current_stock = COALESCE((
				SELECT SUM(qty_change)
				FROM inventory_ledgers
				WHERE product_id = $1
			), 0)
			WHERE id = $1
			RETURNING current_stock
		`, it.ID).Scan(&newStock)
		if err != nil {
			return err
		}

		if onProgress != nil {
			onProgress(it, i+1, total, newStock)
		}
	}

	return nil
}

type StockLedgerEntry struct {
	ID              int32  `json:"id"`
	DateFormatted   string `json:"date_formatted"`
	TransactionType string `json:"transaction_type"`
	ReferenceID     int32  `json:"reference_id"`
	ReferenceURL    string `json:"reference_url"`
	QtyChange       int32  `json:"qty_change"`
	BalanceAfter    int32  `json:"balance_after"`
	Notes           string `json:"notes"`
}

type ProductStockCard struct {
	ProductID    int32              `json:"product_id"`
	SKU          string             `json:"sku"`
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	CurrentStock int32              `json:"current_stock"`
	MinStock     int32              `json:"min_stock"`
	BaseUnit     string             `json:"base_unit"`
	AverageCost  string             `json:"average_cost"`
	TotalIn      int32              `json:"total_in"`
	TotalOut     int32              `json:"total_out"`
	Entries      []StockLedgerEntry `json:"entries"`
}

func (s *svc) GetProductStockCard(ctx context.Context, productID int32) (*ProductStockCard, error) {
	product, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	_ = s.ensureProductInitialLedger(ctx, productID)

	units, _ := s.repo.GetProductUnits(ctx, productID)
	baseUnit := "Pcs"
	for _, u := range units {
		if u.ConversionQty == 1 {
			baseUnit = u.UnitName
			break
		}
	}

	ledgers, err := s.repo.ListProductStockLedger(ctx, productID)
	if err != nil {
		return nil, err
	}

	entries := make([]StockLedgerEntry, 0, len(ledgers))
	var totalIn, totalOut int32
	for _, l := range ledgers {
		if l.QtyChange > 0 {
			totalIn += l.QtyChange
		} else if l.QtyChange < 0 {
			totalOut += -l.QtyChange
		}

		refURL := ""
		if l.ReferenceID.Valid && l.ReferenceID.Int32 > 0 {
			switch l.TransactionType {
			case "penjualan":
				refURL = fmt.Sprintf("/sales/%d", l.ReferenceID.Int32)
			case "pembelian":
				if !strings.HasPrefix(l.Notes, "Stok awal") {
					refURL = fmt.Sprintf("/purchases/%d", l.ReferenceID.Int32)
				}
			case "opname", "pemakaian":
				if !strings.HasPrefix(l.Notes, "Stok awal") && !strings.HasPrefix(l.Notes, "Saldo awal") {
					refURL = "/stock-adjustments"
				}
			case "retur_batal":
				if strings.HasPrefix(l.Notes, "Batal ") {
					refURL = fmt.Sprintf("/sales/%d", l.ReferenceID.Int32)
				} else if strings.HasPrefix(l.Notes, "Pembatalan penerimaan") {
					refURL = fmt.Sprintf("/purchases/%d", l.ReferenceID.Int32)
				}
			}
		}

		dateStr := "-"
		if l.Date.Valid {
			dateStr = l.Date.Time.Local().Format("02/01/2006 15:04")
		}

		entries = append(entries, StockLedgerEntry{
			ID:              l.ID,
			DateFormatted:   dateStr,
			TransactionType: l.TransactionType,
			ReferenceID:     l.ReferenceID.Int32,
			ReferenceURL:    refURL,
			QtyChange:       l.QtyChange,
			BalanceAfter:    l.BalanceAfter,
			Notes:           l.Notes,
		})
	}

	return &ProductStockCard{
		ProductID:    product.ID,
		SKU:          product.Sku,
		Name:         product.Name,
		Type:         product.Type,
		CurrentStock: product.CurrentStock,
		MinStock:     product.MinStock,
		BaseUnit:     baseUnit,
		AverageCost:  utils.FormatRupiah(product.AverageCost),
		TotalIn:      totalIn,
		TotalOut:     totalOut,
		Entries:      entries,
	}, nil
}
