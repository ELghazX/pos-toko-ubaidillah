package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/go-chi/chi/v5"
)

type stubAuthService struct {
	users map[int32]*SessionUser
}

func (s *stubAuthService) EnsureDefaultUsers(ctx context.Context) error { return nil }
func (s *stubAuthService) Authenticate(ctx context.Context, username, password string) (*SessionUser, error) {
	return nil, ErrInvalidCredentials
}
func (s *stubAuthService) GetSessionUserByID(ctx context.Context, id int32) (*SessionUser, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, ErrInvalidCredentials
}
func (s *stubAuthService) ListUsers(ctx context.Context) ([]repo.User, error) { return nil, nil }
func (s *stubAuthService) CreateUser(ctx context.Context, username, name, password, role string) error {
	return nil
}
func (s *stubAuthService) UpdateUser(ctx context.Context, id int32, username, name, password, role string, isActive bool) error {
	return nil
}

func TestSessionTokenVerify(t *testing.T) {
	token := CreateSessionToken(42, time.Now().Add(time.Hour))
	userID, ok := VerifySessionToken(token)
	if !ok || userID != 42 {
		t.Fatalf("expected userID 42 and ok=true, got userID=%d ok=%v", userID, ok)
	}

	expiredToken := CreateSessionToken(42, time.Now().Add(-time.Minute))
	if _, ok := VerifySessionToken(expiredToken); ok {
		t.Fatalf("expected expired token to fail verification")
	}

	tampered := token + "abc"
	if _, ok := VerifySessionToken(tampered); ok {
		t.Fatalf("expected tampered token to fail verification")
	}
}

func TestRoleAccessControl(t *testing.T) {
	svc := &stubAuthService{
		users: map[int32]*SessionUser{
			1: {ID: 1, Username: "admin", Name: "Admin", Role: RoleAdmin},
			2: {ID: 2, Username: "kasir", Name: "Kasir", Role: RoleKasir},
		},
	}

	r := chi.NewRouter()
	r.Group(func(authRouter chi.Router) {
		authRouter.Use(RequireAuth(svc))

		authRouter.Get("/pos", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		authRouter.Get("/sales", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		authRouter.Get("/debts", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		authRouter.Get("/customers", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authRouter.Group(func(adminRouter chi.Router) {
			adminRouter.Use(RequireAdmin)
			adminRouter.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			adminRouter.Get("/products", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})
	})

	reqUnauth := httptest.NewRequest(http.MethodGet, "/pos", nil)
	recUnauth := httptest.NewRecorder()
	r.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusSeeOther || recUnauth.Header().Get("Location") != "/login" {
		t.Fatalf("expected unauthenticated redirect to /login, got status=%d loc=%s", recUnauth.Code, recUnauth.Header().Get("Location"))
	}

	kasirToken := CreateSessionToken(2, time.Now().Add(time.Hour))
	for _, path := range []string{"/pos", "/sales", "/debts", "/customers"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: kasirToken})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected kasir to access %s with 200, got %d", path, rec.Code)
		}
	}

	for _, adminPath := range []string{"/dashboard", "/products"} {
		req := httptest.NewRequest(http.MethodGet, adminPath, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: kasirToken})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/pos" {
			t.Fatalf("expected kasir on %s to redirect to /pos, got status=%d loc=%s", adminPath, rec.Code, rec.Header().Get("Location"))
		}
	}

	adminToken := CreateSessionToken(1, time.Now().Add(time.Hour))
	for _, path := range []string{"/pos", "/sales", "/debts", "/customers", "/dashboard", "/products"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: adminToken})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected admin to access %s with 200, got %d", path, rec.Code)
		}
	}
}
