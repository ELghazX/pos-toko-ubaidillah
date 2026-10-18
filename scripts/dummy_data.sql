TRUNCATE categories, products, product_units RESTART IDENTITY CASCADE;

INSERT INTO categories (code, name) VALUES 
('ATK', 'Alat Tulis Kantor'),
('BKU', 'Buku Tulis & Cetak'),
('JSA', 'Jasa Fotokopi & Print'),
('KRT', 'Kertas & Karton');

INSERT INTO products (category_id, sku, name, type, current_stock, average_cost) VALUES 
-- Kategori 1 (ATK) - ID 1
(1, 'ATK-0001', 'Pulpen Snowman V5 Hitam', 'barang', 150, 1500),
(1, 'ATK-0002', 'Pulpen Snowman V5 Biru', 'barang', 100, 1500),
(1, 'ATK-0003', 'Spidol Boardmarker Hitam', 'barang', 50, 6000),
(1, 'ATK-0004', 'Tipe-X Kenko', 'barang', 75, 4000),
(1, 'ATK-0005', 'Pensil 2B Faber Castell', 'barang', 200, 2500),
(1, 'ATK-0006', 'Penggaris Besi 30cm', 'barang', 40, 5000),
(1, 'ATK-0007', 'Staples Joyko Kecil', 'barang', 60, 4500),
(1, 'ATK-0008', 'Isi Staples Joyko', 'barang', 300, 1500),
(1, 'ATK-0009', 'Gunting Sedang Joyko', 'barang', 25, 8000),
(1, 'ATK-0010', 'Lem Kertas Gluestick', 'barang', 80, 5000),
(1, 'ATK-0011', 'Penghapus Joyko Hitam', 'barang', 120, 1000),

-- Kategori 2 (Buku) - ID 2
(2, 'BKU-0001', 'Buku Tulis Sinar Dunia 38 Lembar', 'barang', 100, 3000),
(2, 'BKU-0002', 'Buku Tulis Sinar Dunia 58 Lembar', 'barang', 100, 4500),
(2, 'BKU-0003', 'Buku Gambar A3', 'barang', 30, 8000),
(2, 'BKU-0004', 'Buku Kas Folio 100 Lembar', 'barang', 20, 15000),
(2, 'BKU-0005', 'Buku Kotak Kecil', 'barang', 50, 3500),

-- Kategori 3 (Jasa) - ID 3
(3, 'JSA-0001', 'Fotokopi A4 Hitam Putih', 'jasa', 0, 100),
(3, 'JSA-0002', 'Print Warna A4', 'jasa', 0, 500),
(3, 'JSA-0003', 'Jilid Lakban Biasa', 'jasa', 0, 2000),
(3, 'JSA-0004', 'Jilid Spiral Mika', 'jasa', 0, 15000),
(3, 'JSA-0005', 'Laminating A4', 'jasa', 0, 3000),

-- Kategori 4 (Kertas) - ID 4
(4, 'KRT-0001', 'Kertas HVS A4 70gr PaperOne', 'barang', 5000, 50),
(4, 'KRT-0002', 'Kertas HVS F4 70gr', 'barang', 3000, 60),
(4, 'KRT-0003', 'Kertas Buffalo Warna Biru', 'barang', 200, 300),
(4, 'KRT-0004', 'Karton Manila Putih', 'barang', 100, 2500);

-- Base Units
INSERT INTO product_units (product_id, unit_name, conversion_qty, price) VALUES
(1, 'Pcs', 1, 2500),
(2, 'Pcs', 1, 2500),
(3, 'Pcs', 1, 8500),
(4, 'Pcs', 1, 6000),
(5, 'Pcs', 1, 3500),
(6, 'Pcs', 1, 8000),
(7, 'Pcs', 1, 7000),
(8, 'Kotak', 1, 2500),
(9, 'Pcs', 1, 12000),
(10, 'Pcs', 1, 8000),
(11, 'Pcs', 1, 2000),
(12, 'Buku', 1, 4500),
(13, 'Buku', 1, 6500),
(14, 'Buku', 1, 12000),
(15, 'Buku', 1, 22000),
(16, 'Buku', 1, 5000),
(17, 'Lembar', 1, 250),
(18, 'Lembar', 1, 1500),
(19, 'Buku', 1, 5000),
(20, 'Buku', 1, 20000),
(21, 'Lembar', 1, 5000),
(22, 'Lembar', 1, 150),
(23, 'Lembar', 1, 150),
(24, 'Lembar', 1, 1000),
(25, 'Lembar', 1, 4000);

-- Extra Units (Satuan Grosir)
INSERT INTO product_units (product_id, unit_name, conversion_qty, price) VALUES
(1, 'Lusin', 12, 28000),
(1, 'Kotak', 12, 28000),
(5, 'Lusin', 12, 40000),
(12, 'Pack', 10, 42000),
(13, 'Pack', 10, 62000),
(22, 'Rim', 500, 55000),
(23, 'Rim', 500, 57000);

