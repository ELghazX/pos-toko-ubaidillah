package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrInactiveUser       = errors.New("akun pengguna ini sedang dinonaktifkan")
)

type Service interface {
	EnsureDefaultUsers(ctx context.Context) error
	Authenticate(ctx context.Context, username, password string) (*SessionUser, error)
	GetSessionUserByID(ctx context.Context, id int32) (*SessionUser, error)
	ListUsers(ctx context.Context) ([]repo.User, error)
	CreateUser(ctx context.Context, username, name, password, role string) error
	UpdateUser(ctx context.Context, id int32, username, name, password, role string, isActive bool) error
}

type svc struct {
	repo repo.Querier
}

func NewService(r repo.Querier) Service {
	return &svc{repo: r}
}

func (s *svc) EnsureDefaultUsers(ctx context.Context) error {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	adminHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.repo.CreateUser(ctx, repo.CreateUserParams{
		Username:     "admin",
		Name:         "Administrator",
		PasswordHash: string(adminHash),
		Role:         RoleAdmin,
		IsActive:     true,
	})
	if err != nil {
		return err
	}

	kasirHash, err := bcrypt.GenerateFromPassword([]byte("kasir123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.repo.CreateUser(ctx, repo.CreateUserParams{
		Username:     "kasir",
		Name:         "Kasir Utama",
		PasswordHash: string(kasirHash),
		Role:         RoleKasir,
		IsActive:     true,
	})
	return err
}

func (s *svc) Authenticate(ctx context.Context, username, password string) (*SessionUser, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrInactiveUser
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return &SessionUser{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
		Role:     user.Role,
	}, nil
}

func (s *svc) GetSessionUserByID(ctx context.Context, id int32) (*SessionUser, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}
	return &SessionUser{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
		Role:     user.Role,
	}, nil
}

func (s *svc) ListUsers(ctx context.Context) ([]repo.User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *svc) CreateUser(ctx context.Context, username, name, password, role string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	name = strings.TrimSpace(name)
	if username == "" {
		return errors.New("username wajib diisi")
	}
	if name == "" {
		return errors.New("nama pengguna wajib diisi")
	}
	if len(password) < 4 {
		return errors.New("password minimal 4 karakter")
	}
	if role != RoleAdmin && role != RoleKasir {
		role = RoleKasir
	}

	if existing, err := s.repo.GetUserByUsername(ctx, username); err == nil && existing.ID > 0 {
		return fmt.Errorf("username '%s' sudah digunakan", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = s.repo.CreateUser(ctx, repo.CreateUserParams{
		Username:     username,
		Name:         name,
		PasswordHash: string(hash),
		Role:         role,
		IsActive:     true,
	})
	return err
}

func (s *svc) UpdateUser(ctx context.Context, id int32, username, name, password, role string, isActive bool) error {
	username = strings.ToLower(strings.TrimSpace(username))
	name = strings.TrimSpace(name)
	if username == "" {
		return errors.New("username wajib diisi")
	}
	if name == "" {
		return errors.New("nama pengguna wajib diisi")
	}
	if role != RoleAdmin && role != RoleKasir {
		role = RoleKasir
	}

	target, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return errors.New("pengguna tidak ditemukan")
	}

	if existing, err := s.repo.GetUserByUsername(ctx, username); err == nil && existing.ID != id {
		return fmt.Errorf("username '%s' sudah digunakan oleh pengguna lain", username)
	}

	if target.Role == RoleAdmin && (!isActive || role != RoleAdmin) {
		allUsers, err := s.repo.ListUsers(ctx)
		if err == nil {
			activeAdmins := 0
			for _, u := range allUsers {
				if u.Role == RoleAdmin && u.IsActive && u.ID != id {
					activeAdmins++
				}
			}
			if activeAdmins == 0 {
				return errors.New("tidak dapat menonaktifkan atau mengubah peran Admin terakhir yang aktif")
			}
		}
	}

	err = s.repo.UpdateUser(ctx, repo.UpdateUserParams{
		ID:       id,
		Username: username,
		Name:     name,
		Role:     role,
		IsActive: isActive,
	})
	if err != nil {
		return err
	}

	if strings.TrimSpace(password) != "" {
		if len(password) < 4 {
			return errors.New("password baru minimal 4 karakter")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := s.repo.UpdateUserPassword(ctx, repo.UpdateUserPasswordParams{
			ID:           id,
			PasswordHash: string(hash),
		}); err != nil {
			return err
		}
	}

	return nil
}
