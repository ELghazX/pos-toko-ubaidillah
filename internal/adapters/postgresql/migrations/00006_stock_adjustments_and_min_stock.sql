-- +goose Up
ALTER TABLE products ADD COLUMN min_stock INT NOT NULL DEFAULT 5;

CREATE TABLE stock_adjustments (
    id SERIAL PRIMARY KEY,
    adjustment_no VARCHAR(50) NOT NULL UNIQUE,
    date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    product_id INT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    system_stock INT NOT NULL,
    physical_stock INT NOT NULL,
    difference_qty INT NOT NULL,
    reason VARCHAR(50) NOT NULL, -- 'selisih_opname', 'rusak', 'kadaluarsa', 'pemakaian_toko', 'koreksi_input', 'lainnya'
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_stock_adjustments_product_id ON stock_adjustments(product_id);
CREATE INDEX idx_stock_adjustments_date ON stock_adjustments(date);

-- +goose Down
DROP TABLE IF EXISTS stock_adjustments;
ALTER TABLE products DROP COLUMN IF EXISTS min_stock;
