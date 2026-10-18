package auth

import (
	"context"

	"github.com/ELghazX/pos-toko-ubaidillah/internal/views/components"
)

const (
	RoleAdmin = components.RoleAdmin
	RoleKasir = components.RoleKasir
)

type SessionUser = components.SessionUser

func WithUser(ctx context.Context, user *SessionUser) context.Context {
	return components.WithUser(ctx, user)
}

func UserFromContext(ctx context.Context) *SessionUser {
	return components.UserFromContext(ctx)
}
