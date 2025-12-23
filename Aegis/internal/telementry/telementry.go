package telementry

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitTracerProvider(serviceName string) func(context.Context) error {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.DeploymentEnvironmentKey.String(os.Getenv("ENV")),
			attribute.String("git.commit.sha", os.Getenv("GIT_COMMIT")),
		),
	)
	if err != nil {
		log.Fatalf("Fehler beim Erstellen der Ressource %v", err)
	}

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("Fehler beim Erstellen des stdout-Exporters: %v", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	log.Println("OpenTelemetry TracerProvider initialisiert.")

	return tp.Shutdown
}