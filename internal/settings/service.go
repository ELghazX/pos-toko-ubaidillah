package settings

import (
	"context"
	"errors"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
)

type UpdateCompanyReq struct {
	Name          string `json:"name"`
	Tagline       string `json:"tagline"`
	Phone         string `json:"phone"`
	Address       string `json:"address"`
	ReceiptFooter string `json:"receipt_footer"`
}

type Service interface {
	GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error)
	UpdateCompanyConfig(ctx context.Context, req UpdateCompanyReq) error
}

type svc struct {
	repo *repo.Queries
}

func NewService(repo *repo.Queries) Service {
	return &svc{repo: repo}
}

func (s *svc) GetCompanyConfig(ctx context.Context) (repo.GetCompanyConfigRow, error) {
	cfg, err := s.repo.GetCompanyConfig(ctx)
	if err != nil {
		return repo.GetCompanyConfigRow{
			ID:            1,
			Name:          "Toko Ubaidillah",
			Tagline:       "ATK & Layanan Fotocopy",
			Phone:         "0812-3456-7890",
			Address:       "",
			ReceiptFooter: "Terima Kasih Atas Kunjungan Anda!\nBarang yang dibeli tidak dapat ditukar/dikembalikan",
		}, nil
	}
	return cfg, nil
}

func (s *svc) UpdateCompanyConfig(ctx context.Context, req UpdateCompanyReq) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errors.New("nama toko tidak boleh kosong")
	}

	return s.repo.UpdateCompanyConfig(ctx, repo.UpdateCompanyConfigParams{
		Name:          name,
		Tagline:       strings.TrimSpace(req.Tagline),
		Phone:         strings.TrimSpace(req.Phone),
		Address:       strings.TrimSpace(req.Address),
		ReceiptFooter: strings.TrimSpace(req.ReceiptFooter),
	})
}
