package purchases

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

type PurchaseFilter struct {
	Query   string
	Status  string
	Payment string
	SortBy  string
	Period  string
	From    string
	To      string
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/purchases", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/purchase-orders", http.StatusFound)
	})

	r.Get("/purchase-orders", h.HandleListPurchaseOrders)
	r.Get("/purchase-orders/new", h.HandleNewPurchaseOrderPage)
	r.Post("/purchase-orders", h.HandleCreatePurchaseOrder)
	r.Get("/purchase-orders/{id}", h.HandlePurchaseOrderDetailPage)
	r.Get("/purchase-orders/{id}/print", h.HandlePrintPOPage)
	r.Post("/purchase-orders/{id}/send-whatsapp", h.HandleSendWhatsAppPO)
	r.Post("/purchase-orders/{id}/cancel", h.HandleCancelPurchaseOrder)

	r.Get("/goods-receipts", h.HandleListGoodsReceipts)
	r.Get("/goods-receipts/new", h.HandleNewGoodsReceiptPage)
	r.Post("/goods-receipts", h.HandleCreateDirectGoodsReceipt)
	r.Get("/goods-receipts/from-po/{id}", h.HandleReceiveFromPOPage)
	r.Post("/goods-receipts/from-po/{id}", h.HandleReceivePurchaseOrder)
	r.Get("/goods-receipts/{id}", h.HandleGoodsReceiptDetailPage)
	r.Post("/goods-receipts/{id}/void", h.HandleVoidGoodsReceipt)
	r.Post("/goods-receipts/{id}/reopen-po", h.HandleReopenPurchaseOrder)
}

func sortPurchases(list []repo.ListPurchasesRow, sortBy string, prioritizePending bool) {
	slices.SortFunc(list, func(a, b repo.ListPurchasesRow) int {
		if prioritizePending {
			pa := 1
			if a.Status == "pending" {
				pa = 0
			}
			pb := 1
			if b.Status == "pending" {
				pb = 0
			}
			if pa != pb {
				return pa - pb
			}
		}
		switch sortBy {
		case "date_asc":
			return int(a.ID - b.ID)
		case "total_desc":
			return cmp.Compare(utils.NumericToFloat(b.TotalCost), utils.NumericToFloat(a.TotalCost))
		case "total_asc":
			return cmp.Compare(utils.NumericToFloat(a.TotalCost), utils.NumericToFloat(b.TotalCost))
		default:
			return int(b.ID - a.ID)
		}
	})
}

func (h *Handler) HandleListPurchaseOrders(w http.ResponseWriter, r *http.Request) {
	all, err := h.svc.ListPurchases(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rawQ := r.URL.Query().Get("q")
	q := strings.ToLower(strings.TrimSpace(rawQ))
	statusFilter := r.URL.Query().Get("status")
	paymentFilter := r.URL.Query().Get("payment")
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "date_desc"
	}

	var pos []repo.ListPurchasesRow
	for _, p := range all {
		if strings.HasPrefix(p.InvoiceNumber, "TB-") {
			continue
		}
		if p.Status == "batal" && p.ReceivedAt.Valid {
			continue
		}
		if statusFilter != "" {
			if p.Status != statusFilter {
				continue
			}
		} else {
			if p.Status != "pending" && p.Status != "batal" {
				continue
			}
		}
		if paymentFilter != "" && p.PaymentMethod != paymentFilter {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(p.InvoiceNumber), q) &&
			!strings.Contains(strings.ToLower(p.SupplierName), q) &&
			!strings.Contains(strings.ToLower(p.Notes), q) {
			continue
		}
		pos = append(pos, p)
	}

	sortPurchases(pos, sortBy, statusFilter == "" && sortBy == "date_desc")

	filter := PurchaseFilter{
		Query:   rawQ,
		Status:  statusFilter,
		Payment: paymentFilter,
		SortBy:  sortBy,
	}

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		PurchaseOrderTable(pos).Render(r.Context(), w)
		return
	}

	PurchaseOrderListPage(pos, filter).Render(r.Context(), w)
}

func (h *Handler) HandleNewPurchaseOrderPage(w http.ResponseWriter, r *http.Request) {
	suppliers, products, units, defaultInvoice, err := h.svc.GetFormMasterData(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	CreatePurchaseOrderPage(suppliers, products, units, defaultInvoice).Render(r.Context(), w)
}

func (h *Handler) HandleCreatePurchaseOrder(w http.ResponseWriter, r *http.Request) {
	req, err := parseCreatePurchaseForm(r, "pending")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.CreatePurchase(r.Context(), req); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/purchase-orders")
}

func (h *Handler) HandlePurchaseOrderDetailPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	purchase, items, err := h.svc.GetPurchaseDetail(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Purchase Order tidak ditemukan", http.StatusNotFound)
		return
	}

	PurchaseOrderDetailPage(purchase, items).Render(r.Context(), w)
}

func (h *Handler) HandlePrintPOPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	po, items, err := h.svc.GetPurchaseDetail(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Purchase Order tidak ditemukan", http.StatusNotFound)
		return
	}

	company, _ := h.svc.GetCompanyConfig(r.Context())

	PurchaseOrderPrintView(po, items, company).Render(r.Context(), w)
}

func (h *Handler) HandleSendWhatsAppPO(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	var req struct {
		Phone string `json:"phone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	err = h.svc.SendWhatsAppPO(r.Context(), int32(id), req.Phone)
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
		"message": "Purchase Order berhasil dikirim ke WhatsApp vendor!",
	})
}

func (h *Handler) HandleCancelPurchaseOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if err := h.svc.CancelPurchaseOrder(r.Context(), int32(id)); err != nil {
		log.Println(err)
		http.Error(w, "Gagal membatalkan PO", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/purchase-orders")
}

func (h *Handler) HandleListGoodsReceipts(w http.ResponseWriter, r *http.Request) {
	all, err := h.svc.ListPurchases(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rawQ := r.URL.Query().Get("q")
	q := strings.ToLower(strings.TrimSpace(rawQ))
	paymentFilter := r.URL.Query().Get("payment")
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "date_desc"
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30_days"
	}
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	startDate, endDate, hasDateFilter := utils.ParseDateFilter(period, fromStr, toStr)

	var pendingPOs []repo.ListPurchasesRow
	var receipts []repo.ListPurchasesRow
	for _, p := range all {
		if p.Status == "pending" {
			pendingPOs = append(pendingPOs, p)
		} else if p.Status == "diterima" || (p.Status == "batal" && (p.ReceivedAt.Valid || strings.HasPrefix(p.InvoiceNumber, "TB-"))) {
			if hasDateFilter {
				checkDate := p.Date.Time
				if p.ReceivedAt.Valid {
					checkDate = p.ReceivedAt.Time
				}
				if !utils.InDateRange(checkDate, startDate, endDate) {
					continue
				}
			}
			if paymentFilter != "" && p.PaymentMethod != paymentFilter {
				continue
			}
			if q != "" &&
				!strings.Contains(strings.ToLower(p.InvoiceNumber), q) &&
				!strings.Contains(strings.ToLower(p.SupplierName), q) &&
				!strings.Contains(strings.ToLower(p.Notes), q) {
				continue
			}
			receipts = append(receipts, p)
		}
	}

	sortPurchases(receipts, sortBy, false)

	filter := PurchaseFilter{
		Query:   rawQ,
		Payment: paymentFilter,
		SortBy:  sortBy,
		Period:  period,
		From:    fromStr,
		To:      toStr,
	}

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		GoodsReceiptTable(receipts).Render(r.Context(), w)
		return
	}

	GoodsReceiptListPage(pendingPOs, receipts, filter).Render(r.Context(), w)
}

func (h *Handler) HandleNewGoodsReceiptPage(w http.ResponseWriter, r *http.Request) {
	suppliers, products, units, _, err := h.svc.GetFormMasterData(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	all, err := h.svc.ListPurchases(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var pendingPOs []repo.ListPurchasesRow
	for _, p := range all {
		if p.Status == "pending" {
			pendingPOs = append(pendingPOs, p)
		}
	}

	defaultTB, err := h.svc.GenerateNextInvoiceNumber(r.Context(), "TB")
	if err != nil {
		defaultTB = fmt.Sprintf("TB-%s-0001", time.Now().Format("20060102"))
	}
	CreateGoodsReceiptPage(suppliers, products, units, pendingPOs, defaultTB).Render(r.Context(), w)
}

func (h *Handler) HandleCreateDirectGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	req, err := parseCreatePurchaseForm(r, "diterima")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.CreatePurchase(r.Context(), req); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/goods-receipts")
}

func (h *Handler) HandleReceiveFromPOPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	purchase, items, err := h.svc.GetPurchaseDetail(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "PO tidak ditemukan", http.StatusNotFound)
		return
	}

	ReceiveFromPOPage(purchase, items).Render(r.Context(), w)
}

func (h *Handler) HandleReceivePurchaseOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	detailIDs := r.Form["detail_ids[]"]
	receivedQtys := r.Form["received_qtys[]"]
	actualCosts := r.Form["actual_costs[]"]
	shortageActions := r.Form["shortage_actions[]"]

	var items []ReceiveItemInput
	for i := range detailIDs {
		if i >= len(receivedQtys) || i >= len(actualCosts) {
			continue
		}
		did, _ := strconv.Atoi(detailIDs[i])
		rqty, _ := strconv.Atoi(receivedQtys[i])
		acost, _ := strconv.ParseFloat(actualCosts[i], 64)

		if did <= 0 {
			continue
		}

		createBO := i < len(shortageActions) && shortageActions[i] == "backorder"

		items = append(items, ReceiveItemInput{
			DetailID:        int32(did),
			ReceivedQty:     int32(rqty),
			ActualCost:      acost,
			CreateBackorder: createBO,
		})
	}

	req := ReceivePurchaseReq{
		PurchaseID:    int32(id),
		PaymentMethod: r.FormValue("payment_method"),
		Notes:         r.FormValue("notes"),
		Items:         items,
	}

	if err := h.svc.ReceivePurchaseOrder(r.Context(), req); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/goods-receipts")
}

func (h *Handler) HandleGoodsReceiptDetailPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	purchase, items, err := h.svc.GetPurchaseDetail(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Nota Terima Barang tidak ditemukan", http.StatusNotFound)
		return
	}

	GoodsReceiptDetailPage(purchase, items).Render(r.Context(), w)
}

func (h *Handler) HandleVoidGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if err := h.svc.VoidGoodsReceipt(r.Context(), int32(id)); err != nil {
		log.Println("Error voiding goods receipt:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/goods-receipts/%d", id))
}

func (h *Handler) HandleReopenPurchaseOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if err := h.svc.ReopenPurchaseOrder(r.Context(), int32(id)); err != nil {
		log.Println("Error reopening purchase order:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/goods-receipts/from-po/%d", id))
}


func parseCreatePurchaseForm(r *http.Request, forcedStatus string) (CreatePurchaseReq, error) {
	r.ParseForm()

	supID, _ := strconv.Atoi(r.FormValue("supplier_id"))

	prodIDs := r.Form["product_ids[]"]
	unitIDs := r.Form["unit_ids[]"]
	unitNames := r.Form["unit_names[]"]
	convQtys := r.Form["conversion_qtys[]"]
	qtys := r.Form["qtys[]"]
	costs := r.Form["costs[]"]

	var items []PurchaseItemReq
	for i := range prodIDs {
		if i >= len(unitIDs) || i >= len(unitNames) || i >= len(convQtys) || i >= len(qtys) || i >= len(costs) {
			continue
		}
		pid, _ := strconv.Atoi(prodIDs[i])
		uid, _ := strconv.Atoi(unitIDs[i])
		qty, _ := strconv.Atoi(qtys[i])
		conv, _ := strconv.Atoi(convQtys[i])
		cost, _ := strconv.ParseFloat(costs[i], 64)

		if pid <= 0 || qty <= 0 {
			continue
		}
		if conv < 1 {
			conv = 1
		}

		items = append(items, PurchaseItemReq{
			ProductID:     int32(pid),
			UnitID:        int32(uid),
			UnitName:      unitNames[i],
			ConversionQty: int32(conv),
			Qty:           int32(qty),
			Cost:          cost,
		})
	}

	return CreatePurchaseReq{
		InvoiceNumber: r.FormValue("invoice_number"),
		SupplierID:    int32(supID),
		PaymentMethod: r.FormValue("payment_method"),
		Status:        forcedStatus,
		Notes:         r.FormValue("notes"),
		Items:         items,
	}, nil
}
