package store

import (
	"context"
	"database/sql"
	"time"
)

type SoftwareProduct struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Vendor      string     `json:"vendor"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags,omitempty"`
	CreatedBy   *int64     `json:"created_by,omitempty"`
	UpdatedBy   *int64     `json:"updated_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Version     int        `json:"version"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type SoftwareProductLite struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
}

type SoftwareVersion struct {
	ID          int64      `json:"id"`
	ProductID   int64      `json:"product_id"`
	Version     string     `json:"version"`
	ReleaseDate *time.Time `json:"release_date,omitempty"`
	EOLDate     *time.Time `json:"eol_date,omitempty"`
	Notes       string     `json:"notes"`
	CreatedBy   *int64     `json:"created_by,omitempty"`
	UpdatedBy   *int64     `json:"updated_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type AssetSoftwareInstallation struct {
	ID          int64      `json:"id"`
	AssetID     int64      `json:"asset_id"`
	ProductID   int64      `json:"product_id"`
	VersionID   *int64     `json:"version_id,omitempty"`
	VersionText string     `json:"version_text"`
	InstalledAt *time.Time `json:"installed_at,omitempty"`
	Source      string     `json:"source"`
	Notes       string     `json:"notes"`
	CreatedBy   *int64     `json:"created_by,omitempty"`
	UpdatedBy   *int64     `json:"updated_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`

	ProductName   string `json:"product_name,omitempty"`
	ProductVendor string `json:"product_vendor,omitempty"`
	VersionLabel  string `json:"version_label,omitempty"`
	AssetName     string `json:"asset_name,omitempty"`
}

type SoftwareFilter struct {
	Search         string
	Vendor         string
	Tag            string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

type SoftwareStore interface {
	ListProducts(ctx context.Context, filter SoftwareFilter) ([]SoftwareProduct, error)
	ListProductsLite(ctx context.Context, search string, limit int) ([]SoftwareProductLite, error)
	GetProduct(ctx context.Context, id int64) (*SoftwareProduct, error)
	CreateProduct(ctx context.Context, p *SoftwareProduct) (int64, error)
	UpdateProduct(ctx context.Context, p *SoftwareProduct) error
	ArchiveProduct(ctx context.Context, id int64, updatedBy int64) error
	RestoreProduct(ctx context.Context, id int64, updatedBy int64) error

	ListVersions(ctx context.Context, productID int64, includeDeleted bool) ([]SoftwareVersion, error)
	CreateVersion(ctx context.Context, v *SoftwareVersion) (int64, error)
	UpdateVersion(ctx context.Context, v *SoftwareVersion) error
	ArchiveVersion(ctx context.Context, id int64, updatedBy int64) error
	RestoreVersion(ctx context.Context, id int64, updatedBy int64) error

	ListAssetSoftware(ctx context.Context, assetID int64, includeDeleted bool) ([]AssetSoftwareInstallation, error)
	AddAssetSoftware(ctx context.Context, inst *AssetSoftwareInstallation) (int64, error)
	UpdateAssetSoftware(ctx context.Context, inst *AssetSoftwareInstallation) error
	ArchiveAssetSoftware(ctx context.Context, id int64, updatedBy int64) error
	RestoreAssetSoftware(ctx context.Context, id int64, updatedBy int64) error
	ListProductAssets(ctx context.Context, productID int64, includeDeleted bool) ([]AssetSoftwareInstallation, error)

	SuggestProductNames(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	SuggestVendors(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	SuggestProductTags(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
}

type softwareStore struct {
	db *sql.DB
}

func NewSoftwareStore(db *sql.DB) SoftwareStore {
	return &softwareStore{db: db}
}
