package customers

import (
	"context"
	"strings"
	"testing"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockCustomerRepo struct {
	repo.Querier
	remainingDebt pgtype.Numeric
	deletedID     int32
	restoredID    int32
}

func (m *mockCustomerRepo) GetCustomerUnpaidDebtTotal(ctx context.Context, customerID pgtype.Int4) (pgtype.Numeric, error) {
	return m.remainingDebt, nil
}

func (m *mockCustomerRepo) DeleteCustomer(ctx context.Context, id int32) error {
	m.deletedID = id
	return nil
}

func (m *mockCustomerRepo) RestoreCustomer(ctx context.Context, id int32) error {
	m.restoredID = id
	return nil
}

func TestDeleteCustomer_BlockedByDebt(t *testing.T) {
	mockRepo := &mockCustomerRepo{
		remainingDebt: utils.FloatToNumeric(75000),
	}
	s := NewService(mockRepo)

	err := s.DeleteCustomer(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error when customer has unpaid debt, got nil")
	}

	if !strings.Contains(err.Error(), "75.000") {
		t.Fatalf("expected error message to contain debt amount 75.000, got: %s", err.Error())
	}

	if mockRepo.deletedID != 0 {
		t.Fatalf("customer should not have been deleted in repo, but got deletedID=%d", mockRepo.deletedID)
	}
}

func TestDeleteCustomer_SuccessWhenNoDebt(t *testing.T) {
	mockRepo := &mockCustomerRepo{
		remainingDebt: utils.FloatToNumeric(0),
	}
	s := NewService(mockRepo)

	err := s.DeleteCustomer(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if mockRepo.deletedID != 10 {
		t.Fatalf("expected deletedID to be 10, got %d", mockRepo.deletedID)
	}
}

func TestRestoreCustomer_Success(t *testing.T) {
	mockRepo := &mockCustomerRepo{}
	s := NewService(mockRepo)

	err := s.RestoreCustomer(context.Background(), 15)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if mockRepo.restoredID != 15 {
		t.Fatalf("expected restoredID to be 15, got %d", mockRepo.restoredID)
	}
}
