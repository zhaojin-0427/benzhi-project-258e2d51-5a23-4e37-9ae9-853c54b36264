package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
)

type contextKey string

const requestIDKey contextKey = "request-id"

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	return value
}

func makeRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("150405.000000")
	}
	return hex.EncodeToString(b)
}

func commandContext(r *http.Request) (application.Context, error) {
	actor := r.Header.Get("X-Actor")
	role := domain.Role(r.Header.Get("X-Role"))
	if actor == "" || role == "" {
		return application.Context{}, domain.NewError("unauthorized", "必须提供 X-Actor 和 X-Role")
	}
	return application.Context{Actor: actor, Role: role, IdempotencyKey: r.Header.Get("Idempotency-Key")}, nil
}

func withRequestID(r *http.Request) *http.Request {
	id := r.Header.Get("X-Request-ID")
	if id == "" {
		id = makeRequestID()
	}
	return r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
}
