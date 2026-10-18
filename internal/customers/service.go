package customers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service interface {
	ListCustomers(ctx context.Context) ([]repo.Customer, error)
	ListCustomersWithFilter(ctx context.Context, status string) ([]repo.Customer, error)
	GetCustomerByID(ctx context.Context, id int32) (repo.Customer, error)
	CreateCustomer(ctx context.Context, name, phone, address string) (repo.Customer, error)
	UpdateCustomer(ctx context.Context, id int32, name, phone, address string) error
	DeleteCustomer(ctx context.Context, id int32) error
	RestoreCustomer(ctx context.Context, id int32) error
}

type svc struct {
	repo repo.Querier
}

func NewService(repo repo.Querier) Service {
	return &svc{repo: repo}
}

func (s *svc) ListCustomers(ctx context.Context) ([]repo.Customer, error) {
	return s.repo.ListCustomers(ctx)
}

func (s *svc) ListCustomersWithFilter(ctx context.Context, status string) ([]repo.Customer, error) {
	if status == "" {
		status = "active"
	}
	return s.repo.ListCustomersWithFilter(ctx, status)
}

func (s *svc) GetCustomerByID(ctx context.Context, id int32) (repo.Customer, error) {
	return s.repo.GetCustomerByIDAnyStatus(ctx, id)
}

func (s *svc) CreateCustomer(ctx context.Context, name, phone, address string) (repo.Customer, error) {
	cleanName := utils.ToPascalCase(strings.TrimSpace(name))
	if cleanName == "" {
		return repo.Customer{}, errors.New("nama pelanggan wajib diisi")
	}

	return s.repo.CreateCustomer(ctx, repo.CreateCustomerParams{
		Name:    cleanName,
		Phone:   pgtype.Text{String: strings.TrimSpace(phone), Valid: strings.TrimSpace(phone) != ""},
		Address: pgtype.Text{String: strings.TrimSpace(address), Valid: strings.TrimSpace(address) != ""},
	})
}

func (s *svc) UpdateCustomer(ctx context.Context, id int32, name, phone, address string) error {
	cleanName := utils.ToPascalCase(strings.TrimSpace(name))
	if cleanName == "" {
		return errors.New("nama pelanggan wajib diisi")
	}

	return s.repo.UpdateCustomer(ctx, repo.UpdateCustomerParams{
		ID:      id,
		Name:    cleanName,
		Phone:   pgtype.Text{String: strings.TrimSpace(phone), Valid: strings.TrimSpace(phone) != ""},
		Address: pgtype.Text{String: strings.TrimSpace(address), Valid: strings.TrimSpace(address) != ""},
	})
}

func (s *svc) DeleteCustomer(ctx context.Context, id int32) error {
	debtNumeric, err := s.repo.GetCustomerUnpaidDebtTotal(ctx, pgtype.Int4{Int32: id, Valid: true})
	if err != nil {
		return fmt.Errorf("gagal memeriksa tanggungan kasbon pelanggan: %w", err)
	}

	remainingDebt := utils.NumericToFloat(debtNumeric)
	if remainingDebt > 0.001 {
		return fmt.Errorf("Pelanggan tidak dapat dihapus karena masih memiliki tanggungan kasbon aktif sebesar Rp %s. Silakan lunasi terlebih dahulu di Buku Rekap Kasbon.", utils.FormatRupiah(debtNumeric))
	}

	return s.repo.DeleteCustomer(ctx, id)
}

func (s *svc) RestoreCustomer(ctx context.Context, id int32) error {
	return s.repo.RestoreCustomer(ctx, id)
}
