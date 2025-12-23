package tracing

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

func InitTracerProvider(serviceName string) *trace.TracerProvider {
	
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName), 
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		log.Fatalf("Fehler beim Erstellen der Resource: %v", err)
	}

	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		log.Fatalf("Fehler beim Erstellen des Stdout-Exporters: %v", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(res),
		trace.WithBatcher(exporter), 
	)

	otel.SetTracerProvider(tp)
	
	otel.SetTextMapPropagator(propagation.TraceContext{})

	log.Println("OpenTelemetry TracerProvider initialisiert.")
	return tp
}