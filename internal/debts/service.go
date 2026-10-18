package debts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/whatsapp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DebtsSummary struct {
	TotalOutstanding      float64
	TotalCustomersInDebt  int64
	TotalOverdue          float64
	TotalSettledThisMonth float64
}

type CustomerDebtDetail struct {
	Customer       repo.Customer
	UnpaidInvoices []repo.ListCustomerUnpaidInvoicesRow
	PaymentHistory []repo.ListDebtPaymentsByCustomerIDRow
	TotalDebt      float64
	TotalPaid      float64
	RemainingDebt  float64
}

type DebtsPageData struct {
	ActiveTab            string // "customers" or "invoices"
	Summary              DebtsSummary
	CustomerRecap        []repo.ListCustomerDebtRecapRow
	Invoices             []repo.ListDebtInvoicesRow
	Query                string
	Filter               string
	Page                 int
	Limit                int
	TotalInvoices        int64
	TotalPages           int
	CompanyPhone         string
	CompanyName          string
}

type Service interface {
	GetDebtsSummary(ctx context.Context) (*DebtsSummary, error)
	ListCustomerDebtRecap(ctx context.Context, query string, filter string) ([]repo.ListCustomerDebtRecapRow, error)
	ListDebtInvoices(ctx context.Context, query string, filter string, page int, limit int) ([]repo.ListDebtInvoicesRow, int64, error)
	GetCustomerDebtDetail(ctx context.Context, customerID int32) (*CustomerDebtDetail, error)
	RecordSaleDebtPayment(ctx context.Context, saleID int32, amount float64, notes string) error
	RecordCustomerDebtPaymentFIFO(ctx context.Context, customerID int32, amount float64, notes string) error
	GetDebtReminderDraft(ctx context.Context, customerID int32, targetPhone string) (string, string, bool, error)
	SendDebtReminderWhatsApp(ctx context.Context, customerID int32, targetPhone string, message string) error
	GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error)
}

type service struct {
	repo repo.Querier
	db   *pgxpool.Pool
	wa   *whatsapp.Service
}

func NewService(r repo.Querier, db *pgxpool.Pool, wa *whatsapp.Service) Service {
	return &service{repo: r, db: db, wa: wa}
}

func (s *service) GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error) {
	return s.repo.GetCompanyConfig(ctx)
}

func (s *service) GetDebtsSummary(ctx context.Context) (*DebtsSummary, error) {
	metrics, err := s.repo.GetDebtsSummaryMetrics(ctx)
	if err != nil {
		return nil, err
	}

	return &DebtsSummary{
		TotalOutstanding:      utils.NumericToFloat(metrics.TotalOutstanding),
		TotalCustomersInDebt:  metrics.TotalCustomersInDebt,
		TotalOverdue:          utils.NumericToFloat(metrics.TotalOverdue),
		TotalSettledThisMonth: utils.NumericToFloat(metrics.TotalSettledThisMonth),
	}, nil
}

func (s *service) ListCustomerDebtRecap(ctx context.Context, query string, filter string) ([]repo.ListCustomerDebtRecapRow, error) {
	searchPattern := ""
	if strings.TrimSpace(query) != "" {
		searchPattern = "%" + strings.TrimSpace(query) + "%"
	}

	if filter == "" {
		filter = "all"
	}

	return s.repo.ListCustomerDebtRecap(ctx, repo.ListCustomerDebtRecapParams{
		Column1: searchPattern,
		Column2: filter,
	})
}

func (s *service) ListDebtInvoices(ctx context.Context, query string, filter string, page int, limit int) ([]repo.ListDebtInvoicesRow, int64, error) {
	searchPattern := ""
	if strings.TrimSpace(query) != "" {
		searchPattern = "%" + strings.TrimSpace(query) + "%"
	}

	if filter == "" {
		filter = "kasbon"
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 15
	}
	offset := (page - 1) * limit

	total, err := s.repo.CountDebtInvoices(ctx, repo.CountDebtInvoicesParams{
		Column1: searchPattern,
		Column2: filter,
	})
	if err != nil {
		return nil, 0, err
	}

	invoices, err := s.repo.ListDebtInvoices(ctx, repo.ListDebtInvoicesParams{
		Column1: searchPattern,
		Column2: filter,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	return invoices, total, nil
}

func (s *service) GetCustomerDebtDetail(ctx context.Context, customerID int32) (*CustomerDebtDetail, error) {
	cust, err := s.repo.GetCustomerByIDAnyStatus(ctx, customerID)
	if err != nil {
		return nil, err
	}

	unpaid, err := s.repo.ListCustomerUnpaidInvoices(ctx, pgtype.Int4{Int32: customerID, Valid: true})
	if err != nil {
		return nil, err
	}

	history, err := s.repo.ListDebtPaymentsByCustomerID(ctx, pgtype.Int4{Int32: customerID, Valid: true})
	if err != nil {
		return nil, err
	}

	var totalDebt, totalPaid, remaining float64
	for _, inv := range unpaid {
		totalDebt += utils.NumericToFloat(inv.TotalAmount)
		totalPaid += utils.NumericToFloat(inv.PaidAmount)
		remaining += utils.NumericToFloat(inv.RemainingAmount)
	}

	return &CustomerDebtDetail{
		Customer:       cust,
		UnpaidInvoices: unpaid,
		PaymentHistory: history,
		TotalDebt:      totalDebt,
		TotalPaid:      totalPaid,
		RemainingDebt:  remaining,
	}, nil
}

func (s *service) RecordSaleDebtPayment(ctx context.Context, saleID int32, amount float64, notes string) error {
	if amount <= 0 {
		return errors.New("nominal pembayaran harus lebih besar dari 0")
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
		return fmt.Errorf("gagal memperbarui status pembayaran nota: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *service) RecordCustomerDebtPaymentFIFO(ctx context.Context, customerID int32, amount float64, notes string) error {
	if amount <= 0 {
		return errors.New("nominal pembayaran harus lebih besar dari 0")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, total_amount, paid_amount, payment_status
		FROM sales
		WHERE customer_id = $1 AND status != 'batal' AND payment_status = 'kasbon'
		ORDER BY id ASC
		FOR UPDATE
	`, customerID)
	if err != nil {
		return err
	}

	type unpaidSale struct {
		id            int32
		total         float64
		paid          float64
		paymentStatus string
	}
	var salesList []unpaidSale

	for rows.Next() {
		var u unpaidSale
		var t, p pgtype.Numeric
		if err := rows.Scan(&u.id, &t, &p, &u.paymentStatus); err != nil {
			rows.Close()
			return err
		}
		u.total = utils.NumericToFloat(t)
		u.paid = utils.NumericToFloat(p)
		salesList = append(salesList, u)
	}
	rows.Close()

	if len(salesList) == 0 {
		return errors.New("pelanggan tidak memiliki nota kasbon yang belum lunas")
	}

	var totalRemaining float64
	for _, sl := range salesList {
		if rem := sl.total - sl.paid; rem > 0 {
			totalRemaining += rem
		}
	}
	if totalRemaining <= 0 {
		return errors.New("pelanggan tidak memiliki sisa kasbon yang belum lunas")
	}
	if amount > totalRemaining {
		amount = totalRemaining
	}

	remainingPayment := amount
	for _, sl := range salesList {
		if remainingPayment <= 0 {
			break
		}

		unpaidThisSale := sl.total - sl.paid
		if unpaidThisSale <= 0 {
			continue
		}

		payThis := math.Min(remainingPayment, unpaidThisSale)
		newPaid := sl.paid + payThis
		newStatus := "kasbon"
		if newPaid >= sl.total {
			newStatus = "lunas"
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO debt_payments (sale_id, amount_paid, notes) 
			VALUES ($1, $2, $3)
		`, sl.id, utils.FloatToNumeric(payThis), notes)
		if err != nil {
			return fmt.Errorf("gagal mencatat pembayaran nota #%d: %w", sl.id, err)
		}

		_, err = tx.Exec(ctx, `
			UPDATE sales 
			SET paid_amount = $1, payment_status = $2, updated_at = CURRENT_TIMESTAMP 
			WHERE id = $3
		`, utils.FloatToNumeric(newPaid), newStatus, sl.id)
		if err != nil {
			return fmt.Errorf("gagal memperbarui nota #%d: %w", sl.id, err)
		}

		remainingPayment -= payThis
	}

	return tx.Commit(ctx)
}

func (s *service) GetDebtReminderDraft(ctx context.Context, customerID int32, targetPhone string) (string, string, bool, error) {
	detail, err := s.GetCustomerDebtDetail(ctx, customerID)
	if err != nil {
		return "", "", false, err
	}

	phone := strings.TrimSpace(targetPhone)
	if phone == "" {
		phone = strings.TrimSpace(detail.Customer.Phone.String)
	}

	company, _ := s.repo.GetCompanyConfig(ctx)
	compName := company.Name
	if compName == "" {
		compName = "Toko Ubaidillah"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*PENGINGAT KASBON / PIUTANG - %s*\n\n", strings.ToUpper(compName)))
	sb.WriteString(fmt.Sprintf("Yth. *%s*,\n", detail.Customer.Name))
	sb.WriteString("Berikut kami sampaikan rincian tagihan kasbon belanja Anda:\n\n")

	var earliestDue string
	for i, inv := range detail.UnpaidInvoices {
		dateStr := inv.Date.Time.Local().Format("02/01/2006")
		remStr := utils.FormatRupiah(inv.RemainingAmount)
		totStr := utils.FormatRupiah(inv.TotalAmount)
		sb.WriteString(fmt.Sprintf("%d. *%s* (Tgl: %s)\n", i+1, inv.InvoiceNo, dateStr))
		sb.WriteString(fmt.Sprintf("   Total: Rp %s | *Sisa Tagihan: Rp %s*\n", totStr, remStr))
		if inv.DueDate.Valid {
			dueStr := inv.DueDate.Time.Format("02/01/2006")
			if earliestDue == "" {
				earliestDue = dueStr
			}
			if inv.IsOverdue.Bool {
				sb.WriteString(fmt.Sprintf("   ⚠️ *Jatuh Tempo: %s (Lewat)*\n", dueStr))
			} else {
				sb.WriteString(fmt.Sprintf("   Jatuh Tempo: %s\n", dueStr))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\n*TOTAL SISA PIUTANG: Rp %s*\n", utils.FormatRupiah(utils.FloatToNumeric(detail.RemainingDebt))))
	if earliestDue != "" {
		sb.WriteString(fmt.Sprintf("Jatuh Tempo Terdekat: *%s*\n", earliestDue))
	}
	sb.WriteString("\nPembayaran dapat dilakukan langsung di toko atau melalui transfer.\n")
	sb.WriteString(fmt.Sprintf("Terima kasih atas kepercayaannya berbelanja di %s. 🙏", compName))

	msg := sb.String()
	isConnected := s.wa != nil && s.wa.GetStatus().Connected

	return msg, phone, isConnected, nil
}

func (s *service) SendDebtReminderWhatsApp(ctx context.Context, customerID int32, targetPhone string, message string) error {
	if s.wa == nil || !s.wa.GetStatus().Connected {
		return errors.New("WhatsApp belum terhubung. Silakan buka menu Pengaturan untuk memindai QR code WhatsApp toko terlebih dahulu.")
	}

	phone := strings.TrimSpace(targetPhone)
	if phone == "" {
		detail, err := s.GetCustomerDebtDetail(ctx, customerID)
		if err == nil {
			phone = strings.TrimSpace(detail.Customer.Phone.String)
		}
	}
	if phone == "" {
		return errors.New("nomor telepon WhatsApp tujuan tidak boleh kosong")
	}

	msg := strings.TrimSpace(message)
	if msg == "" {
		draft, _, _, err := s.GetDebtReminderDraft(ctx, customerID, phone)
		if err != nil {
			return err
		}
		msg = draft
	}

	return s.wa.SendMessage(ctx, phone, msg)
}
