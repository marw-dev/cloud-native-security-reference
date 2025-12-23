package middleware

import (
	"context"
	"net/http"
)

type auditContextKey string

const (
	AuditContextKey auditContextKey = "auditData"
)

type AuditData struct {
	IPAddress string
}

func AuditContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		ip := r.RemoteAddr

		data := AuditData{
			IPAddress: ip,
		}

		ctx = context.WithValue(ctx, AuditContextKey, data)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetAuditDataFromContext(ctx context.Context) (AuditData, bool) {
	data, ok := ctx.Value(AuditContextKey).(AuditData)
	return data, ok
}