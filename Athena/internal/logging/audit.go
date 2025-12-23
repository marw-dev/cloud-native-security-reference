// internal/logging/audit.go
package logging

import (
	"athena/internal/middleware"
	"context"
	"log/slog"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type AuditStatus string

const (
	AuditSuccess AuditStatus = "SUCCESS"
	AuditFailure AuditStatus = "FAILURE"
)

const (
	AuditKey = "audit_event"
)


func LogAuditEvent(ctx context.Context, eventType string, status AuditStatus, details ...slog.Attr) {
	
	userID, _ := middleware.GetUserIDFromContext(ctx)
	projectID, _ := middleware.GetProjectIDFromContext(ctx)

	auditData, ok := middleware.GetAuditDataFromContext(ctx)
	if !ok {
		auditData = middleware.AuditData{}
	}

	traceID := chimiddleware.GetReqID(ctx)

	attrs := []slog.Attr{
		slog.String("event_type", eventType), 
		slog.String("status", string(status)), 
		slog.String("user_id", userID),       
		slog.String("project_id", projectID),   
		slog.String("trace_id", traceID),       
		slog.String("source_ip", auditData.IPAddress), 
	}

	attrs = append(attrs, details...)
	attrs = append(attrs, slog.Bool(AuditKey, true))


	args := make([]any, len(attrs))
	
	for i, attr := range attrs {
		args[i] = attr
	}

	slog.WarnContext(ctx, "Audit Event: "+eventType, args...)
}