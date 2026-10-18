-- name: ListCategories :many
SELECT * FROM categories
WHERE is_active = true
ORDER BY name ASC;

-- name: GetCategoryByID :one
SELECT * FROM categories
WHERE id = $1 LIMIT 1;

-- name: GetCategoryByCode :one
SELECT * FROM categories
WHERE UPPER(code) = UPPER($1::text) LIMIT 1;

-- name: GetCategoryByName :one
SELECT * FROM categories
WHERE LOWER(name) = LOWER($1::text) AND is_active = true LIMIT 1;


-- name: CreateCategory :one
INSERT INTO categories (
	code, name
) VALUES (
	$1, $2
) RETURNING *;


-- name: UpdateCategory :exec
UPDATE categories
SET code = $2, name = $3, updated_at = CURRENT_TIMESTAMP
WHERE id  = $1;

-- name: DeleteCategory :exec
UPDATE categories
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;


-- name: CreateProduct :one
INSERT INTO products (
category_id, sku, name, type, is_open_price, current_stock, average_cost, min_stock
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8
) RETURNING id;

-- name: CreateProductUnit :exec
INSERT INTO product_units (
	product_id, unit_name, conversion_qty, price
) VALUES (
	$1, $2, $3, $4
);

-- name: SearchProducts :many
SELECT 
	p.id AS product_id,
	c.name AS category_name,
	p.sku,
	p.name AS product_name,
	p.type,
	p.current_stock,
	p.min_stock,
	p.average_cost,
	pu.unit_name,
	pu.conversion_qty,
	pu.price
FROM products p
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN product_units pu ON p.id = pu.product_id AND pu.conversion_qty = 1 AND pu.is_active = true
WHERE p.is_active = true
  AND (p.name ILIKE $1 OR p.sku ILIKE $1)
  AND ($2::int = 0 OR p.category_id = $2::int)
  AND ($3::text = '' OR p.type = $3::text)
  AND (
    $4::text = '' OR
    ($4::text = 'low_stock' AND p.type = 'barang' AND p.current_stock <= p.min_stock) OR
    ($4::text = 'out_of_stock' AND p.type = 'barang' AND p.current_stock <= 0) OR
    ($4::text = 'in_stock' AND (p.type != 'barang' OR p.current_stock > p.min_stock)) OR
    ($4::text = 'no_hpp' AND p.type = 'barang' AND COALESCE(p.average_cost, 0) <= 0)
  )
ORDER BY
  CASE WHEN $5::text = 'name_asc' THEN LOWER(p.name) END ASC,
  CASE WHEN $5::text = 'name_desc' THEN LOWER(p.name) END DESC,
  CASE WHEN $5::text = 'stock_asc' THEN p.current_stock END ASC,
  CASE WHEN $5::text = 'stock_desc' THEN p.current_stock END DESC,
  CASE WHEN $5::text = 'price_asc' THEN COALESCE(pu.price, 0) END ASC,
  CASE WHEN $5::text = 'price_desc' THEN COALESCE(pu.price, 0) END DESC,
  p.id DESC
LIMIT $6 OFFSET $7;

-- name: CountProducts :one
SELECT COUNT(*) 
FROM products p
WHERE p.is_active = true
  AND (p.name ILIKE $1 OR p.sku ILIKE $1)
  AND ($2::int = 0 OR p.category_id = $2::int)
  AND ($3::text = '' OR p.type = $3::text)
  AND (
    $4::text = '' OR
    ($4::text = 'low_stock' AND p.type = 'barang' AND p.current_stock <= p.min_stock) OR
    ($4::text = 'out_of_stock' AND p.type = 'barang' AND p.current_stock <= 0) OR
    ($4::text = 'in_stock' AND (p.type != 'barang' OR p.current_stock > p.min_stock)) OR
    ($4::text = 'no_hpp' AND p.type = 'barang' AND COALESCE(p.average_cost, 0) <= 0)
  );

-- name: CountLowStockProducts :one
SELECT COUNT(*)
FROM products
WHERE is_active = true
  AND type = 'barang'
  AND current_stock <= min_stock;

-- name: GetProductByID :one
SELECT * FROM products
WHERE id = $1 AND is_active = true LIMIT 1;

-- name: GetProductBySKU :one
SELECT * FROM products
WHERE sku = $1 AND is_active = true LIMIT 1;

-- name: GetLastProductByCategory :one
SELECT sku FROM products 
WHERE category_id = $1 AND sku LIKE $2 
ORDER BY id DESC LIMIT 1;

-- name: UpdateProduct :exec
UPDATE products
SET 
	category_id = $2,
	sku = $3,
	name = $4,
	type = $5,
	is_open_price = $6,
	min_stock = $7,
	average_cost = $8,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: BackfillZeroCogsByProductID :exec
UPDATE sale_details
SET cogs = ($2::numeric * GREATEST(conversion_qty, 1))::numeric(15,2)
WHERE product_id = $1 AND cogs = 0;

-- name: GetProductUnits :many
SELECT * FROM product_units
WHERE product_id = $1 AND is_active = true
ORDER BY conversion_qty DESC;

-- name: DeactivateProductUnitsByProductID :exec
UPDATE product_units
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE product_id = $1;

-- name: UpdateExistingProductUnit :one
UPDATE product_units
SET 
	unit_name = $2,
	conversion_qty = $3,
	price = $4,
	is_active = true,
	updated_at = CURRENT_TIMESTAMP
WHERE id = (
	SELECT pu.id FROM product_units pu
	WHERE pu.product_id = $1 AND LOWER(pu.unit_name) = LOWER($2)
	LIMIT 1
)
RETURNING id;

-- name: DeleteProduct :exec
UPDATE products
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: ListSuppliers :many
SELECT * FROM suppliers
WHERE is_active = true
ORDER BY name ASC;

-- name: GetSupplierByID :one
SELECT * FROM suppliers
WHERE id = $1 AND is_active = true
LIMIT 1;

-- name: CreateSupplier :one
INSERT INTO suppliers (
	name, phone, address
) VALUES (
	$1, $2, $3
) RETURNING *;

-- name: UpdateSupplier :exec
UPDATE suppliers
SET name = $2, phone = $3, address = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: DeleteSupplier :exec
UPDATE suppliers
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: ListActiveGoodsProducts :many
SELECT * FROM products
WHERE is_active = true AND type = 'barang'
ORDER BY name ASC;

-- name: ListAllActiveProductUnitsWithLastCost :many
SELECT 
	pu.id,
	pu.product_id,
	pu.unit_name,
	pu.conversion_qty,
	COALESCE(
		(
			SELECT pd.cost 
			FROM purchase_details pd
			JOIN purchases pur ON pd.purchase_id = pur.id
			WHERE pd.product_id = pu.product_id 
			  AND LOWER(pd.unit_name) = LOWER(pu.unit_name)
			  AND pur.status != 'batal'
			ORDER BY pd.id DESC
			LIMIT 1
		),
		(p.average_cost * pu.conversion_qty)
	)::NUMERIC(15,2) AS last_cost
FROM product_units pu
JOIN products p ON pu.product_id = p.id
WHERE pu.is_active = true AND p.is_active = true AND p.type = 'barang'
ORDER BY pu.product_id ASC, pu.conversion_qty DESC;

-- name: CreatePurchase :one
INSERT INTO purchases (
	invoice_number, supplier_id, payment_method, total_cost, status, notes, received_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7
) RETURNING id;

-- name: CreatePurchaseDetail :exec
INSERT INTO purchase_details (
	purchase_id, product_id, unit_id, unit_name, conversion_qty, qty, cost, subtotal
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8
);

-- name: UpdatePurchaseDetailOnReceive :exec
UPDATE purchase_details
SET qty = $2, cost = $3, subtotal = $4
WHERE id = $1;

-- name: DeletePurchaseDetailByID :exec
DELETE FROM purchase_details
WHERE id = $1;

-- name: UpdatePurchaseOnReceive :exec
UPDATE purchases
SET 
	status = 'diterima',
	payment_method = $2,
	total_cost = $3,
	notes = $4,
	received_at = CURRENT_TIMESTAMP,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: CancelPurchaseOrder :exec
UPDATE purchases
SET status = 'batal', updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'pending';

-- name: VoidGoodsReceipt :exec
UPDATE purchases
SET status = 'batal', updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'diterima';

-- name: CancelPendingBackordersByPrefix :exec
UPDATE purchases
SET status = 'batal', updated_at = CURRENT_TIMESTAMP
WHERE invoice_number LIKE $1 AND status = 'pending';

-- name: ReopenPurchaseOrder :exec
UPDATE purchases
SET status = 'pending', received_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'batal';


-- name: ListPurchases :many
SELECT 
	pu.id,
	pu.invoice_number,
	pu.supplier_id,
	COALESCE(s.name, 'Supplier Umum') AS supplier_name,
	pu.payment_method,
	pu.total_cost,
	pu.status,
	pu.notes,
	pu.date,
	pu.received_at
FROM purchases pu
LEFT JOIN suppliers s ON pu.supplier_id = s.id
ORDER BY 
	CASE WHEN pu.status = 'pending' THEN 0 ELSE 1 END ASC,
	pu.id DESC;

-- name: GetPurchaseByID :one
SELECT 
	pu.id,
	pu.invoice_number,
	pu.supplier_id,
	COALESCE(s.name, 'Supplier Umum') AS supplier_name,
	COALESCE(s.phone, '') AS supplier_phone,
	COALESCE(s.address, '') AS supplier_address,
	pu.payment_method,
	pu.total_cost,
	pu.status,
	pu.notes,
	pu.date,
	pu.received_at
FROM purchases pu
LEFT JOIN suppliers s ON pu.supplier_id = s.id
WHERE pu.id = $1
LIMIT 1;

-- name: GetPurchaseDetails :many
SELECT 
	pd.id,
	pd.product_id,
	p.sku,
	p.name AS product_name,
	pd.unit_id,
	pd.unit_name,
	pd.conversion_qty,
	pd.qty,
	pd.cost,
	pd.subtotal
FROM purchase_details pd
JOIN products p ON pd.product_id = p.id
WHERE pd.purchase_id = $1
ORDER BY pd.id ASC;

-- name: UpdateProductStockAndCost :exec
UPDATE products
SET 
	current_stock = $2,
	average_cost = $3,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: CreateInventoryLedger :exec
INSERT INTO inventory_ledgers (
	product_id, transaction_type, reference_id, qty_change, balance_after, notes
) VALUES (
	$1, $2, $3, $4, $5, $6
);

-- name: ListCustomers :many
SELECT * FROM customers
WHERE is_active = true
ORDER BY name ASC;

-- name: GetCustomerByID :one
SELECT * FROM customers
WHERE id = $1 AND is_active = true
LIMIT 1;

-- name: GetCustomerByIDAnyStatus :one
SELECT * FROM customers
WHERE id = $1
LIMIT 1;

-- name: GetCustomerUnpaidDebtTotal :one
SELECT COALESCE(SUM(total_amount - paid_amount), 0)::NUMERIC(15,2) AS remaining_debt
FROM sales
WHERE customer_id = $1 AND status != 'batal' AND payment_status = 'kasbon';

-- name: CreateCustomer :one
INSERT INTO customers (
	name, phone, address
) VALUES (
	$1, $2, $3
) RETURNING *;

-- name: UpdateCustomer :exec
UPDATE customers
SET name = $2, phone = $3, address = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: DeleteCustomer :exec
UPDATE customers
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: RestoreCustomer :exec
UPDATE customers
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: ListCustomersWithFilter :many
SELECT * FROM customers
WHERE (
    ($1::text = 'active' AND is_active = true) OR
    ($1::text = 'inactive' AND is_active = false) OR
    ($1::text = 'all')
)
ORDER BY name ASC;

-- name: ListPOSProducts :many
SELECT 
	p.id,
	p.category_id,
	c.name AS category_name,
	p.sku,
	p.name,
	p.type,
	p.is_open_price,
	p.current_stock,
	p.average_cost,
	p.min_stock
FROM products p
JOIN categories c ON p.category_id = c.id
LEFT JOIN (
    SELECT 
        sd.product_id,
        COUNT(sd.id) AS sales_count,
        SUM(sd.qty * sd.conversion_qty) AS total_qty
    FROM sale_details sd
    JOIN sales s ON sd.sale_id = s.id
    WHERE s.status != 'batal'
    GROUP BY sd.product_id
) stats ON p.id = stats.product_id
WHERE p.is_active = true
ORDER BY COALESCE(stats.sales_count, 0) DESC, COALESCE(stats.total_qty, 0) DESC, p.name ASC;

-- name: ListPOSProductUnits :many
SELECT 
	pu.id,
	pu.product_id,
	pu.unit_name,
	pu.conversion_qty,
	pu.price
FROM product_units pu
JOIN products p ON pu.product_id = p.id
WHERE pu.is_active = true AND p.is_active = true
ORDER BY pu.product_id ASC, pu.conversion_qty ASC;

-- name: CreateSale :one
INSERT INTO sales (
	invoice_no, customer_id, date, total_amount, discount_amount, paid_amount, change_amount, payment_method, payment_status, due_date, status, notes
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING id;

-- name: CreateSaleDetail :exec
INSERT INTO sale_details (
	sale_id, product_id, unit_id, unit_name, conversion_qty, qty, price, discount, subtotal, cogs, is_open_price
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: ListSales :many
SELECT 
	s.id,
	s.invoice_no,
	s.customer_id,
	COALESCE(c.name, 'Pelanggan Umum') AS customer_name,
	s.date,
	s.total_amount,
	s.discount_amount,
	s.paid_amount,
	s.change_amount,
	s.payment_method,
	s.payment_status,
	s.due_date,
	s.status,
	s.notes,
	s.created_at
FROM sales s
LEFT JOIN customers c ON s.customer_id = c.id
ORDER BY s.id DESC;

-- name: GetSaleByID :one
SELECT 
	s.id,
	s.invoice_no,
	s.customer_id,
	COALESCE(c.name, 'Pelanggan Umum') AS customer_name,
	COALESCE(c.phone, '') AS customer_phone,
	COALESCE(c.address, '') AS customer_address,
	s.date,
	s.total_amount,
	s.discount_amount,
	s.paid_amount,
	s.change_amount,
	s.payment_method,
	s.payment_status,
	s.due_date,
	s.status,
	s.notes,
	s.created_at
FROM sales s
LEFT JOIN customers c ON s.customer_id = c.id
WHERE s.id = $1
LIMIT 1;

-- name: GetSaleDetails :many
SELECT 
	sd.id,
	sd.sale_id,
	sd.product_id,
	p.sku,
	p.name AS product_name,
	p.type AS product_type,
	(p.is_open_price OR sd.is_open_price) AS is_open_price,
	sd.unit_id,
	sd.unit_name,
	sd.conversion_qty,
	sd.qty,
	sd.price,
	sd.discount,
	sd.subtotal,
	sd.cogs
FROM sale_details sd
JOIN products p ON sd.product_id = p.id
WHERE sd.sale_id = $1
ORDER BY sd.id ASC;

-- name: VoidSale :exec
UPDATE sales
SET status = 'batal', updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: GetCompanyConfig :one
SELECT id, name, tagline, phone, address, receipt_footer, updated_at
FROM company_configs
WHERE id = 1
LIMIT 1;

-- name: UpdateCompanyConfig :exec
UPDATE company_configs
SET 
	name = $1,
	tagline = $2,
	phone = $3,
	address = $4,
	receipt_footer = $5,
	updated_at = CURRENT_TIMESTAMP
WHERE id = 1;

-- name: CreateStockAdjustment :one
INSERT INTO stock_adjustments (
    adjustment_no, product_id, system_stock, physical_stock, difference_qty, reason, notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: ListStockAdjustments :many
SELECT 
    sa.id,
    sa.adjustment_no,
    sa.date,
    sa.product_id,
    p.sku AS product_sku,
    p.name AS product_name,
    c.name AS category_name,
    pu.unit_name AS base_unit_name,
    sa.system_stock,
    sa.physical_stock,
    sa.difference_qty,
    sa.reason,
    sa.notes,
    sa.created_at
FROM stock_adjustments sa
JOIN products p ON sa.product_id = p.id
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN product_units pu ON p.id = pu.product_id AND pu.conversion_qty = 1 AND pu.is_active = true
WHERE ($1::text = '' OR p.name ILIKE $1 OR p.sku ILIKE $1 OR sa.adjustment_no ILIKE $1)
  AND ($2::text = '' OR sa.reason = $2::text)
  AND ($3::timestamptz IS NULL OR sa.date >= $3::timestamptz)
  AND ($4::timestamptz IS NULL OR sa.date <= $4::timestamptz)
ORDER BY sa.id DESC
LIMIT $5 OFFSET $6;

-- name: CountStockAdjustments :one
SELECT COUNT(*)
FROM stock_adjustments sa
JOIN products p ON sa.product_id = p.id
WHERE ($1::text = '' OR p.name ILIKE $1 OR p.sku ILIKE $1 OR sa.adjustment_no ILIKE $1)
  AND ($2::text = '' OR sa.reason = $2::text)
  AND ($3::timestamptz IS NULL OR sa.date >= $3::timestamptz)
  AND ($4::timestamptz IS NULL OR sa.date <= $4::timestamptz);

-- name: GetStockAdjustmentByID :one
SELECT 
    sa.id,
    sa.adjustment_no,
    sa.date,
    sa.product_id,
    p.sku AS product_sku,
    p.name AS product_name,
    c.name AS category_name,
    pu.unit_name AS base_unit_name,
    sa.system_stock,
    sa.physical_stock,
    sa.difference_qty,
    sa.reason,
    sa.notes,
    sa.created_at
FROM stock_adjustments sa
JOIN products p ON sa.product_id = p.id
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN product_units pu ON p.id = pu.product_id AND pu.conversion_qty = 1 AND pu.is_active = true
WHERE sa.id = $1 LIMIT 1;

-- name: GetLastStockAdjustmentNo :one
SELECT adjustment_no FROM stock_adjustments
WHERE adjustment_no LIKE $1
ORDER BY id DESC LIMIT 1;

-- name: GetTodaySalesMetrics :one
SELECT 
    COALESCE(SUM(s.total_amount), 0)::NUMERIC(15,2) AS total_sales,
    COALESCE(SUM(s.discount_amount), 0)::NUMERIC(15,2) AS total_discount,
    COALESCE(COUNT(s.id), 0)::BIGINT AS total_transactions,
    COALESCE(SUM(
        (SELECT COALESCE(SUM(sd.qty * sd.cogs), 0) FROM sale_details sd WHERE sd.sale_id = s.id)
    ), 0)::NUMERIC(15,2) AS total_cogs
FROM sales s
WHERE s.status != 'batal'
  AND DATE(s.date) = CURRENT_DATE;

-- name: GetTodayReceivablesMetrics :one
SELECT 
    COALESCE(SUM(total_amount - paid_amount), 0)::NUMERIC(15,2) AS total_unpaid_debt,
    COALESCE(COUNT(DISTINCT customer_id), 0)::BIGINT AS customers_count,
    COALESCE(SUM(
        CASE WHEN due_date IS NOT NULL AND due_date < CURRENT_DATE 
             THEN (total_amount - paid_amount) 
             ELSE 0 
        END
    ), 0)::NUMERIC(15,2) AS overdue_debt
FROM sales
WHERE status != 'batal'
  AND payment_status = 'kasbon';

-- name: GetWeeklySalesTrend :many
WITH last_7_days AS (
    SELECT generate_series(
        CURRENT_DATE - INTERVAL '6 days',
        CURRENT_DATE,
        INTERVAL '1 day'
    )::DATE AS day_date
)
SELECT 
    d.day_date,
    TO_CHAR(d.day_date, 'Dy, DD/MM') AS day_label,
    COALESCE(SUM(s.total_amount), 0)::NUMERIC(15,2) AS total_sales,
    COALESCE(COUNT(s.id), 0)::BIGINT AS total_transactions,
    COALESCE(SUM(
        (SELECT COALESCE(SUM(sd.qty * (sd.price - sd.cogs)), 0) FROM sale_details sd WHERE sd.sale_id = s.id)
    ), 0)::NUMERIC(15,2) AS gross_profit
FROM last_7_days d
LEFT JOIN sales s ON DATE(s.date) = d.day_date AND s.status != 'batal'
GROUP BY d.day_date
ORDER BY d.day_date ASC;

-- name: GetTopProductsTodayOrWeek :many
SELECT 
    p.id AS product_id,
    p.name AS product_name,
    p.sku AS product_sku,
    COALESCE(c.name, 'Umum') AS category_name,
    COALESCE(SUM(sd.qty), 0)::BIGINT AS total_qty_sold,
    COALESCE(SUM(sd.subtotal), 0)::NUMERIC(15,2) AS total_revenue
FROM sale_details sd
JOIN sales s ON sd.sale_id = s.id
JOIN products p ON sd.product_id = p.id
LEFT JOIN categories c ON p.category_id = c.id
WHERE s.status != 'batal'
  AND s.date >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY p.id, p.name, p.sku, c.name
ORDER BY total_qty_sold DESC, total_revenue DESC
LIMIT 5;

-- name: GetRecentDashboardSales :many
SELECT 
    s.id,
    s.invoice_no,
    COALESCE(c.name, 'Pelanggan Umum') AS customer_name,
    s.date,
    s.total_amount,
    s.paid_amount,
    s.payment_method,
    s.payment_status,
    s.status
FROM sales s
LEFT JOIN customers c ON s.customer_id = c.id
WHERE s.status != 'batal'
ORDER BY s.id DESC
LIMIT 7;

-- name: GetDashboardLowStockProducts :many
SELECT 
    p.id,
    p.name,
    p.sku,
    COALESCE(c.name, 'Umum') AS category_name,
    p.current_stock,
    p.min_stock,
    COALESCE(pu.unit_name, 'Pcs') AS unit_name
FROM products p
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN product_units pu ON p.id = pu.product_id AND pu.conversion_qty = 1 AND pu.is_active = true
WHERE p.is_active = true 
  AND p.type = 'barang' 
  AND p.current_stock <= p.min_stock
ORDER BY (p.current_stock - p.min_stock) ASC, p.current_stock ASC
LIMIT 5;

-- name: GetDebtsSummaryMetrics :one
SELECT 
    COALESCE(SUM(total_amount - paid_amount), 0)::NUMERIC(15,2) AS total_outstanding,
    COALESCE(COUNT(DISTINCT customer_id), 0)::BIGINT AS total_customers_in_debt,
    COALESCE(SUM(
        CASE WHEN due_date IS NOT NULL AND due_date < CURRENT_DATE 
             THEN (total_amount - paid_amount) 
             ELSE 0 
        END
    ), 0)::NUMERIC(15,2) AS total_overdue,
    COALESCE((
        SELECT SUM(dp.amount_paid)
        FROM debt_payments dp
        WHERE EXTRACT(MONTH FROM dp.date) = EXTRACT(MONTH FROM CURRENT_DATE)
          AND EXTRACT(YEAR FROM dp.date) = EXTRACT(YEAR FROM CURRENT_DATE)
    ), 0)::NUMERIC(15,2) AS total_settled_this_month
FROM sales
WHERE status != 'batal'
  AND payment_status = 'kasbon';

-- name: ListCustomerDebtRecap :many
SELECT 
    c.id AS customer_id,
    c.name AS customer_name,
    COALESCE(c.phone, '') AS customer_phone,
    COALESCE(c.address, '') AS customer_address,
    COALESCE(c.is_active, true)::BOOLEAN AS is_active,
    COUNT(s.id)::BIGINT AS total_invoices,
    SUM(s.total_amount)::NUMERIC(15,2) AS total_debt_amount,
    SUM(s.paid_amount)::NUMERIC(15,2) AS total_paid_amount,
    SUM(s.total_amount - s.paid_amount)::NUMERIC(15,2) AS remaining_debt,
    MIN(s.due_date)::DATE AS earliest_due_date,
    MAX(s.date)::TIMESTAMPTZ AS latest_sale_date,
    BOOL_OR(s.due_date IS NOT NULL AND s.due_date < CURRENT_DATE) AS has_overdue
FROM sales s
JOIN customers c ON s.customer_id = c.id
WHERE s.status != 'batal'
  AND s.payment_status = 'kasbon'
  AND ($1::text = '' OR c.name ILIKE $1 OR c.phone ILIKE $1)
GROUP BY c.id, c.name, c.phone, c.address, c.is_active
HAVING SUM(s.total_amount - s.paid_amount) > 0
ORDER BY 
    CASE WHEN $2 = 'overdue' THEN (CASE WHEN BOOL_OR(s.due_date IS NOT NULL AND s.due_date < CURRENT_DATE) THEN 0 ELSE 1 END) END ASC,
    CASE WHEN $2 = 'debt_desc' THEN SUM(s.total_amount - s.paid_amount) END DESC,
    CASE WHEN $2 = 'debt_asc' THEN SUM(s.total_amount - s.paid_amount) END ASC,
    c.name ASC;

-- name: ListDebtInvoices :many
SELECT 
    s.id AS sale_id,
    s.invoice_no,
    s.customer_id,
    COALESCE(c.name, 'Pelanggan Umum') AS customer_name,
    COALESCE(c.phone, '') AS customer_phone,
    COALESCE(c.is_active, true)::BOOLEAN AS is_customer_active,
    s.date,
    s.due_date,
    s.total_amount,
    s.paid_amount,
    (s.total_amount - s.paid_amount)::NUMERIC(15,2) AS remaining_amount,
    s.payment_status,
    s.status,
    s.notes,
    (s.due_date IS NOT NULL AND s.due_date < CURRENT_DATE AND s.payment_status = 'kasbon') AS is_overdue
FROM sales s
LEFT JOIN customers c ON s.customer_id = c.id
WHERE s.status != 'batal'
  AND ($1::text = '' OR s.invoice_no ILIKE $1 OR c.name ILIKE $1 OR c.phone ILIKE $1)
  AND (
      ($2::text = 'kasbon' AND s.payment_status = 'kasbon') OR
      ($2::text = 'lunas' AND s.payment_status = 'lunas') OR
      ($2::text = 'overdue' AND s.payment_status = 'kasbon' AND s.due_date IS NOT NULL AND s.due_date < CURRENT_DATE) OR
      ($2::text = '' OR $2::text = 'all')
  )
ORDER BY 
    CASE WHEN s.payment_status = 'kasbon' THEN 0 ELSE 1 END,
    s.id DESC
LIMIT $3 OFFSET $4;

-- name: CountDebtInvoices :one
SELECT COUNT(*)
FROM sales s
LEFT JOIN customers c ON s.customer_id = c.id
WHERE s.status != 'batal'
  AND ($1::text = '' OR s.invoice_no ILIKE $1 OR c.name ILIKE $1 OR c.phone ILIKE $1)
  AND (
      ($2::text = 'kasbon' AND s.payment_status = 'kasbon') OR
      ($2::text = 'lunas' AND s.payment_status = 'lunas') OR
      ($2::text = 'overdue' AND s.payment_status = 'kasbon' AND s.due_date IS NOT NULL AND s.due_date < CURRENT_DATE) OR
      ($2::text = '' OR $2::text = 'all')
  );

-- name: ListCustomerUnpaidInvoices :many
SELECT 
    s.id AS sale_id,
    s.invoice_no,
    s.date,
    s.due_date,
    s.total_amount,
    s.paid_amount,
    (s.total_amount - s.paid_amount)::NUMERIC(15,2) AS remaining_amount,
    s.payment_status,
    s.notes,
    (s.due_date IS NOT NULL AND s.due_date < CURRENT_DATE) AS is_overdue
FROM sales s
WHERE s.customer_id = $1
  AND s.status != 'batal'
  AND s.payment_status = 'kasbon'
ORDER BY s.id ASC;

-- name: ListDebtPaymentsByCustomerID :many
SELECT 
    dp.id,
    dp.sale_id,
    s.invoice_no,
    dp.date,
    dp.amount_paid,
    dp.notes,
    dp.created_at
FROM debt_payments dp
JOIN sales s ON dp.sale_id = s.id
WHERE s.customer_id = $1
ORDER BY dp.id DESC
LIMIT 50;

-- name: GetSalesReportDaily :many
SELECT 
    DATE(s.date)::DATE AS sale_date,
    COUNT(s.id)::BIGINT AS transaction_count,
    COALESCE(SUM(s.total_amount), 0)::NUMERIC(15,2) AS total_gross_sales,
    COALESCE(SUM(s.discount_amount), 0)::NUMERIC(15,2) AS total_discounts,
    COALESCE(SUM(s.total_amount), 0)::NUMERIC(15,2) AS total_net_sales,
    COALESCE(SUM((SELECT COALESCE(SUM(sd.qty * sd.cogs), 0) FROM sale_details sd WHERE sd.sale_id = s.id)), 0)::NUMERIC(15,2) AS total_cogs,
    COALESCE(SUM((SELECT COALESCE(SUM(sd.qty * (sd.price - sd.cogs)), 0) FROM sale_details sd WHERE sd.sale_id = s.id) - s.discount_amount), 0)::NUMERIC(15,2) AS gross_profit
FROM sales s
WHERE s.status != 'batal'
  AND s.date >= $1::timestamptz AND s.date <= $2::timestamptz
GROUP BY DATE(s.date)
ORDER BY DATE(s.date) DESC;

-- name: GetSalesReportPaymentMethods :many
SELECT 
    s.payment_method,
    COUNT(s.id)::BIGINT AS transaction_count,
    COALESCE(SUM(s.total_amount), 0)::NUMERIC(15,2) AS total_amount
FROM sales s
WHERE s.status != 'batal'
  AND s.date >= $1::timestamptz AND s.date <= $2::timestamptz
GROUP BY s.payment_method
ORDER BY total_amount DESC;

-- name: GetSalesDetailReport :many
SELECT 
    s.id AS sale_id,
    s.invoice_no,
    s.date AS sale_date,
    COALESCE(c.name, 'Pelanggan Umum') AS customer_name,
    p.id AS product_id,
    p.sku AS product_sku,
    p.name AS product_name,
    COALESCE(cat.name, 'Umum') AS category_name,
    sd.unit_name,
    sd.qty,
    sd.price,
    sd.discount,
    sd.subtotal,
    sd.cogs,
    (sd.subtotal - (sd.qty * sd.cogs))::NUMERIC(15,2) AS item_profit
FROM sale_details sd
JOIN sales s ON sd.sale_id = s.id
JOIN products p ON sd.product_id = p.id
LEFT JOIN categories cat ON p.category_id = cat.id
LEFT JOIN customers c ON s.customer_id = c.id
WHERE s.status != 'batal'
  AND s.date >= $1::timestamptz AND s.date <= $2::timestamptz
  AND ($3::text = '' OR p.name ILIKE $3 OR p.sku ILIKE $3 OR s.invoice_no ILIKE $3)
  AND ($4::int = 0 OR p.category_id = $4)
ORDER BY s.date DESC, sd.id ASC
LIMIT $5 OFFSET $6;

-- name: CountSalesDetailReport :one
SELECT COUNT(*)
FROM sale_details sd
JOIN sales s ON sd.sale_id = s.id
JOIN products p ON sd.product_id = p.id
WHERE s.status != 'batal'
  AND s.date >= $1::timestamptz AND s.date <= $2::timestamptz
  AND ($3::text = '' OR p.name ILIKE $3 OR p.sku ILIKE $3 OR s.invoice_no ILIKE $3)
  AND ($4::int = 0 OR p.category_id = $4);

-- name: GetProfitLossReport :one
SELECT 
    COALESCE(SUM(s.total_amount), 0)::NUMERIC(15,2) AS gross_sales,
    COALESCE(SUM(s.discount_amount), 0)::NUMERIC(15,2) AS total_discounts,
    COALESCE(COUNT(s.id), 0)::BIGINT AS total_transactions,
    COALESCE(SUM((SELECT COALESCE(SUM(sd.qty * sd.cogs), 0) FROM sale_details sd WHERE sd.sale_id = s.id)), 0)::NUMERIC(15,2) AS total_cogs,
    COALESCE(SUM((SELECT COALESCE(SUM(sd.qty), 0) FROM sale_details sd WHERE sd.sale_id = s.id)), 0)::BIGINT AS total_items_sold
FROM sales s
WHERE s.status != 'batal'
  AND s.date >= $1::timestamptz AND s.date <= $2::timestamptz;

-- name: GetProductOpeningStock :one
SELECT COALESCE(
    (SELECT balance_after 
     FROM inventory_ledgers 
     WHERE product_id = $1 AND date < $2::timestamptz 
     ORDER BY id DESC LIMIT 1),
    0
)::INT AS opening_stock;

-- name: GetProductStockLedger :many
SELECT 
    il.id,
    il.date,
    il.transaction_type,
    il.reference_id,
    il.qty_change,
    il.balance_after,
    il.notes
FROM inventory_ledgers il
WHERE il.product_id = $1
  AND il.date >= $2::timestamptz AND il.date <= $3::timestamptz
ORDER BY il.id ASC;

-- name: ListProductStockLedger :many
SELECT 
    il.id,
    il.date,
    il.transaction_type,
    il.reference_id,
    il.qty_change,
    il.balance_after,
    il.notes
FROM inventory_ledgers il
WHERE il.product_id = $1
ORDER BY il.date DESC, il.id DESC
LIMIT 200;

-- name: GetNonMovingStockReport :many
SELECT 
    p.id AS product_id,
    p.sku AS product_sku,
    p.name AS product_name,
    COALESCE(c.name, 'Umum') AS category_name,
    p.current_stock,
    COALESCE(pu.unit_name, 'Pcs') AS unit_name,
    p.average_cost,
    (p.current_stock * p.average_cost)::NUMERIC(15,2) AS capital_tied_up,
    MAX(s.date)::TIMESTAMPTZ AS last_sold_date,
    COALESCE(
        EXTRACT(DAY FROM CURRENT_TIMESTAMP - MAX(s.date)),
        EXTRACT(DAY FROM CURRENT_TIMESTAMP - p.created_at)
    )::BIGINT AS days_inactive
FROM products p
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN product_units pu ON p.id = pu.product_id AND pu.conversion_qty = 1 AND pu.is_active = true
LEFT JOIN sale_details sd ON sd.product_id = p.id
LEFT JOIN sales s ON sd.sale_id = s.id AND s.status != 'batal'
WHERE p.is_active = true 
  AND p.type = 'barang'
  AND p.current_stock > 0
GROUP BY p.id, p.sku, p.name, c.name, pu.unit_name, p.current_stock, p.average_cost, p.created_at
HAVING MAX(s.date) IS NULL OR MAX(s.date) <= $1::timestamptz
ORDER BY capital_tied_up DESC, days_inactive DESC;

-- name: GetStockValuationReport :many
SELECT 
    p.id AS product_id,
    p.sku AS product_sku,
    p.name AS product_name,
    COALESCE(c.name, 'Umum') AS category_name,
    p.current_stock,
    COALESCE(pu.unit_name, 'Pcs') AS unit_name,
    p.average_cost,
    COALESCE(pu.price, 0)::NUMERIC(15,2) AS retail_price,
    (p.current_stock * p.average_cost)::NUMERIC(15,2) AS total_cost_value,
    (p.current_stock * COALESCE(pu.price, 0))::NUMERIC(15,2) AS total_retail_value,
    ((p.current_stock * COALESCE(pu.price, 0)) - (p.current_stock * p.average_cost))::NUMERIC(15,2) AS potential_profit
FROM products p
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN product_units pu ON p.id = pu.product_id AND pu.conversion_qty = 1 AND pu.is_active = true
WHERE p.is_active = true 
  AND p.type = 'barang'
ORDER BY total_cost_value DESC;

-- name: GetPurchasesReport :many
SELECT 
    p.id AS purchase_id,
    p.invoice_number,
    p.received_at,
    COALESCE(s.name, 'Supplier Umum') AS supplier_name,
    p.payment_method,
    p.total_cost,
    p.status,
    p.notes
FROM purchases p
LEFT JOIN suppliers s ON p.supplier_id = s.id
WHERE p.status != 'batal'
  AND p.received_at >= $1::timestamptz AND p.received_at <= $2::timestamptz
ORDER BY p.received_at DESC;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE LOWER(username) = LOWER($1)
LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY role ASC, username ASC;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :one
INSERT INTO users (
    username, name, password_hash, role, is_active
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdateUser :exec
UPDATE users
SET username = $2,
    name = $3,
    role = $4,
    is_active = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;
