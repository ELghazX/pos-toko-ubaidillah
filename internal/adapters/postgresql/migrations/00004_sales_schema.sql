-- +goose Up
CREATE TABLE sales (
    id SERIAL PRIMARY KEY,
    invoice_no VARCHAR(50) NOT NULL UNIQUE,
    customer_id INT REFERENCES customers(id) ON DELETE SET NULL,
    date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    total_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    paid_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    change_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    payment_method VARCHAR(30) NOT NULL DEFAULT 'tunai', -- 'tunai', 'transfer', 'qris', 'kasbon'
    payment_status VARCHAR(20) NOT NULL DEFAULT 'lunas',  -- 'lunas', 'kasbon'
    due_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'sukses',        -- 'sukses', 'batal'
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sale_details (
    id SERIAL PRIMARY KEY,
    sale_id INT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id INT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    unit_id INT REFERENCES product_units(id) ON DELETE SET NULL,
    unit_name VARCHAR(50) NOT NULL,
    conversion_qty INT NOT NULL DEFAULT 1,
    qty INT NOT NULL,
    price NUMERIC(15,2) NOT NULL DEFAULT 0,
    discount NUMERIC(15,2) NOT NULL DEFAULT 0,
    subtotal NUMERIC(15,2) NOT NULL DEFAULT 0,
    cogs NUMERIC(15,2) NOT NULL DEFAULT 0,
    is_open_price BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE debt_payments (
    id SERIAL PRIMARY KEY,
    sale_id INT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    amount_paid NUMERIC(15,2) NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sales_invoice_no ON sales(invoice_no);
CREATE INDEX idx_sales_date ON sales(date);
CREATE INDEX idx_sales_customer_id ON sales(customer_id);
CREATE INDEX idx_sale_details_sale_id ON sale_details(sale_id);
CREATE INDEX idx_sale_details_product_id ON sale_details(product_id);

-- +goose Down
DROP TABLE IF EXISTS debt_payments;
DROP TABLE IF EXISTS sale_details;
DROP TABLE IF EXISTS sales;
