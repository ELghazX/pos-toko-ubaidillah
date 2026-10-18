package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ELghazX/pos-toko-ubaidillah/internal/env"
)

const (
	sessionCookieName = "pos_session"
	sessionDuration   = 7 * 24 * time.Hour
)

func getSessionSecret() []byte {
	secret := env.GetString("SESSION_SECRET", "pos-toko-ubaidillah-default-secret-key-2026")
	return []byte(secret)
}

func signPayload(payload string) string {
	mac := hmac.New(sha256.New, getSessionSecret())
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func CreateSessionToken(userID int32, expiresAt time.Time) string {
	payload := fmt.Sprintf("%d:%d", userID, expiresAt.Unix())
	sig := signPayload(payload)
	return fmt.Sprintf("%s:%s", payload, sig)
}

func VerifySessionToken(token string) (int32, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return 0, false
	}

	userID64, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil || userID64 <= 0 {
		return 0, false
	}

	expUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}

	if time.Now().Unix() > expUnix {
		return 0, false
	}

	payload := parts[0] + ":" + parts[1]
	expectedSig := signPayload(payload)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return 0, false
	}

	return int32(userID64), true
}

func SetSessionCookie(w http.ResponseWriter, userID int32) {
	expiresAt := time.Now().Add(sessionDuration)
	token := CreateSessionToken(userID, expiresAt)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ParseSessionCookie(r *http.Request) (int32, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return 0, false
	}
	return VerifySessionToken(cookie.Value)
}
