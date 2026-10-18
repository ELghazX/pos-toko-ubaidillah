package categories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgconn"
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
	name = strings.TrimSpace(name)

	if len(code) < 3 || len(code) > 5 {
		return repo.Category{}, errors.New("Kode kategori harus terdiri dari 3 hingga 5 karakter")
	}
	if name == "" {
		return repo.Category{}, errors.New("Nama kategori tidak boleh kosong")
	}

	formattedName := utils.ToPascalCase(name)

	// Cek duplikasi kode sebelum insert
	if existing, err := s.repo.GetCategoryByCode(ctx, code); err == nil && existing.ID > 0 {
		if existing.IsActive.Valid && existing.IsActive.Bool {
			return repo.Category{}, fmt.Errorf("Kode kategori '%s' sudah digunakan oleh kategori '%s'", code, existing.Name)
		}
		return repo.Category{}, fmt.Errorf("Kode kategori '%s' sudah pernah digunakan pada data arsip nonaktif. Silakan gunakan kode lain.", code)
	}

	// Cek duplikasi nama sebelum insert
	if existing, err := s.repo.GetCategoryByName(ctx, formattedName); err == nil && existing.ID > 0 {
		return repo.Category{}, fmt.Errorf("Nama kategori '%s' sudah terdaftar (Kode: %s)", existing.Name, existing.Code)
	}

	arg := repo.CreateCategoryParams{
		Code: code,
		Name: formattedName,
	}

	cat, err := s.repo.CreateCategory(ctx, arg)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(strings.ToLower(pgErr.ConstraintName), "code") || strings.Contains(strings.ToLower(pgErr.Detail), "code") {
				return repo.Category{}, fmt.Errorf("Kode kategori '%s' sudah terdaftar. Silakan gunakan kode lain.", code)
			}
			if strings.Contains(strings.ToLower(pgErr.ConstraintName), "name") || strings.Contains(strings.ToLower(pgErr.Detail), "name") {
				return repo.Category{}, fmt.Errorf("Nama kategori '%s' sudah terdaftar. Silakan gunakan nama lain.", formattedName)
			}
			return repo.Category{}, errors.New("Kategori dengan kode atau nama tersebut sudah terdaftar.")
		}
		return repo.Category{}, err
	}

	return cat, nil
}

func (s *svc) UpdateCategory(ctx context.Context, id int32, code, name string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)

	if len(code) < 3 || len(code) > 5 {
		return errors.New("Kode kategori harus terdiri dari 3 hingga 5 karakter")
	}
	if name == "" {
		return errors.New("Nama kategori tidak boleh kosong")
	}

	formattedName := utils.ToPascalCase(name)

	// Cek apakah kategori ada
	if _, err := s.repo.GetCategoryByID(ctx, id); err != nil {
		return errors.New("Kategori tidak ditemukan")
	}

	// Cek duplikasi kode pada kategori lain
	if existing, err := s.repo.GetCategoryByCode(ctx, code); err == nil && existing.ID != id {
		if existing.IsActive.Valid && existing.IsActive.Bool {
			return fmt.Errorf("Kode kategori '%s' sudah digunakan oleh kategori '%s'", code, existing.Name)
		}
		return fmt.Errorf("Kode kategori '%s' sudah pernah digunakan pada data arsip nonaktif. Silakan gunakan kode lain.", code)
	}

	// Cek duplikasi nama pada kategori lain
	if existing, err := s.repo.GetCategoryByName(ctx, formattedName); err == nil && existing.ID != id {
		return fmt.Errorf("Nama kategori '%s' sudah digunakan oleh kategori lain (Kode: %s)", existing.Name, existing.Code)
	}

	arg := repo.UpdateCategoryParams{
		ID:   id,
		Code: code,
		Name: formattedName,
	}

	if err := s.repo.UpdateCategory(ctx, arg); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(strings.ToLower(pgErr.ConstraintName), "code") || strings.Contains(strings.ToLower(pgErr.Detail), "code") {
				return fmt.Errorf("Kode kategori '%s' sudah terdaftar. Silakan gunakan kode lain.", code)
			}
			if strings.Contains(strings.ToLower(pgErr.ConstraintName), "name") || strings.Contains(strings.ToLower(pgErr.Detail), "name") {
				return fmt.Errorf("Nama kategori '%s' sudah terdaftar. Silakan gunakan nama lain.", formattedName)
			}
			return errors.New("Kategori dengan kode atau nama tersebut sudah terdaftar.")
		}
		return err
	}

	return nil
}
func (s *svc) DeleteCategory(ctx context.Context, id int32) error {
	return s.repo.DeleteCategory(ctx, id)
}
