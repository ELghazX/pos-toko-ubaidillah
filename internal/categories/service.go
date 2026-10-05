package categories

import (
	"context"
	"errors"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
)

type Service interface {
	ListCategories(ctx context.Context) ([]repo.Category, error)
	GetCategoryByID(ctx context.Context, id int32) (repo.Category, error)
	CreateCategory(ctx context.Context, code, name string) (repo.Category, error)
	UpdateCategory(ctx context.Context, id int32, code, name string) error
	DeleteCategory(ctx context.Context, id int32) error
}

type svc struct {
	repo repo.Querier
}

func NewService(repo repo.Querier) Service {
	return &svc{
		repo: repo,
	}
}

func (s *svc) ListCategories(ctx context.Context) ([]repo.Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *svc) GetCategoryByID(ctx context.Context, id int32) (repo.Category, error) {
	return s.repo.GetCategoryByID(ctx, id)
}

func (s *svc) CreateCategory(ctx context.Context, code, name string) (repo.Category, error) {
	code = strings.ToUpper(strings.TrimSpace(code))

	if len(code) < 3 || len(code) > 5 {
		return repo.Category{}, errors.New("Kode kategori harus terdiri dari 3 hingga 5 karakter")
	}

	arg := repo.CreateCategoryParams{
		Code: code,
		Name: name,
	}
	return s.repo.CreateCategory(ctx, arg)
}

func (s *svc) UpdateCategory(ctx context.Context, id int32, code, name string) error {
	code = strings.ToUpper(strings.TrimSpace(code))

	if len(code) < 3 || len(code) > 5 {
		return errors.New("Kode kategori harus terdiri dari 3 hingga 5 karakter")
	}

	arg := repo.UpdateCategoryParams{
		ID:   id,
		Code: code,
		Name: name,
	}
	return s.repo.UpdateCategory(ctx, arg)
}
func (s *svc) DeleteCategory(ctx context.Context, id int32) error {
	return s.repo.DeleteCategory(ctx, id)
}
