package components

import "context"

type authContextKey string

const userContextKey authContextKey = "pos_auth_user"

const (
	RoleAdmin = "admin"
	RoleKasir = "kasir"
)

type SessionUser struct {
	ID       int32
	Username string
	Name     string
	Role     string
}

func (u *SessionUser) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin
}

func (u *SessionUser) IsKasir() bool {
	return u != nil && u.Role == RoleKasir
}

func (u *SessionUser) RoleLabel() string {
	if u == nil {
		return ""
	}
	if u.Role == RoleAdmin {
		return "Admin"
	}
	return "Kasir"
}

func WithUser(ctx context.Context, user *SessionUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *SessionUser {
	if ctx == nil {
		return nil
	}
	u, ok := ctx.Value(userContextKey).(*SessionUser)
	if !ok {
		return nil
	}
	return u
}

func IsAdminContext(ctx context.Context) bool {
	u := UserFromContext(ctx)
	if u == nil {
		return true
	}
	return u.IsAdmin()
}
