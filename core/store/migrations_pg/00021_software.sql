-- +goose Up

CREATE TABLE IF NOT EXISTS software_products (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  vendor TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  tags_json TEXT NOT NULL DEFAULT '[]',
  created_by BIGINT,
  updated_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  version INT NOT NULL DEFAULT 1,
  deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_software_products_name ON software_products(name);
CREATE INDEX IF NOT EXISTS idx_software_products_vendor ON software_products(vendor);
CREATE INDEX IF NOT EXISTS idx_software_products_deleted_at ON software_products(deleted_at);
CREATE INDEX IF NOT EXISTS idx_software_products_updated_at ON software_products(updated_at);

CREATE TABLE IF NOT EXISTS software_versions (
  id BIGSERIAL PRIMARY KEY,
  product_id BIGINT NOT NULL REFERENCES software_products(id) ON DELETE CASCADE,
  version TEXT NOT NULL,
  release_date DATE,
  eol_date DATE,
  notes TEXT NOT NULL DEFAULT '',
  created_by BIGINT,
  updated_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_software_versions_product_id ON software_versions(product_id);
CREATE INDEX IF NOT EXISTS idx_software_versions_eol_date ON software_versions(eol_date);
CREATE INDEX IF NOT EXISTS idx_software_versions_deleted_at ON software_versions(deleted_at);

CREATE TABLE IF NOT EXISTS asset_software (
  id BIGSERIAL PRIMARY KEY,
  asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  product_id BIGINT NOT NULL REFERENCES software_products(id) ON DELETE CASCADE,
  version_id BIGINT REFERENCES software_versions(id) ON DELETE SET NULL,
  version_text TEXT NOT NULL DEFAULT '',
  installed_at TIMESTAMPTZ,
  source TEXT NOT NULL DEFAULT 'manual',
  notes TEXT NOT NULL DEFAULT '',
  created_by BIGINT,
  updated_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_asset_software_asset_id ON asset_software(asset_id);
CREATE INDEX IF NOT EXISTS idx_asset_software_product_id ON asset_software(product_id);
CREATE INDEX IF NOT EXISTS idx_asset_software_version_id ON asset_software(version_id);
CREATE INDEX IF NOT EXISTS idx_asset_software_deleted_at ON asset_software(deleted_at);

-- +goose Down

DROP INDEX IF EXISTS idx_asset_software_deleted_at;
DROP INDEX IF EXISTS idx_asset_software_version_id;
DROP INDEX IF EXISTS idx_asset_software_product_id;
DROP INDEX IF EXISTS idx_asset_software_asset_id;
DROP TABLE IF EXISTS asset_software;

DROP INDEX IF EXISTS idx_software_versions_deleted_at;
DROP INDEX IF EXISTS idx_software_versions_eol_date;
DROP INDEX IF EXISTS idx_software_versions_product_id;
DROP TABLE IF EXISTS software_versions;

DROP INDEX IF EXISTS idx_software_products_updated_at;
DROP INDEX IF EXISTS idx_software_products_deleted_at;
DROP INDEX IF EXISTS idx_software_products_vendor;
DROP INDEX IF EXISTS idx_software_products_name;
DROP TABLE IF EXISTS software_products;
