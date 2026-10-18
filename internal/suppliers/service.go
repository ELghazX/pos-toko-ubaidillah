package suppliers

import (
	"context"
	"errors"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service interface {
	ListSuppliers(ctx context.Context) ([]repo.Supplier, error)
	GetSupplierByID(ctx context.Context, id int32) (repo.Supplier, error)
	CreateSupplier(ctx context.Context, name, phone, address string) (repo.Supplier, error)
	UpdateSupplier(ctx context.Context, id int32, name, phone, address string) error
	DeleteSupplier(ctx context.Context, id int32) error
}

type svc struct {
	repo *repo.Queries
}

func NewService(repo *repo.Queries) Service {
	return &svc{repo: repo}
}

func (s *svc) ListSuppliers(ctx context.Context) ([]repo.Supplier, error) {
	return s.repo.ListSuppliers(ctx)
}

func (s *svc) GetSupplierByID(ctx context.Context, id int32) (repo.Supplier, error) {
	return s.repo.GetSupplierByID(ctx, id)
}

func (s *svc) CreateSupplier(ctx context.Context, name, phone, address string) (repo.Supplier, error) {
	cleanName := utils.ToPascalCase(strings.TrimSpace(name))
	if cleanName == "" {
		return repo.Supplier{}, errors.New("nama supplier wajib diisi")
	}

	return s.repo.CreateSupplier(ctx, repo.CreateSupplierParams{
		Name:    cleanName,
		Phone:   pgtype.Text{String: strings.TrimSpace(phone), Valid: strings.TrimSpace(phone) != ""},
		Address: pgtype.Text{String: strings.TrimSpace(address), Valid: strings.TrimSpace(address) != ""},
	})
}

func (s *svc) UpdateSupplier(ctx context.Context, id int32, name, phone, address string) error {
	cleanName := utils.ToPascalCase(strings.TrimSpace(name))
	if cleanName == "" {
		return errors.New("nama supplier wajib diisi")
	}

	return s.repo.UpdateSupplier(ctx, repo.UpdateSupplierParams{
		ID:      id,
		Name:    cleanName,
		Phone:   pgtype.Text{String: strings.TrimSpace(phone), Valid: strings.TrimSpace(phone) != ""},
		Address: pgtype.Text{String: strings.TrimSpace(address), Valid: strings.TrimSpace(address) != ""},
	})
}

func (s *svc) DeleteSupplier(ctx context.Context, id int32) error {
	return s.repo.DeleteSupplier(ctx, id)
}
