package purchases

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/whatsapp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PurchaseItemReq struct {
	ProductID     int32
	UnitID        int32
	UnitName      string
	ConversionQty int32
	Qty           int32
	Cost          float64
}

type CreatePurchaseReq struct {
	InvoiceNumber string
	SupplierID    int32
	PaymentMethod string
	Status        string // "pending" (PO) atau "diterima" (Langsung Terima)
	Notes         string
	Items         []PurchaseItemReq
}

type ReceiveItemInput struct {
	DetailID        int32
	ReceivedQty     int32
	ActualCost      float64
	CreateBackorder bool
}

type ReceivePurchaseReq struct {
	PurchaseID    int32
	PaymentMethod string
	Notes         string
	Items         []ReceiveItemInput
}

type Service interface {
	ListPurchases(ctx context.Context) ([]repo.ListPurchasesRow, error)
	GetPurchaseDetail(ctx context.Context, id int32) (repo.GetPurchaseByIDRow, []repo.GetPurchaseDetailsRow, error)
	GetFormMasterData(ctx context.Context) ([]repo.Supplier, []repo.Product, []repo.ListAllActiveProductUnitsWithLastCostRow, string, error)
	GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error)
	CreatePurchase(ctx context.Context, req CreatePurchaseReq) error
	ReceivePurchaseOrder(ctx context.Context, req ReceivePurchaseReq) error
	CancelPurchaseOrder(ctx context.Context, id int32) error
	GenerateNextInvoiceNumber(ctx context.Context, prefix string) (string, error)
	SendWhatsAppPO(ctx context.Context, poID int32, targetPhone string) error
	ListSuppliers(ctx context.Context) ([]repo.Supplier, error)
	VoidGoodsReceipt(ctx context.Context, id int32) error
	ReopenPurchaseOrder(ctx context.Context, id int32) error
}

type svc struct {
	repo repo.Querier
	db   *pgxpool.Pool
	wa   *whatsapp.Service
}

func NewService(repo repo.Querier, db *pgxpool.Pool, wa *whatsapp.Service) Service {
	return &svc{
		repo: repo,
		db:   db,
		wa:   wa,
	}
}

func floatToNumeric(val float64) pgtype.Numeric {
	return utils.FloatToNumeric(val)
}

func numericToFloat(n pgtype.Numeric) float64 {
	return utils.NumericToFloat(n)
}

func (s *svc) ListPurchases(ctx context.Context) ([]repo.ListPurchasesRow, error) {
	return s.repo.ListPurchases(ctx)
}

func (s *svc) GetPurchaseDetail(ctx context.Context, id int32) (repo.GetPurchaseByIDRow, []repo.GetPurchaseDetailsRow, error) {
	p, err := s.repo.GetPurchaseByID(ctx, id)
	if err != nil {
		return repo.GetPurchaseByIDRow{}, nil, err
	}
	items, err := s.repo.GetPurchaseDetails(ctx, id)
	if err != nil {
		return repo.GetPurchaseByIDRow{}, nil, err
	}
	return p, items, nil
}

func (s *svc) GenerateNextInvoiceNumber(ctx context.Context, prefix string) (string, error) {
	todayStr := time.Now().Format("20060102")
	pattern := fmt.Sprintf("%s-%s-%%", prefix, todayStr)

	var lastInvoice string
	err := s.db.QueryRow(ctx, `
		SELECT invoice_number 
		FROM purchases 
		WHERE invoice_number LIKE $1 
		  AND invoice_number NOT LIKE '%-BO%'
		ORDER BY invoice_number DESC, id DESC 
		LIMIT 1
	`, pattern).Scan(&lastInvoice)

	if err != nil || lastInvoice == "" {
		return fmt.Sprintf("%s-%s-0001", prefix, todayStr), nil
	}

	parts := strings.Split(lastInvoice, "-")
	if len(parts) >= 3 {
		lastNum, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			return fmt.Sprintf("%s-%s-%04d", prefix, todayStr, lastNum+1), nil
		}
	}

	return fmt.Sprintf("%s-%s-0001", prefix, todayStr), nil
}

func (s *svc) GetFormMasterData(ctx context.Context) ([]repo.Supplier, []repo.Product, []repo.ListAllActiveProductUnitsWithLastCostRow, string, error) {
	suppliers, err := s.repo.ListSuppliers(ctx)
	if err != nil {
		return nil, nil, nil, "", err
	}
	products, err := s.repo.ListActiveGoodsProducts(ctx)
	if err != nil {
		return nil, nil, nil, "", err
	}
	units, err := s.repo.ListAllActiveProductUnitsWithLastCost(ctx)
	if err != nil {
		return nil, nil, nil, "", err
	}

	defaultInvoice, err := s.GenerateNextInvoiceNumber(ctx, "PO")
	if err != nil {
		defaultInvoice = fmt.Sprintf("PO-%s-0001", time.Now().Format("20060102"))
	}
	return suppliers, products, units, defaultInvoice, nil
}

func applyStockAndMovingAverage(
	ctx context.Context,
	qtx *repo.Queries,
	purchaseID int32,
	invoiceNo string,
	productID int32,
	unitName string,
	conversionQty int32,
	qty int32,
	subtotal float64,
) error {
	prod, err := qtx.GetProductByID(ctx, productID)
	if err != nil {
		return err
	}

	oldStock := prod.CurrentStock
	effectiveOldStock := float64(oldStock)
	if effectiveOldStock < 0 {
		effectiveOldStock = 0
	}

	oldCost := numericToFloat(prod.AverageCost)
	addedBaseQty := qty * conversionQty
	newStock := oldStock + addedBaseQty

	var newAvgCost float64
	if oldCost <= 0 && addedBaseQty > 0 {
		newAvgCost = subtotal / float64(addedBaseQty)
	} else {
		totalBaseQtyForAvg := effectiveOldStock + float64(addedBaseQty)
		newAvgCost = oldCost
		if totalBaseQtyForAvg > 0 {
			newAvgCost = ((effectiveOldStock * oldCost) + subtotal) / totalBaseQtyForAvg
		}
	}

	err = qtx.UpdateProductStockAndCost(ctx, repo.UpdateProductStockAndCostParams{
		ID:           productID,
		CurrentStock: newStock,
		AverageCost:  floatToNumeric(newAvgCost),
	})
	if err != nil {
		return err
	}

	if oldCost <= 0 && newAvgCost > 0 {
		_ = qtx.BackfillZeroCogsByProductID(ctx, repo.BackfillZeroCogsByProductIDParams{
			ProductID: productID,
			Column2:   floatToNumeric(newAvgCost),
		})
	}

	return qtx.CreateInventoryLedger(ctx, repo.CreateInventoryLedgerParams{
		ProductID:       productID,
		TransactionType: "pembelian",
		ReferenceID:     pgtype.Int4{Int32: purchaseID, Valid: true},
		QtyChange:       addedBaseQty,
		BalanceAfter:    newStock,
		Notes:           fmt.Sprintf("Penerimaan %s (%d %s)", invoiceNo, qty, unitName),
	})
}

func (s *svc) CreatePurchase(ctx context.Context, req CreatePurchaseReq) error {
	if len(req.Items) == 0 {
		return errors.New("minimal harus ada 1 barang dalam daftar")
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "diterima" {
		status = "pending"
	}

	if status == "pending" && req.SupplierID <= 0 {
		return errors.New("Purchase Order wajib memilih supplier dari daftar master data")
	}

	invoiceNo := strings.ToUpper(strings.TrimSpace(req.InvoiceNumber))
	if invoiceNo == "" {
		if status == "diterima" {
			invoiceNo, _ = s.GenerateNextInvoiceNumber(ctx, "TB")
		} else {
			invoiceNo, _ = s.GenerateNextInvoiceNumber(ctx, "PO")
		}
	}

	var totalCost float64
	for _, item := range req.Items {
		totalCost += float64(item.Qty) * item.Cost
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	var supID pgtype.Int4
	if req.SupplierID > 0 {
		supID = pgtype.Int4{Int32: req.SupplierID, Valid: true}
	}

	paymentMethod := strings.ToLower(strings.TrimSpace(req.PaymentMethod))
	if paymentMethod == "" {
		paymentMethod = "tunai"
	}

	var receivedAt pgtype.Timestamptz
	if status == "diterima" {
		receivedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	purchaseID, err := qtx.CreatePurchase(ctx, repo.CreatePurchaseParams{
		InvoiceNumber: invoiceNo,
		SupplierID:    supID,
		PaymentMethod: paymentMethod,
		TotalCost:     floatToNumeric(totalCost),
		Status:        status,
		Notes:         strings.TrimSpace(req.Notes),
		ReceivedAt:    receivedAt,
	})
	if err != nil {
		return err
	}

	for _, item := range req.Items {
		if item.ProductID <= 0 || item.Qty <= 0 {
			continue
		}
		if item.ConversionQty < 1 {
			item.ConversionQty = 1
		}

		subtotal := float64(item.Qty) * item.Cost
		unitName := utils.ToPascalCase(item.UnitName)

		var unitID pgtype.Int4
		if item.UnitID > 0 {
			unitID = pgtype.Int4{Int32: item.UnitID, Valid: true}
		}

		err = qtx.CreatePurchaseDetail(ctx, repo.CreatePurchaseDetailParams{
			PurchaseID:    purchaseID,
			ProductID:     item.ProductID,
			UnitID:        unitID,
			UnitName:      unitName,
			ConversionQty: item.ConversionQty,
			Qty:           item.Qty,
			Cost:          floatToNumeric(item.Cost),
			Subtotal:      floatToNumeric(subtotal),
		})
		if err != nil {
			return err
		}

		if status == "diterima" {
			err = applyStockAndMovingAverage(
				ctx, qtx, purchaseID, invoiceNo,
				item.ProductID, unitName, item.ConversionQty, item.Qty, subtotal,
			)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (s *svc) ReceivePurchaseOrder(ctx context.Context, req ReceivePurchaseReq) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	po, err := qtx.GetPurchaseByID(ctx, req.PurchaseID)
	if err != nil {
		return errors.New("PO tidak ditemukan")
	}
	if po.Status != "pending" {
		return errors.New("PO ini sudah pernah diterima atau dibatalkan")
	}

	details, err := qtx.GetPurchaseDetails(ctx, req.PurchaseID)
	if err != nil {
		return err
	}

	inputMap := make(map[int32]ReceiveItemInput)
	for _, in := range req.Items {
		inputMap[in.DetailID] = in
	}

	type backorderItem struct {
		detail       repo.GetPurchaseDetailsRow
		remainingQty int32
		cost         float64
	}

	var backorders []backorderItem
	var receivedTotalCost float64
	var totalReceivedItems int

	for _, d := range details {
		in, exists := inputMap[d.ID]
		recQty := d.Qty
		actCost := numericToFloat(d.Cost)
		shouldBackorder := false
		if exists {
			recQty = in.ReceivedQty
			actCost = in.ActualCost
			shouldBackorder = in.CreateBackorder
		}
		if recQty < 0 {
			recQty = 0
		}

		if recQty < d.Qty && shouldBackorder {
			backorders = append(backorders, backorderItem{
				detail:       d,
				remainingQty: d.Qty - recQty,
				cost:         actCost,
			})
		}

		if recQty == 0 {
			if err := qtx.DeletePurchaseDetailByID(ctx, d.ID); err != nil {
				return err
			}
			continue
		}

		totalReceivedItems++
		subtotal := float64(recQty) * actCost
		receivedTotalCost += subtotal

		err = qtx.UpdatePurchaseDetailOnReceive(ctx, repo.UpdatePurchaseDetailOnReceiveParams{
			ID:       d.ID,
			Qty:      recQty,
			Cost:     floatToNumeric(actCost),
			Subtotal: floatToNumeric(subtotal),
		})
		if err != nil {
			return err
		}

		err = applyStockAndMovingAverage(
			ctx, qtx, po.ID, po.InvoiceNumber,
			d.ProductID, d.UnitName, d.ConversionQty, recQty, subtotal,
		)
		if err != nil {
			return err
		}
	}

	if totalReceivedItems == 0 {
		return errors.New("minimal harus ada 1 barang dengan jumlah diterima lebih dari 0")
	}

	paymentMethod := strings.ToLower(strings.TrimSpace(req.PaymentMethod))
	if paymentMethod == "" {
		paymentMethod = po.PaymentMethod
	}

	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = po.Notes
	}

	err = qtx.UpdatePurchaseOnReceive(ctx, repo.UpdatePurchaseOnReceiveParams{
		ID:            po.ID,
		PaymentMethod: paymentMethod,
		TotalCost:     floatToNumeric(receivedTotalCost),
		Notes:         notes,
	})
	if err != nil {
		return err
	}

	if len(backorders) > 0 {
		var boTotal float64
		for _, bo := range backorders {
			boTotal += float64(bo.remainingQty) * bo.cost
		}

		boInvoice := fmt.Sprintf("%s-BO%02d", po.InvoiceNumber, time.Now().Unix()%100)
		boNotes := fmt.Sprintf("Backorder otomatis (sisa kiriman) dari %s", po.InvoiceNumber)

		newPOID, err := qtx.CreatePurchase(ctx, repo.CreatePurchaseParams{
			InvoiceNumber: boInvoice,
			SupplierID:    po.SupplierID,
			PaymentMethod: paymentMethod,
			TotalCost:     floatToNumeric(boTotal),
			Status:        "pending",
			Notes:         boNotes,
			ReceivedAt:    pgtype.Timestamptz{Valid: false},
		})
		if err != nil {
			return err
		}

		for _, bo := range backorders {
			boSub := float64(bo.remainingQty) * bo.cost
			err = qtx.CreatePurchaseDetail(ctx, repo.CreatePurchaseDetailParams{
				PurchaseID:    newPOID,
				ProductID:     bo.detail.ProductID,
				UnitID:        bo.detail.UnitID,
				UnitName:      bo.detail.UnitName,
				ConversionQty: bo.detail.ConversionQty,
				Qty:           bo.remainingQty,
				Cost:          floatToNumeric(bo.cost),
				Subtotal:      floatToNumeric(boSub),
			})
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (s *svc) CancelPurchaseOrder(ctx context.Context, id int32) error {
	return s.repo.CancelPurchaseOrder(ctx, id)
}

func (s *svc) GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error) {
	return s.repo.GetCompanyConfig(ctx)
}

func (s *svc) SendWhatsAppPO(ctx context.Context, poID int32, targetPhone string) error {
	if s.wa == nil {
		return errors.New("layanan WhatsApp belum aktif atau belum terhubung")
	}

	po, items, err := s.GetPurchaseDetail(ctx, poID)
	if err != nil {
		return err
	}

	phone := strings.TrimSpace(targetPhone)
	if phone == "" {
		phone = po.SupplierPhone
	}
	if phone == "" {
		return errors.New("nomor WhatsApp vendor tidak ditemukan, silakan isi nomor tujuan")
	}

	company, _ := s.GetCompanyConfig(ctx)

	var poItems []whatsapp.PurchaseOrderItem
	for _, it := range items {
		poItems = append(poItems, whatsapp.PurchaseOrderItem{
			ProductName: it.ProductName,
			UnitName:    it.UnitName,
			Qty:         it.Qty,
			Cost:        utils.NumericToFloat(it.Cost),
			Subtotal:    utils.NumericToFloat(it.Subtotal),
		})
	}

	poData := whatsapp.PurchaseOrderData{
		CompanyName:   company.Name,
		StoreAddress:  company.Address,
		StorePhone:    company.Phone,
		InvoiceNumber: po.InvoiceNumber,
		Date:          po.Date.Time.Local(),
		SupplierName:  po.SupplierName,
		SupplierPhone: phone,
		Items:         poItems,
		TotalCost:     utils.NumericToFloat(po.TotalCost),
		Notes:         po.Notes,
	}

	msg := whatsapp.FormatPurchaseOrderText(poData)
	return s.wa.SendMessage(ctx, phone, msg)
}

func (s *svc) ListSuppliers(ctx context.Context) ([]repo.Supplier, error) {
	return s.repo.ListSuppliers(ctx)
}

func calculateReversedMovingAverage(currentStock, removedBaseQty int32, currentAvgCost, removedSubtotal, fallbackPrevCost float64) float64 {
	newStock := currentStock - removedBaseQty
	if newStock <= 0 {
		if fallbackPrevCost > 0 {
			return fallbackPrevCost
		}
		return 0
	}
	remainingValue := (float64(currentStock) * currentAvgCost) - removedSubtotal
	if remainingValue <= 0 {
		if fallbackPrevCost > 0 {
			return fallbackPrevCost
		}
		return 0
	}
	return remainingValue / float64(newStock)
}

func (s *svc) VoidGoodsReceipt(ctx context.Context, id int32) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	purchase, err := qtx.GetPurchaseByID(ctx, id)
	if err != nil {
		return errors.New("penerimaan barang tidak ditemukan")
	}
	if purchase.Status != "diterima" {
		return errors.New("hanya penerimaan barang berstatus 'diterima' yang dapat dibatalkan")
	}

	details, err := qtx.GetPurchaseDetails(ctx, id)
	if err != nil {
		return err
	}
	if len(details) == 0 {
		return errors.New("tidak ada item barang dalam nota penerimaan ini")
	}

	for _, d := range details {
		baseQty := d.Qty * d.ConversionQty
		prod, err := qtx.GetProductByID(ctx, d.ProductID)
		if err != nil {
			return err
		}
		if prod.CurrentStock < baseQty {
			return fmt.Errorf("stok barang '%s' tidak mencukupi untuk dibatalkan (stok saat ini: %d, kuantitas penerimaan: %d). Barang kemungkinan sudah terjual atau terpakai", d.ProductName, prod.CurrentStock, baseQty)
		}
	}

	for _, d := range details {
		baseQty := d.Qty * d.ConversionQty
		prod, err := qtx.GetProductByID(ctx, d.ProductID)
		if err != nil {
			return err
		}

		var fallbackPrevCost pgtype.Numeric
		_ = tx.QueryRow(ctx, `
			SELECT (pd.cost / GREATEST(pd.conversion_qty, 1))::numeric(15,2)
			FROM purchase_details pd
			JOIN purchases pur ON pd.purchase_id = pur.id
			WHERE pd.product_id = $1 AND pur.id != $2 AND pur.status = 'diterima'
			ORDER BY pur.id DESC, pd.id DESC
			LIMIT 1
		`, d.ProductID, id).Scan(&fallbackPrevCost)

		newStock := prod.CurrentStock - baseQty
		newAvgCost := calculateReversedMovingAverage(
			prod.CurrentStock,
			baseQty,
			numericToFloat(prod.AverageCost),
			numericToFloat(d.Subtotal),
			numericToFloat(fallbackPrevCost),
		)

		err = qtx.UpdateProductStockAndCost(ctx, repo.UpdateProductStockAndCostParams{
			ID:           d.ProductID,
			CurrentStock: newStock,
			AverageCost:  floatToNumeric(newAvgCost),
		})
		if err != nil {
			return err
		}

		err = qtx.CreateInventoryLedger(ctx, repo.CreateInventoryLedgerParams{
			ProductID:       d.ProductID,
			TransactionType: "retur_batal",
			ReferenceID:     pgtype.Int4{Int32: id, Valid: true},
			QtyChange:       -baseQty,
			BalanceAfter:    newStock,
			Notes:           fmt.Sprintf("Pembatalan penerimaan %s (%d %s)", purchase.InvoiceNumber, d.Qty, d.UnitName),
		})
		if err != nil {
			return err
		}
	}

	boPattern := purchase.InvoiceNumber + "-BO%"
	_ = qtx.CancelPendingBackordersByPrefix(ctx, boPattern)

	err = qtx.VoidGoodsReceipt(ctx, id)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *svc) ReopenPurchaseOrder(ctx context.Context, id int32) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.(*repo.Queries).WithTx(tx)

	purchase, err := qtx.GetPurchaseByID(ctx, id)
	if err != nil {
		return errors.New("Purchase Order tidak ditemukan")
	}
	if purchase.Status != "batal" {
		return errors.New("hanya PO berstatus 'batal' yang dapat dibuka kembali")
	}
	if !strings.HasPrefix(purchase.InvoiceNumber, "PO-") {
		return errors.New("hanya Purchase Order (kode PO-) yang dapat dibuka ulang")
	}

	err = qtx.ReopenPurchaseOrder(ctx, id)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}



