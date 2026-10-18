package sales

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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SaleItemReq struct {
	ProductID     int32   `json:"product_id"`
	UnitID        int32   `json:"unit_id"`
	UnitName      string  `json:"unit_name"`
	ConversionQty int32   `json:"conversion_qty"`
	Qty           int32   `json:"qty"`
	Price         float64 `json:"price"`
	Discount      float64 `json:"discount"`
	Subtotal      float64 `json:"subtotal"`
	IsOpenPrice   bool    `json:"is_open_price"`
}

type CreateSaleReq struct {
	InvoiceNo      string        `json:"invoice_no"`
	CustomerID     *int32        `json:"customer_id"`
	PaymentMethod  string        `json:"payment_method"` // "tunai", "transfer", "qris", "kasbon"
	TotalAmount    float64       `json:"total_amount"`
	DiscountAmount float64       `json:"discount_amount"`
	PaidAmount     float64       `json:"paid_amount"`
	ChangeAmount   float64       `json:"change_amount"`
	DueDate        *time.Time    `json:"due_date"`
	Notes          string        `json:"notes"`
	Items          []SaleItemReq `json:"items"`
}

type POSMasterData struct {
	Products      []repo.ListPOSProductsRow
	Units         []repo.ListPOSProductUnitsRow
	Categories    []repo.Category
	Customers     []repo.Customer
	RecentSales   []repo.ListSalesRow
	InvoiceNo     string
	CompanyConfig repo.GetCompanyConfigRow
}

type Service interface {
	GenerateNextInvoiceNumber(ctx context.Context) (string, error)
	GetPOSMasterData(ctx context.Context) (POSMasterData, error)
	GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error)
	CreateSale(ctx context.Context, req CreateSaleReq) (int32, string, bool, error)
	ListSales(ctx context.Context) ([]repo.ListSalesRow, error)
	GetSaleDetail(ctx context.Context, id int32) (repo.GetSaleByIDRow, []repo.GetSaleDetailsRow, error)
	VoidSale(ctx context.Context, id int32) error
	RecordDebtPayment(ctx context.Context, saleID int32, amount float64, notes string) error
	SendReceiptViaWhatsApp(ctx context.Context, saleID int32, targetPhone string) error
	ListCustomers(ctx context.Context) ([]repo.Customer, error)
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

func (s *svc) GenerateNextInvoiceNumber(ctx context.Context) (string, error) {
	todayStr := time.Now().Format("20060102")
	pattern := fmt.Sprintf("TRX-%s-%%", todayStr)

	var lastInvoice string
	err := s.db.QueryRow(ctx, `
		SELECT invoice_no 
		FROM sales 
		WHERE invoice_no LIKE $1 
		ORDER BY invoice_no DESC, id DESC 
		LIMIT 1
	`, pattern).Scan(&lastInvoice)

	if err != nil || lastInvoice == "" {
		return fmt.Sprintf("TRX-%s-0001", todayStr), nil
	}

	parts := strings.Split(lastInvoice, "-")
	if len(parts) >= 3 {
		lastNum, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			return fmt.Sprintf("TRX-%s-%04d", todayStr, lastNum+1), nil
		}
	}

	return fmt.Sprintf("TRX-%s-0001", todayStr), nil
}

func (s *svc) GetPOSMasterData(ctx context.Context) (POSMasterData, error) {
	products, err := s.repo.ListPOSProducts(ctx)
	if err != nil {
		return POSMasterData{}, fmt.Errorf("gagal memuat data produk: %w", err)
	}

	units, err := s.repo.ListPOSProductUnits(ctx)
	if err != nil {
		return POSMasterData{}, fmt.Errorf("gagal memuat satuan produk: %w", err)
	}

	categories, err := s.repo.ListCategories(ctx)
	if err != nil {
		return POSMasterData{}, fmt.Errorf("gagal memuat kategori: %w", err)
	}

	customers, err := s.repo.ListCustomers(ctx)
	if err != nil {
		return POSMasterData{}, fmt.Errorf("gagal memuat pelanggan: %w", err)
	}

	invoiceNo, err := s.GenerateNextInvoiceNumber(ctx)
	if err != nil {
		invoiceNo = fmt.Sprintf("TRX-%s-0001", time.Now().Format("20060102"))
	}

	recentSales, err := s.repo.ListSales(ctx)
	if err != nil {
		recentSales = nil
	}
	if len(recentSales) > 30 {
		recentSales = recentSales[:30]
	}

	company, err := s.GetCompanyConfig(ctx)
	if err != nil {
		company = repo.GetCompanyConfigRow{
			ID:            1,
			Name:          "Toko Ubaidillah",
			Tagline:       "ATK & Layanan Fotocopy",
			Phone:         "0812-3456-7890",
			Address:       "",
			ReceiptFooter: "Terima Kasih Atas Kunjungan Anda!\nBarang yang dibeli tidak dapat ditukar/dikembalikan",
		}
	}

	return POSMasterData{
		Products:      products,
		Units:         units,
		Categories:    categories,
		Customers:     customers,
		RecentSales:   recentSales,
		InvoiceNo:     invoiceNo,
		CompanyConfig: company,
	}, nil
}

func (s *svc) GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error) {
	cfg, err := s.repo.GetCompanyConfig(ctx)
	if err != nil {
		return repo.GetCompanyConfigRow{
			ID:            1,
			Name:          "Toko Ubaidillah",
			Tagline:       "ATK & Layanan Fotocopy",
			Phone:         "0812-3456-7890",
			Address:       "",
			ReceiptFooter: "Terima Kasih Atas Kunjungan Anda!\nBarang yang dibeli tidak dapat ditukar/dikembalikan",
		}, nil
	}
	return cfg, nil
}

func (s *svc) CreateSale(ctx context.Context, req CreateSaleReq) (int32, string, bool, error) {
	if len(req.Items) == 0 {
		return 0, "", false, errors.New("keranjang belanja kosong")
	}

	invoiceNo := req.InvoiceNo
	if strings.TrimSpace(invoiceNo) == "" {
		var err error
		invoiceNo, err = s.GenerateNextInvoiceNumber(ctx)
		if err != nil {
			return 0, "", false, err
		}
	}

	paymentMethod := strings.ToLower(strings.TrimSpace(req.PaymentMethod))
	if paymentMethod == "" {
		paymentMethod = "tunai"
	}

	paymentStatus := "lunas"
	if paymentMethod == "kasbon" {
		paymentStatus = "kasbon"
		if req.CustomerID == nil || *req.CustomerID <= 0 {
			return 0, "", false, errors.New("pelanggan walk-in tidak bisa kasbon, transaksi kasbon wajib memilih pelanggan terdaftar")
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, "", false, err
	}
	defer tx.Rollback(ctx)

	var custID pgtype.Int4
	if req.CustomerID != nil && *req.CustomerID > 0 {
		custID = pgtype.Int4{Int32: *req.CustomerID, Valid: true}
	}

	var dueDate pgtype.Date
	if req.DueDate != nil {
		dueDate = pgtype.Date{Time: *req.DueDate, Valid: true}
	}

	var saleID int32
	err = tx.QueryRow(ctx, `
		INSERT INTO sales (
			invoice_no, customer_id, date, total_amount, discount_amount, paid_amount, 
			change_amount, payment_method, payment_status, due_date, status, notes
		) VALUES (
			$1, $2, CURRENT_TIMESTAMP, $3, $4, $5, $6, $7, $8, $9, 'sukses', $10
		) RETURNING id
	`,
		invoiceNo,
		custID,
		utils.FloatToNumeric(req.TotalAmount),
		utils.FloatToNumeric(req.DiscountAmount),
		utils.FloatToNumeric(req.PaidAmount),
		utils.FloatToNumeric(req.ChangeAmount),
		paymentMethod,
		paymentStatus,
		dueDate,
		req.Notes,
	).Scan(&saleID)

	if err != nil {
		return 0, "", false, fmt.Errorf("gagal membuat data penjualan: %w", err)
	}

	for _, item := range req.Items {
		var pType string
		var currentStock int32
		var avgCost pgtype.Numeric
		var isOpenPrice bool

		err = tx.QueryRow(ctx, `
			SELECT type, current_stock, average_cost, is_open_price 
			FROM products 
			WHERE id = $1 
			FOR UPDATE
		`, item.ProductID).Scan(&pType, &currentStock, &avgCost, &isOpenPrice)
		if err != nil {
			return 0, "", false, fmt.Errorf("produk ID %d tidak ditemukan: %w", item.ProductID, err)
		}

		convQty := item.ConversionQty
		if convQty < 1 {
			convQty = 1
		}
		cogs := utils.FloatToNumeric(utils.NumericToFloat(avgCost) * float64(convQty))

		if pType != "jasa" {
			baseQty := item.Qty * item.ConversionQty
			newStock := currentStock - baseQty

			_, err = tx.Exec(ctx, `
				UPDATE products 
				SET current_stock = $1, updated_at = CURRENT_TIMESTAMP 
				WHERE id = $2
			`, newStock, item.ProductID)
			if err != nil {
				return 0, "", false, fmt.Errorf("gagal mengurangi stok produk ID %d: %w", item.ProductID, err)
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO inventory_ledgers (
					product_id, transaction_type, reference_id, qty_change, balance_after, notes
				) VALUES (
					$1, 'penjualan', $2, $3, $4, $5
				)
			`, item.ProductID, saleID, -baseQty, newStock, invoiceNo)
			if err != nil {
				return 0, "", false, fmt.Errorf("gagal mencatat ledger inventori: %w", err)
			}
		}

		var unitID pgtype.Int4
		if item.UnitID > 0 {
			unitID = pgtype.Int4{Int32: item.UnitID, Valid: true}
		}

		itemIsOpen := item.IsOpenPrice || isOpenPrice
		_, err = tx.Exec(ctx, `
			INSERT INTO sale_details (
				sale_id, product_id, unit_id, unit_name, conversion_qty, qty, price, discount, subtotal, cogs, is_open_price
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
			)
		`,
			saleID,
			item.ProductID,
			unitID,
			item.UnitName,
			item.ConversionQty,
			item.Qty,
			utils.FloatToNumeric(item.Price),
			utils.FloatToNumeric(item.Discount),
			utils.FloatToNumeric(item.Subtotal),
			cogs,
			itemIsOpen,
		)
		if err != nil {
			return 0, "", false, fmt.Errorf("gagal menyimpan detail penjualan: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, "", false, fmt.Errorf("gagal menyelesaikan transaksi: %w", err)
	}

	waAutoSent := false
	if req.CustomerID != nil && s.wa != nil {
		cust, err := s.repo.GetCustomerByID(ctx, *req.CustomerID)
		if err == nil && cust.Phone.Valid && strings.TrimSpace(cust.Phone.String) != "" {
			waAutoSent = true
			go func(id int32) {
				bgCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
				defer cancel()
				_ = s.SendReceiptViaWhatsApp(bgCtx, id, "")
			}(saleID)
		}
	}

	return saleID, invoiceNo, waAutoSent, nil
}

func (s *svc) ListSales(ctx context.Context) ([]repo.ListSalesRow, error) {
	return s.repo.ListSales(ctx)
}

func (s *svc) GetSaleDetail(ctx context.Context, id int32) (repo.GetSaleByIDRow, []repo.GetSaleDetailsRow, error) {
	sale, err := s.repo.GetSaleByID(ctx, id)
	if err != nil {
		return repo.GetSaleByIDRow{}, nil, err
	}

	details, err := s.repo.GetSaleDetails(ctx, id)
	if err != nil {
		return repo.GetSaleByIDRow{}, nil, err
	}

	return sale, details, nil
}

func (s *svc) VoidSale(ctx context.Context, id int32) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status, invoiceNo string
	err = tx.QueryRow(ctx, `
		SELECT status, invoice_no 
		FROM sales 
		WHERE id = $1 
		FOR UPDATE
	`, id).Scan(&status, &invoiceNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("transaksi penjualan tidak ditemukan")
		}
		return err
	}

	if status == "batal" {
		return errors.New("transaksi ini sudah dibatalkan sebelumnya")
	}

	_, err = tx.Exec(ctx, `
		UPDATE sales 
		SET status = 'batal', updated_at = CURRENT_TIMESTAMP 
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("gagal membatalkan status penjualan: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT 
			sd.product_id, 
			sd.qty, 
			sd.conversion_qty, 
			p.type
		FROM sale_details sd
		JOIN products p ON sd.product_id = p.id
		WHERE sd.sale_id = $1
		FOR UPDATE OF p
	`, id)
	if err != nil {
		return fmt.Errorf("gagal membaca detail penjualan: %w", err)
	}
	defer rows.Close()

	type restoreItem struct {
		productID int32
		qty       int32
		convQty   int32
		pType     string
	}
	var toRestore []restoreItem

	for rows.Next() {
		var it restoreItem
		if err := rows.Scan(&it.productID, &it.qty, &it.convQty, &it.pType); err != nil {
			return err
		}
		toRestore = append(toRestore, it)
	}
	rows.Close()

	for _, it := range toRestore {
		if it.pType != "jasa" {
			restoreQty := it.qty * it.convQty
			var newStock int32
			err = tx.QueryRow(ctx, `
				UPDATE products 
				SET current_stock = current_stock + $1, updated_at = CURRENT_TIMESTAMP 
				WHERE id = $2
				RETURNING current_stock
			`, restoreQty, it.productID).Scan(&newStock)
			if err != nil {
				return fmt.Errorf("gagal mengembalikan stok produk: %w", err)
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO inventory_ledgers (
					product_id, transaction_type, reference_id, qty_change, balance_after, notes
				) VALUES (
					$1, 'retur_batal', $2, $3, $4, $5
				)
			`, it.productID, id, restoreQty, newStock, fmt.Sprintf("Batal %s", invoiceNo))
			if err != nil {
				return fmt.Errorf("gagal mencatat ledger pengembalian stok: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (s *svc) RecordDebtPayment(ctx context.Context, saleID int32, amount float64, notes string) error {
	if amount <= 0 {
		return errors.New("jumlah pembayaran hutang harus lebih dari 0")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var totalAmount, paidAmount pgtype.Numeric
	var paymentStatus, status string
	err = tx.QueryRow(ctx, `
		SELECT total_amount, paid_amount, payment_status, status 
		FROM sales 
		WHERE id = $1 
		FOR UPDATE
	`, saleID).Scan(&totalAmount, &paidAmount, &paymentStatus, &status)
	if err != nil {
		return err
	}

	if status == "batal" {
		return errors.New("nota penjualan yang sudah dibatalkan tidak dapat menerima pembayaran")
	}

	currPaid := utils.NumericToFloat(paidAmount)
	total := utils.NumericToFloat(totalAmount)
	remaining := total - currPaid
	if remaining <= 0 || paymentStatus == "lunas" {
		return errors.New("nota ini sudah lunas")
	}
	if amount > remaining {
		amount = remaining
	}

	newPaid := currPaid + amount
	newStatus := paymentStatus
	if newPaid >= total {
		newStatus = "lunas"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO debt_payments (sale_id, amount_paid, notes) 
		VALUES ($1, $2, $3)
	`, saleID, utils.FloatToNumeric(amount), notes)
	if err != nil {
		return fmt.Errorf("gagal menyimpan catatan pembayaran kasbon: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE sales 
		SET paid_amount = $1, payment_status = $2, updated_at = CURRENT_TIMESTAMP 
		WHERE id = $3
	`, utils.FloatToNumeric(newPaid), newStatus, saleID)
	if err != nil {
		return fmt.Errorf("gagal memperbarui status pembayaran: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *svc) SendReceiptViaWhatsApp(ctx context.Context, saleID int32, targetPhone string) error {
	if s.wa == nil {
		return errors.New("layanan WhatsApp tidak tersedia")
	}

	sale, details, err := s.GetSaleDetail(ctx, saleID)
	if err != nil {
		return err
	}

	phone := targetPhone
	if phone == "" {
		phone = sale.CustomerPhone
	}
	if phone == "" {
		return errors.New("nomor WhatsApp pelanggan tidak ditemukan")
	}

	company, _ := s.GetCompanyConfig(ctx)

	var items []whatsapp.SaleReceiptItem
	for _, d := range details {
		items = append(items, whatsapp.SaleReceiptItem{
			ProductName: d.ProductName,
			UnitName:    d.UnitName,
			Qty:         d.Qty,
			Price:       utils.NumericToFloat(d.Price),
			Discount:    utils.NumericToFloat(d.Discount),
			Subtotal:    utils.NumericToFloat(d.Subtotal),
			IsOpenPrice: d.IsOpenPrice.Bool,
		})
	}

	receiptData := whatsapp.SaleReceiptData{
		CompanyName:   company.Name,
		Tagline:       company.Tagline,
		Address:       company.Address,
		Phone:         company.Phone,
		ReceiptFooter: company.ReceiptFooter,
		InvoiceNo:     sale.InvoiceNo,
		Date:          sale.Date.Time.Local(),
		CustomerName:  sale.CustomerName,
		CustomerPhone: phone,
		Items:         items,
		DiscountNota:  utils.NumericToFloat(sale.DiscountAmount),
		TotalAmount:   utils.NumericToFloat(sale.TotalAmount),
		PaidAmount:    utils.NumericToFloat(sale.PaidAmount),
		ChangeAmount:  utils.NumericToFloat(sale.ChangeAmount),
		PaymentMethod: sale.PaymentMethod,
		PaymentStatus: sale.PaymentStatus,
	}
	if sale.DueDate.Valid {
		t := sale.DueDate.Time
		receiptData.DueDate = &t
	}

	msg := whatsapp.FormatReceiptText(receiptData)
	return s.wa.SendMessage(ctx, phone, msg)
}

func (s *svc) ListCustomers(ctx context.Context) ([]repo.Customer, error) {
	return s.repo.ListCustomers(ctx)
}
