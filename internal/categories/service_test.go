package categories

import (
	"context"
	"errors"
	"strings"
	"testing"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockCategoryRepo struct {
	repo.Querier
	categories []repo.Category
}

func (m *mockCategoryRepo) GetCategoryByID(ctx context.Context, id int32) (repo.Category, error) {
	for _, c := range m.categories {
		if c.ID == id {
			return c, nil
		}
	}
	return repo.Category{}, errors.New("not found")
}

func (m *mockCategoryRepo) GetCategoryByCode(ctx context.Context, code string) (repo.Category, error) {
	for _, c := range m.categories {
		if strings.EqualFold(c.Code, code) {
			return c, nil
		}
	}
	return repo.Category{}, errors.New("not found")
}

func (m *mockCategoryRepo) GetCategoryByName(ctx context.Context, name string) (repo.Category, error) {
	for _, c := range m.categories {
		if strings.EqualFold(c.Name, name) && c.IsActive.Valid && c.IsActive.Bool {
			return c, nil
		}
	}
	return repo.Category{}, errors.New("not found")
}

func (m *mockCategoryRepo) CreateCategory(ctx context.Context, arg repo.CreateCategoryParams) (repo.Category, error) {
	newCat := repo.Category{
		ID:       int32(len(m.categories) + 1),
		Code:     arg.Code,
		Name:     arg.Name,
		IsActive: pgtype.Bool{Bool: true, Valid: true},
	}
	m.categories = append(m.categories, newCat)
	return newCat, nil
}

func (m *mockCategoryRepo) UpdateCategory(ctx context.Context, arg repo.UpdateCategoryParams) error {
	for i, c := range m.categories {
		if c.ID == arg.ID {
			m.categories[i].Code = arg.Code
			m.categories[i].Name = arg.Name
			return nil
		}
	}
	return errors.New("not found")
}

func TestCreateCategory_DuplicateCode(t *testing.T) {
	mock := &mockCategoryRepo{
		categories: []repo.Category{
			{
				ID:       1,
				Code:     "ATK",
				Name:     "Alat Tulis Kantor",
				IsActive: pgtype.Bool{Bool: true, Valid: true},
			},
		},
	}
	svc := NewService(mock)

	_, err := svc.CreateCategory(context.Background(), "atk", "Alat Kantor Lain")
	if err == nil {
		t.Fatal("expected error for duplicate code, got nil")
	}

	if !strings.Contains(err.Error(), "sudah digunakan") || strings.Contains(err.Error(), "SQLSTATE") {
		t.Fatalf("expected friendly error message, got: %s", err.Error())
	}
}

func TestCreateCategory_DuplicateName(t *testing.T) {
	mock := &mockCategoryRepo{
		categories: []repo.Category{
			{
				ID:       1,
				Code:     "ATK",
				Name:     "Alat Tulis Kantor",
				IsActive: pgtype.Bool{Bool: true, Valid: true},
			},
		},
	}
	svc := NewService(mock)

	_, err := svc.CreateCategory(context.Background(), "KTR", "alat tulis kantor")
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}

	if !strings.Contains(err.Error(), "sudah terdaftar") || strings.Contains(err.Error(), "SQLSTATE") {
		t.Fatalf("expected friendly error message, got: %s", err.Error())
	}
}

func TestCreateCategory_InvalidCodeLength(t *testing.T) {
	svc := NewService(&mockCategoryRepo{})

	_, err := svc.CreateCategory(context.Background(), "AB", "Kertas")
	if err == nil {
		t.Fatal("expected error for short code, got nil")
	}

	_, err = svc.CreateCategory(context.Background(), "ABCDEF", "Kertas")
	if err == nil {
		t.Fatal("expected error for long code, got nil")
	}
}

func TestUpdateCategory_SelfUpdateAllowed(t *testing.T) {
	mock := &mockCategoryRepo{
		categories: []repo.Category{
			{
				ID:       1,
				Code:     "ATK",
				Name:     "Alat Tulis Kantor",
				IsActive: pgtype.Bool{Bool: true, Valid: true},
			},
		},
	}
	svc := NewService(mock)

	err := svc.UpdateCategory(context.Background(), 1, "ATK", "Alat Tulis Kantor")
	if err != nil {
		t.Fatalf("expected self update to succeed, got error: %v", err)
	}
}

func TestUpdateCategory_ConflictWithOtherCategory(t *testing.T) {
	mock := &mockCategoryRepo{
		categories: []repo.Category{
			{
				ID:       1,
				Code:     "ATK",
				Name:     "Alat Tulis Kantor",
				IsActive: pgtype.Bool{Bool: true, Valid: true},
			},
			{
				ID:       2,
				Code:     "KTS",
				Name:     "Kertas & Amplop",
				IsActive: pgtype.Bool{Bool: true, Valid: true},
			},
		},
	}
	svc := NewService(mock)

	// Trying to change category 2's code to ATK
	err := svc.UpdateCategory(context.Background(), 2, "ATK", "Kertas Baru")
	if err == nil {
		t.Fatal("expected error when updating to another category's code, got nil")
	}
	if !strings.Contains(err.Error(), "sudah digunakan") || strings.Contains(err.Error(), "SQLSTATE") {
		t.Fatalf("expected friendly error message, got: %s", err.Error())
	}

	// Trying to change category 2's name to Alat Tulis Kantor
	err = svc.UpdateCategory(context.Background(), 2, "KTS", "alat tulis kantor")
	if err == nil {
		t.Fatal("expected error when updating to another category's name, got nil")
	}
	if !strings.Contains(err.Error(), "sudah digunakan") || strings.Contains(err.Error(), "SQLSTATE") {
		t.Fatalf("expected friendly error message, got: %s", err.Error())
	}
}
