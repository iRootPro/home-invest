package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "user"

type SessionData struct {
	UserID         int64  `json:"uid"`
	Username       string `json:"uname"`
	FamilyMemberID int64  `json:"fmid,omitempty"`
	IsAdmin        bool   `json:"adm"`
}

var sessionSecret []byte

func InitSessions(secret string) {
	sessionSecret = []byte(secret)
}

func SetSession(w http.ResponseWriter, data SessionData) {
	payload, _ := json.Marshal(data)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(encoded)
	value := encoded + "." + sig

	http.SetCookie(w, &http.Cookie{
		Name:     "banki-session",
		Value:    value,
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "banki-session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func GetSession(r *http.Request) *SessionData {
	if v := r.Context().Value(userContextKey); v != nil {
		return v.(*SessionData)
	}
	return nil
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("banki-session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		data, err := parseSession(cookie.Value)
		if err != nil {
			ClearSession(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, data)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// IsAdmin returns true if the current user is an admin
func IsAdmin(r *http.Request) bool {
	s := GetSession(r)
	return s != nil && s.IsAdmin
}

// GetFamilyMemberID returns the family member ID for the current user (0 for admin)
func GetFamilyMemberID(r *http.Request) int64 {
	s := GetSession(r)
	if s == nil {
		return 0
	}
	return s.FamilyMemberID
}

// RequireAdmin returns 403 for non-admin users
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdmin(r) {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseSession(value string) (*SessionData, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid session")
	}
	if !hmac.Equal([]byte(sign(parts[0])), []byte(parts[1])) {
		return nil, fmt.Errorf("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var data SessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func sign(data string) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

