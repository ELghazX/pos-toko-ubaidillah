-- +goose Up
CREATE TABLE purchases (
	id SERIAL PRIMARY KEY,
	invoice_number VARCHAR(50) NOT NULL UNIQUE,
	supplier_id INT REFERENCES suppliers(id) ON DELETE SET NULL,
	payment_method VARCHAR(30) NOT NULL DEFAULT 'tunai',
	total_cost NUMERIC(15,2) NOT NULL DEFAULT 0,
	status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending' (PO), 'diterima' (Goods Receipt), 'batal' (Void)
	notes TEXT NOT NULL DEFAULT '',
	date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	received_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE purchase_details (
	id SERIAL PRIMARY KEY,
	purchase_id INT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
	product_id INT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
	unit_id INT REFERENCES product_units(id) ON DELETE SET NULL,
	unit_name VARCHAR(50) NOT NULL,
	conversion_qty INT NOT NULL DEFAULT 1,
	qty INT NOT NULL,
	cost NUMERIC(15,2) NOT NULL DEFAULT 0,
	subtotal NUMERIC(15,2) NOT NULL DEFAULT 0
);

CREATE TABLE inventory_ledgers (
	id SERIAL PRIMARY KEY,
	product_id INT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	transaction_type VARCHAR(30) NOT NULL, -- 'pembelian', 'penjualan', 'pemakaian', 'opname', 'retur_batal'
	reference_id INT,
	qty_change INT NOT NULL,
	balance_after INT NOT NULL DEFAULT 0,
	notes TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS inventory_ledgers;
DROP TABLE IF EXISTS purchase_details;
DROP TABLE IF EXISTS purchases;
