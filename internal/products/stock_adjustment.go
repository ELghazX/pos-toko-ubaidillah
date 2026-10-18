package products

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type StockAdjustmentFilter struct {
	Query  string
	Reason string
	Period string
	From   string
	To     string
	Page   int
	Limit  int
}

type CreateStockAdjustmentReq struct {
	Mode          string
	ProductID     int32
	UnitID        int32
	PhysicalStock int32
	UsageQty      int32
	Reason        string
	Notes         string
}

func ReasonLabel(reason string) string {
	switch reason {
	case "selisih_opname":
		return "Selisih Stok Opname"
	case "koreksi_input":
		return "Koreksi Salah Input"
	case "pemakaian_toko":
		return "Pemakaian Operasional Toko"
	case "rusak":
		return "Barang Rusak / Cacat"
	case "kadaluarsa":
		return "Barang Kadaluarsa"
	default:
		return "Penyesuaian Lainnya"
	}
}

func (s *svc) generateNextAdjustmentNo(ctx context.Context, qtx *repo.Queries, prefix string) (string, error) {
	todayStr := time.Now().Format("20060102")
	pattern := fmt.Sprintf("%s-%s-%%", prefix, todayStr)

	lastNo, err := qtx.GetLastStockAdjustmentNo(ctx, pattern)
	if err != nil || lastNo == "" {
		return fmt.Sprintf("%s-%s-0001", prefix, todayStr), nil
	}

	parts := strings.Split(lastNo, "-")
	if len(parts) >= 3 {
		if lastNum, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return fmt.Sprintf("%s-%s-%04d", prefix, todayStr, lastNum+1), nil
		}
	}
	return fmt.Sprintf("%s-%s-0001", prefix, todayStr), nil
}

func (s *svc) ListStockAdjustments(ctx context.Context, filter StockAdjustmentFilter) ([]repo.ListStockAdjustmentsRow, int64, error) {
	searchTerm := ""
	if strings.TrimSpace(filter.Query) != "" {
		searchTerm = "%" + strings.TrimSpace(filter.Query) + "%"
	}

	startDate, endDate, hasDateFilter := utils.ParseDateFilter(filter.Period, filter.From, filter.To)
	var fromTS, toTS pgtype.Timestamptz
	if hasDateFilter && !startDate.IsZero() {
		fromTS = pgtype.Timestamptz{Time: startDate, Valid: true}
	}
	if hasDateFilter && !endDate.IsZero() {
		toTS = pgtype.Timestamptz{Time: endDate, Valid: true}
	}

	limit := int32(filter.Limit)
	if limit <= 0 {
		limit = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := int32((page - 1)) * limit

	rows, err := s.repo.ListStockAdjustments(ctx, repo.ListStockAdjustmentsParams{
		Column1: searchTerm,
		Column2: filter.Reason,
		Column3: fromTS,
		Column4: toTS,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountStockAdjustments(ctx, repo.CountStockAdjustmentsParams{
		Column1: searchTerm,
		Column2: filter.Reason,
		Column3: fromTS,
		Column4: toTS,
	})
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func (s *svc) CreateStockAdjustment(ctx context.Context, req CreateStockAdjustmentReq) (repo.StockAdjustment, error) {
	if req.ProductID <= 0 {
		return repo.StockAdjustment{}, errors.New("produk wajib dipilih")
	}

	_ = s.ensureProductInitialLedger(ctx, req.ProductID)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repo.StockAdjustment{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	product, err := qtx.GetProductByID(ctx, req.ProductID)
	if err != nil {
		return repo.StockAdjustment{}, errors.New("produk tidak ditemukan")
	}
	if product.Type != "barang" {
		return repo.StockAdjustment{}, errors.New("stok opname dan pemakaian hanya berlaku untuk produk fisik (barang)")
	}

	units, _ := qtx.GetProductUnits(ctx, req.ProductID)
	baseUnitName := "Pcs"
	selectedUnitName := "Pcs"
	conversionQty := int32(1)
	for _, u := range units {
		if u.ConversionQty == 1 {
			baseUnitName = u.UnitName
			if req.UnitID == 0 {
				selectedUnitName = u.UnitName
			}
		}
		if req.UnitID > 0 && u.ID == req.UnitID {
			selectedUnitName = u.UnitName
			conversionQty = u.ConversionQty
		}
	}
	if conversionQty < 1 {
		conversionQty = 1
	}

	systemStock := product.CurrentStock
	var physicalStock int32
	var diffQty int32
	var prefix string
	var ledgerTxType string
	var ledgerNotes string
	reason := strings.TrimSpace(req.Reason)
	userNotes := strings.TrimSpace(req.Notes)

	if req.Mode == "pemakaian" {
		if req.UsageQty <= 0 {
			return repo.StockAdjustment{}, errors.New("jumlah barang keluar / pemakaian harus lebih dari 0")
		}
		totalOutBase := req.UsageQty * conversionQty
		if systemStock < totalOutBase {
			return repo.StockAdjustment{}, fmt.Errorf("stok %s tidak mencukupi (stok sistem: %d %s, jumlah keluar: %d %s)", product.Name, systemStock, baseUnitName, totalOutBase, baseUnitName)
		}
		diffQty = -totalOutBase
		physicalStock = systemStock - totalOutBase
		if reason != "pemakaian_toko" && reason != "rusak" && reason != "kadaluarsa" && reason != "lainnya" {
			reason = "pemakaian_toko"
		}
		prefix = "PMK"
		ledgerTxType = "pemakaian"
	} else {
		if req.PhysicalStock < 0 {
			return repo.StockAdjustment{}, errors.New("stok fisik hasil hitung tidak boleh negatif")
		}
		physicalStock = req.PhysicalStock
		diffQty = physicalStock - systemStock
		if reason != "selisih_opname" && reason != "koreksi_input" && reason != "lainnya" {
			reason = "selisih_opname"
		}
		prefix = "OPN"
		ledgerTxType = "opname"
	}

	adjNo, err := s.generateNextAdjustmentNo(ctx, qtx, prefix)
	if err != nil {
		return repo.StockAdjustment{}, err
	}

	if req.Mode == "pemakaian" {
		unitDesc := ""
		if conversionQty > 1 {
			unitDesc = fmt.Sprintf(" [%d %s]", req.UsageQty, selectedUnitName)
		}
		ledgerNotes = fmt.Sprintf("%s (%s)%s", adjNo, ReasonLabel(reason), unitDesc)
	} else {
		ledgerNotes = fmt.Sprintf("%s (%s)", adjNo, ReasonLabel(reason))
	}
	if userNotes != "" {
		ledgerNotes += " - " + userNotes
	}

	adj, err := qtx.CreateStockAdjustment(ctx, repo.CreateStockAdjustmentParams{
		AdjustmentNo:  adjNo,
		ProductID:     product.ID,
		SystemStock:   systemStock,
		PhysicalStock: physicalStock,
		DifferenceQty: diffQty,
		Reason:        reason,
		Notes:         userNotes,
	})
	if err != nil {
		return repo.StockAdjustment{}, err
	}

	err = qtx.UpdateProductStockAndCost(ctx, repo.UpdateProductStockAndCostParams{
		ID:           product.ID,
		CurrentStock: physicalStock,
		AverageCost:  product.AverageCost,
	})
	if err != nil {
		return repo.StockAdjustment{}, err
	}

	err = qtx.CreateInventoryLedger(ctx, repo.CreateInventoryLedgerParams{
		ProductID:       product.ID,
		TransactionType: ledgerTxType,
		ReferenceID:     pgtype.Int4{Int32: adj.ID, Valid: true},
		QtyChange:       diffQty,
		BalanceAfter:    physicalStock,
		Notes:           ledgerNotes,
	})
	if err != nil {
		return repo.StockAdjustment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return repo.StockAdjustment{}, err
	}

	return adj, nil
}
