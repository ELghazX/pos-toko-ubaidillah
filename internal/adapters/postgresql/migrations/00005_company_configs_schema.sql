-- +goose Up
CREATE TABLE company_configs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL DEFAULT 'Toko Ubaidillah',
    tagline VARCHAR(200) NOT NULL DEFAULT 'ATK & Layanan Fotocopy',
    phone VARCHAR(50) NOT NULL DEFAULT '0812-3456-7890',
    address TEXT NOT NULL DEFAULT '',
    receipt_footer TEXT NOT NULL DEFAULT 'Terima Kasih Atas Kunjungan Anda!
Barang yang dibeli tidak dapat ditukar/dikembalikan',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO company_configs (id, name, tagline, phone, address, receipt_footer)
VALUES (1, 'Toko Ubaidillah', 'ATK & Layanan Fotocopy', '0812-3456-7890', '', 'Terima Kasih Atas Kunjungan Anda!
Barang yang dibeli tidak dapat ditukar/dikembalikan')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS company_configs;
