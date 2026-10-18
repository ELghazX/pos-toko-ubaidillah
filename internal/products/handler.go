package products

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
)

type Handler struct {
	svc  Service
	repo repo.Querier
}

func NewHandler(svc Service, repo repo.Querier) *Handler {
	return &Handler{svc: svc, repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/products", h.HandleListProducts)
	r.Get("/products/new", h.HandleNewProductPage)
	r.Post("/products", h.HandleCreateProduct)
	r.Get("/products/print-labels", h.HandlePrintLabelsPage)
	r.Get("/products/print-labels/preview", h.HandlePrintLabelsPreview)
	r.Post("/products/print-labels/preview", h.HandlePrintLabelsPreview)
	r.Get("/products/{id}/stock-card", h.HandleGetProductStockCard)
	r.Get("/products/{id}/edit", h.HandleEditProductPage)
	r.Post("/products/{id}/edit", h.HandleUpdateProduct)
	r.Delete("/products/{id}", h.HandleDeleteProduct)
	r.Get("/products/generate-sku", h.HandleGenerateSKU)
	r.Post("/products/recalculate-stock", h.HandleRecalculateStock)
	r.Get("/products/recalculate-stock-stream", h.HandleRecalculateStockStream)
	r.Get("/stock-adjustments", h.HandleStockAdjustmentsPage)
	r.Post("/stock-adjustments", h.HandleCreateStockAdjustment)
}


type ProductPageData struct {
	Products      []repo.SearchProductsRow
	Categories    []repo.Category
	Units         []repo.ListPOSProductUnitsRow
	Query         string
	CategoryID    int32
	ProductType   string
	StockFilter   string
	SortBy        string
	Page          int
	Limit         int
	TotalRows     int64
	TotalPages    int
	LowStockCount int64
}

func (h *Handler) HandleListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	catID, _ := strconv.Atoi(r.URL.Query().Get("category_id"))
	prodType := r.URL.Query().Get("type")
	stockFilter := r.URL.Query().Get("stock_filter")
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "newest"
	}

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	limit := 10
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	offset := (page - 1) * limit

	products, total, _ := h.svc.SearchProducts(r.Context(), q, int32(catID), prodType, stockFilter, sortBy, int32(limit), int32(offset))
	categories, _ := h.repo.ListCategories(r.Context())
	units, _ := h.repo.ListPOSProductUnits(r.Context())
	lowStockCount, _ := h.svc.CountLowStock(r.Context())

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages == 0 {
		totalPages = 1
	}

	data := ProductPageData{
		Products:      products,
		Categories:    categories,
		Units:         units,
		Query:         q,
		CategoryID:    int32(catID),
		ProductType:   prodType,
		StockFilter:   stockFilter,
		SortBy:        sortBy,
		Page:          page,
		Limit:         limit,
		TotalRows:     total,
		TotalPages:    totalPages,
		LowStockCount: lowStockCount,
	}

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		ProductTable(data).Render(r.Context(), w)
		return
	}

	ProductPage(data).Render(r.Context(), w)
}

func (h *Handler) HandleNewProductPage(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.repo.ListCategories(r.Context())
	CreateProductPage(categories).Render(r.Context(), w)
}

func parseCascadingUnits(r *http.Request) []UnitReq {
	names := r.Form["unit_names[]"]
	stepQtys := r.Form["step_qtys[]"]
	prices := r.Form["prices[]"]

	type rawUnit struct {
		name    string
		stepQty int32
		price   pgtype.Numeric
	}

	var raw []rawUnit
	for i := range names {
		if names[i] == "" {
			continue
		}
		if i >= len(stepQtys) || i >= len(prices) {
			continue
		}
		step, _ := strconv.Atoi(stepQtys[i])
		if step < 1 {
			step = 1
		}
		var p pgtype.Numeric
		p.Scan(prices[i])

		raw = append(raw, rawUnit{
			name:    names[i],
			stepQty: int32(step),
			price:   p,
		})
	}

	n := len(raw)
	if n == 0 {
		return nil
	}

	units := make([]UnitReq, n)
	units[n-1] = UnitReq{
		UnitName:      raw[n-1].name,
		ConversionQty: 1,
		Price:         raw[n-1].price,
	}

	for i := n - 2; i >= 0; i-- {
		units[i] = UnitReq{
			UnitName:      raw[i].name,
			ConversionQty: units[i+1].ConversionQty * raw[i+1].stepQty,
			Price:         raw[i].price,
		}
	}

	return units
}

func (h *Handler) HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	catID, _ := strconv.Atoi(r.FormValue("category_id"))
	isOpenPrice := r.FormValue("is_open_price") == "on"

	avgCostVal, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("average_cost")), 64)
	if err != nil || avgCostVal < 0 {
		avgCostVal = 0
	}
	avgCost := utils.FloatToNumeric(avgCostVal)

	units := parseCascadingUnits(r)
	if len(units) == 0 {
		http.Error(w, "Minimal harus ada 1 satuan produk", http.StatusBadRequest)
		return
	}

	initStock, _ := strconv.Atoi(r.FormValue("initial_stock"))
	stockUnitIdx, _ := strconv.Atoi(r.FormValue("initial_stock_unit_idx"))

	multiplier := int32(1)
	selectedUnitName := ""
	if stockUnitIdx >= 0 && stockUnitIdx < len(units) {
		multiplier = units[stockUnitIdx].ConversionQty
		selectedUnitName = utils.ToPascalCase(units[stockUnitIdx].UnitName)
	}
	totalStock := int32(initStock) * multiplier

	initialStockNote := "Stok awal produk baru"
	if initStock > 0 && multiplier > 1 && selectedUnitName != "" {
		initialStockNote = fmt.Sprintf("Stok awal produk baru (%d %s)", initStock, selectedUnitName)
	}

	minStock, err := strconv.Atoi(r.FormValue("min_stock"))
	if err != nil || minStock < 0 {
		minStock = 5
	}

	req := CreateProductReq{
		CategoryID:        int32(catID),
		SKU:               r.FormValue("sku"),
		Name:              r.FormValue("name"),
		ProductType:       r.FormValue("type"),
		IsOpenPrice:       isOpenPrice,
		CurrentStock:      totalStock,
		InitialStockNotes: initialStockNote,
		MinStock:          int32(minStock),
		AverageCost:       avgCost,
		Units:             units,
	}

	if err := h.svc.CreateProduct(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/products")
}

func (h *Handler) HandleGenerateSKU(w http.ResponseWriter, r *http.Request) {
	catID, _ := strconv.Atoi(r.URL.Query().Get("category_id"))

	newSKU, err := h.svc.GenerateSKU(r.Context(), int32(catID))
	if err != nil {
		newSKU = "ERROR"
	}

	SKUInput(newSKU).Render(r.Context(), w)
}

func (h *Handler) HandleEditProductPage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	
	product, units, err := h.svc.GetProductForEdit(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Produk tidak ditemukan", http.StatusNotFound)
		return
	}
	categories, _ := h.repo.ListCategories(r.Context())
	
	EditProductPage(product, units, categories).Render(r.Context(), w)
}

func (h *Handler) HandleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	catID, _ := strconv.Atoi(r.FormValue("category_id"))
	isOpenPrice := r.FormValue("is_open_price") == "on"

	avgCostVal, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("average_cost")), 64)
	if err != nil || avgCostVal < 0 {
		avgCostVal = 0
	}
	avgCost := utils.FloatToNumeric(avgCostVal)

	units := parseCascadingUnits(r)
	if len(units) == 0 {
		http.Error(w, "Minimal harus ada 1 satuan produk", http.StatusBadRequest)
		return
	}

	minStock, err := strconv.Atoi(r.FormValue("min_stock"))
	if err != nil || minStock < 0 {
		minStock = 5
	}

	req := UpdateProductReq{
		ID:          int32(id),
		CategoryID:  int32(catID),
		SKU:         r.FormValue("sku"),
		Name:        r.FormValue("name"),
		ProductType: r.FormValue("type"),
		IsOpenPrice: isOpenPrice,
		MinStock:    int32(minStock),
		AverageCost: avgCost,
		Units:       units,
	}

	if err := h.svc.UpdateProduct(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/products")
}

func (h *Handler) HandleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteProduct(r.Context(), int32(id)); err != nil {
		http.Error(w, "Gagal menghapus produk: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/products")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleRecalculateStock(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RecalculateStock(r.Context()); err != nil {
		log.Println("Gagal recalculate stock:", err)
		http.Error(w, "Gagal mengupdate stok dari ledger: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.HandleListProducts(w, r)
}

func (h *Handler) HandleRecalculateStockStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming tidak didukung", http.StatusInternalServerError)
		return
	}

	err := h.svc.RecalculateStockStream(r.Context(), func(item ProductSyncItem, index, total int, newStock int32) {
		percent := int((float64(index) / float64(total)) * 100)
		payload, _ := json.Marshal(map[string]any{
			"product_name": item.Name,
			"sku":          item.SKU,
			"index":        index,
			"total":        total,
			"percent":      percent,
			"new_stock":    newStock,
		})
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()

		sleepMs := 20
		if total > 50 {
			sleepMs = 8
		}
		if total > 150 {
			sleepMs = 2
		}
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	})

	if err != nil {
		errPayload, _ := json.Marshal(map[string]any{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errPayload)
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "data: {\"done\": true}\n\n")
	flusher.Flush()
}

type PrintLabelDTO struct {
	ProductID      int32  `json:"product_id"`
	ProductName    string `json:"product_name"`
	SKU            string `json:"sku"`
	UnitName       string `json:"unit_name"`
	FormattedPrice string `json:"formatted_price"`
	StoreName      string `json:"store_name"`
	PrintDate      string `json:"print_date"`
}

func (h *Handler) HandlePrintLabelsPage(w http.ResponseWriter, r *http.Request) {
	preselectedID, _ := strconv.Atoi(r.URL.Query().Get("product_id"))

	products, err := h.repo.ListPOSProducts(r.Context())
	if err != nil {
		http.Error(w, "Gagal memuat produk: "+err.Error(), http.StatusInternalServerError)
		return
	}

	units, err := h.repo.ListPOSProductUnits(r.Context())
	if err != nil {
		http.Error(w, "Gagal memuat satuan produk: "+err.Error(), http.StatusInternalServerError)
		return
	}

	categories, err := h.repo.ListCategories(r.Context())
	if err != nil {
		http.Error(w, "Gagal memuat kategori: "+err.Error(), http.StatusInternalServerError)
		return
	}

	PrintLabelsPage(products, units, categories, int32(preselectedID)).Render(r.Context(), w)
}

func (h *Handler) HandlePrintLabelsPreview(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = r.FormValue("format")
	}
	if format != "a4" {
		format = "thermal"
	}

	company, _ := h.repo.GetCompanyConfig(r.Context())
	storeName := company.Name
	if storeName == "" {
		storeName = "Toko Ubaidillah"
	}
	printDate := time.Now().Format("02/01/2006")

	allUnits, _ := h.repo.ListPOSProductUnits(r.Context())
	unitMap := make(map[int32]repo.ListPOSProductUnitsRow)
	productUnitsMap := make(map[int32][]repo.ListPOSProductUnitsRow)
	for _, u := range allUnits {
		unitMap[u.ID] = u
		productUnitsMap[u.ProductID] = append(productUnitsMap[u.ProductID], u)
	}

	allProducts, _ := h.repo.ListPOSProducts(r.Context())
	productMap := make(map[int32]repo.ListPOSProductsRow)
	for _, p := range allProducts {
		productMap[p.ID] = p
	}

	var labels []PrintLabelDTO

	if r.Method == http.MethodPost {
		r.ParseForm()
		prodIDs := r.Form["product_ids[]"]
		unitIDs := r.Form["unit_ids[]"]
		qtys := r.Form["qtys[]"]

		for i, pIDStr := range prodIDs {
			pID, err := strconv.Atoi(pIDStr)
			if err != nil {
				continue
			}
			p, ok := productMap[int32(pID)]
			if !ok {
				continue
			}

			qty := 1
			if i < len(qtys) {
				if q, err := strconv.Atoi(qtys[i]); err == nil && q > 0 {
					qty = q
				}
			}

			unitName := "Pcs"
			formattedPrice := "0"
			var unitID int
			if i < len(unitIDs) {
				unitID, _ = strconv.Atoi(unitIDs[i])
			}

			if u, ok := unitMap[int32(unitID)]; ok {
				unitName = u.UnitName
				formattedPrice = utils.FormatRupiah(u.Price)
			} else if units, ok := productUnitsMap[int32(pID)]; ok && len(units) > 0 {
				unitName = units[len(units)-1].UnitName
				formattedPrice = utils.FormatRupiah(units[len(units)-1].Price)
			}

			sku := p.Sku
			if sku == "" {
				sku = fmt.Sprintf("PRD%06d", p.ID)
			}

			for c := 0; c < qty; c++ {
				labels = append(labels, PrintLabelDTO{
					ProductID:      p.ID,
					ProductName:    p.Name,
					SKU:            sku,
					UnitName:       unitName,
					FormattedPrice: formattedPrice,
					StoreName:      storeName,
					PrintDate:      printDate,
				})
			}
		}
	} else {
		prodID, _ := strconv.Atoi(r.URL.Query().Get("product_id"))
		unitID, _ := strconv.Atoi(r.URL.Query().Get("unit_id"))
		qty, _ := strconv.Atoi(r.URL.Query().Get("qty"))
		if qty <= 0 {
			qty = 1
		}

		if p, ok := productMap[int32(prodID)]; ok {
			unitName := "Pcs"
			formattedPrice := "0"
			if u, ok := unitMap[int32(unitID)]; ok {
				unitName = u.UnitName
				formattedPrice = utils.FormatRupiah(u.Price)
			} else if units, ok := productUnitsMap[int32(prodID)]; ok && len(units) > 0 {
				for _, u := range units {
					if u.ConversionQty == 1 {
						unitName = u.UnitName
						formattedPrice = utils.FormatRupiah(u.Price)
						break
					}
				}
				if formattedPrice == "0" {
					unitName = units[len(units)-1].UnitName
					formattedPrice = utils.FormatRupiah(units[len(units)-1].Price)
				}
			}

			sku := p.Sku
			if sku == "" {
				sku = fmt.Sprintf("PRD%06d", p.ID)
			}

			for c := 0; c < qty; c++ {
				labels = append(labels, PrintLabelDTO{
					ProductID:      p.ID,
					ProductName:    p.Name,
					SKU:            sku,
					UnitName:       unitName,
					FormattedPrice: formattedPrice,
					StoreName:      storeName,
					PrintDate:      printDate,
				})
			}
		}
	}

	PrintLabelsPreviewView(labels, format, storeName).Render(r.Context(), w)
}

func (h *Handler) HandleGetProductStockCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "ID produk tidak valid", http.StatusBadRequest)
		return
	}

	card, err := h.svc.GetProductStockCard(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Gagal memuat kartu stok produk: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

type StockAdjustmentPageData struct {
	Adjustments          []repo.ListStockAdjustmentsRow
	Products             []repo.ListPOSProductsRow
	Units                []repo.ListPOSProductUnitsRow
	Filter               StockAdjustmentFilter
	TotalRows            int64
	TotalPages           int
	PreselectedProductID int32
	PreselectedMode      string
	SuccessMsg           string
	ErrorMsg             string
}

func (h *Handler) HandleStockAdjustmentsPage(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	filter := StockAdjustmentFilter{
		Query:  r.URL.Query().Get("q"),
		Reason: r.URL.Query().Get("reason"),
		Period: r.URL.Query().Get("period"),
		From:   r.URL.Query().Get("from"),
		To:     r.URL.Query().Get("to"),
		Page:   page,
		Limit:  limit,
	}

	adjustments, total, err := h.svc.ListStockAdjustments(r.Context(), filter)
	if err != nil {
		log.Println("Error loading stock adjustments:", err)
	}

	allProducts, _ := h.repo.ListPOSProducts(r.Context())
	var goodsProducts []repo.ListPOSProductsRow
	for _, p := range allProducts {
		if p.Type == "barang" {
			goodsProducts = append(goodsProducts, p)
		}
	}
	units, _ := h.repo.ListPOSProductUnits(r.Context())

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages == 0 {
		totalPages = 1
	}

	preProdID, _ := strconv.Atoi(r.URL.Query().Get("product_id"))
	preMode := r.URL.Query().Get("mode")
	if preMode != "opname" && preMode != "pemakaian" {
		preMode = ""
	}

	successMsg := ""
	switch r.URL.Query().Get("saved") {
	case "opname":
		successMsg = "Stok opname berhasil disimpan dan stok produk serta kartu stok telah diperbarui."
	case "pemakaian":
		successMsg = "Pemakaian / pengeluaran barang berhasil dicatat dan stok produk telah dikurangi."
	}

	data := StockAdjustmentPageData{
		Adjustments:          adjustments,
		Products:             goodsProducts,
		Units:                units,
		Filter:               filter,
		TotalRows:            total,
		TotalPages:           totalPages,
		PreselectedProductID: int32(preProdID),
		PreselectedMode:      preMode,
		SuccessMsg:           successMsg,
	}

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		StockAdjustmentTable(data).Render(r.Context(), w)
		return
	}

	StockAdjustmentsPage(data).Render(r.Context(), w)
}

func (h *Handler) HandleCreateStockAdjustment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Data formulir tidak valid", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	if mode != "pemakaian" {
		mode = "opname"
	}

	prodID, _ := strconv.Atoi(r.FormValue("product_id"))
	unitID, _ := strconv.Atoi(r.FormValue("unit_id"))
	physicalStock, _ := strconv.Atoi(r.FormValue("physical_stock"))
	usageQty, _ := strconv.Atoi(r.FormValue("usage_qty"))
	reason := r.FormValue("reason")
	notes := r.FormValue("notes")

	req := CreateStockAdjustmentReq{
		Mode:          mode,
		ProductID:     int32(prodID),
		UnitID:        int32(unitID),
		PhysicalStock: int32(physicalStock),
		UsageQty:      int32(usageQty),
		Reason:        reason,
		Notes:         notes,
	}

	if _, err := h.svc.CreateStockAdjustment(r.Context(), req); err != nil {
		allProducts, _ := h.repo.ListPOSProducts(r.Context())
		var goodsProducts []repo.ListPOSProductsRow
		for _, p := range allProducts {
			if p.Type == "barang" {
				goodsProducts = append(goodsProducts, p)
			}
		}
		units, _ := h.repo.ListPOSProductUnits(r.Context())
		filter := StockAdjustmentFilter{Page: 1, Limit: 20}
		adjustments, total, _ := h.svc.ListStockAdjustments(r.Context(), filter)
		totalPages := int(math.Ceil(float64(total) / 20.0))
		if totalPages == 0 {
			totalPages = 1
		}

		w.WriteHeader(http.StatusBadRequest)
		StockAdjustmentsPage(StockAdjustmentPageData{
			Adjustments:          adjustments,
			Products:             goodsProducts,
			Units:                units,
			Filter:               filter,
			TotalRows:            total,
			TotalPages:           totalPages,
			PreselectedProductID: int32(prodID),
			PreselectedMode:      mode,
			ErrorMsg:             err.Error(),
		}).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/stock-adjustments?saved="+mode, http.StatusSeeOther)
}

