package sales

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/go-chi/chi/v5"
)

type SalesFilter struct {
	Query  string
	Status string
	Method string
	Period string
	From   string
	To     string
}

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/pos", h.HandlePOSPage)
	r.Post("/pos/checkout", h.HandleCheckout)

	r.Get("/sales", h.ListSales)
	r.Get("/sales/{id}", h.GetSaleDetail)
	r.Get("/sales/{id}/receipt", h.GetSaleReceipt)
	r.Post("/sales/{id}/void", h.VoidSale)
	r.Post("/sales/{id}/pay-debt", h.PayDebt)
	r.Post("/sales/{id}/send-whatsapp", h.SendReceiptWhatsApp)
}

func (h *Handler) HandlePOSPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.GetPOSMasterData(r.Context())
	if err != nil {
		log.Println("Error loading POS master data:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	POSPage(data).Render(r.Context(), w)
}

func (h *Handler) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	var req CreateSaleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format data tidak valid: "+err.Error(), http.StatusBadRequest)
		return
	}

	saleID, invoiceNo, waAutoSent, err := h.service.CreateSale(r.Context(), req)
	if err != nil {
		log.Println("Error creating sale:", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"sale_id":      saleID,
		"invoice_no":   invoiceNo,
		"wa_auto_sent": waAutoSent,
	})
}

func (h *Handler) ListSales(w http.ResponseWriter, r *http.Request) {
	all, err := h.service.ListSales(r.Context())
	if err != nil {
		log.Println("Error listing sales:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rawQ := r.URL.Query().Get("q")
	q := strings.ToLower(strings.TrimSpace(rawQ))
	statusFilter := r.URL.Query().Get("status")
	methodFilter := r.URL.Query().Get("method")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30_days"
	}
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	startDate, endDate, hasDateFilter := utils.ParseDateFilter(period, fromStr, toStr)

	var filtered []repo.ListSalesRow
	for _, s := range all {
		if hasDateFilter && s.Date.Valid {
			if !utils.InDateRange(s.Date.Time, startDate, endDate) {
				continue
			}
		}

		if q != "" {
			matchInvoice := strings.Contains(strings.ToLower(s.InvoiceNo), q)
			matchCust := strings.Contains(strings.ToLower(s.CustomerName), q)
			if !matchInvoice && !matchCust {
				continue
			}
		}

		if statusFilter != "" && s.Status != statusFilter {
			continue
		}

		if methodFilter != "" && s.PaymentMethod != methodFilter {
			continue
		}

		filtered = append(filtered, s)
	}

	slices.SortFunc(filtered, func(a, b repo.ListSalesRow) int {
		return int(b.ID - a.ID)
	})

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		SalesTable(filtered).Render(r.Context(), w)
		return
	}

	filter := SalesFilter{
		Query:  rawQ,
		Status: statusFilter,
		Method: methodFilter,
		Period: period,
		From:   fromStr,
		To:     toStr,
	}

	SalesPage(filtered, filter).Render(r.Context(), w)
}

func (h *Handler) GetSaleDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	sale, details, err := h.service.GetSaleDetail(r.Context(), int32(id))
	if err != nil {
		log.Println("Error getting sale detail:", err)
		http.Error(w, "Penjualan tidak ditemukan", http.StatusNotFound)
		return
	}

	customers, _ := h.service.ListCustomers(r.Context())

	SaleDetailPage(sale, details, customers).Render(r.Context(), w)
}

func (h *Handler) GetSaleReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	sale, details, err := h.service.GetSaleDetail(r.Context(), int32(id))
	if err != nil {
		log.Println("Error getting receipt:", err)
		http.Error(w, "Penjualan tidak ditemukan", http.StatusNotFound)
		return
	}

	company, _ := h.service.GetCompanyConfig(r.Context())

	ReceiptView(sale, details, company).Render(r.Context(), w)
}

func (h *Handler) VoidSale(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	err = h.service.VoidSale(r.Context(), int32(id))
	if err != nil {
		log.Println("Error voiding sale:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/sales/%d", id))
}

func (h *Handler) PayDebt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	amountStr := r.FormValue("amount")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		http.Error(w, "Jumlah bayar tidak valid", http.StatusBadRequest)
		return
	}

	notes := r.FormValue("notes")
	err = h.service.RecordDebtPayment(r.Context(), int32(id), amount, notes)
	if err != nil {
		log.Println("Error paying debt:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/sales/%d", id))
}

func (h *Handler) SendReceiptWhatsApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	var req struct {
		Phone string `json:"phone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	err = h.service.SendReceiptViaWhatsApp(r.Context(), int32(id), req.Phone)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Nota berhasil dikirim ke WhatsApp!",
	})
}

