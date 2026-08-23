package tracing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-harness/pkg/config"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	globalTracer trace.Tracer
	tracerName   = "go-harness"
	mu           sync.RWMutex
	tpInstance   *sdktrace.TracerProvider
	openFiles    []*os.File
)

// Init initializes the OpenTelemetry TracerProvider according to configuration.
func Init(ctx context.Context, cfg *config.Config) (*sdktrace.TracerProvider, error) {
	mu.Lock()
	defer mu.Unlock()

	exporterType := strings.ToLower(cfg.OTelExporter)
	if exporterType == "none" || exporterType == "off" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		globalTracer = otel.Tracer(tracerName)
		return nil, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("go-harness"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	var spanProcessors []sdktrace.SpanProcessor

	// Parse comma-separated or keywords ("console", "file", "otlp", "all")
	exporters := strings.Split(exporterType, ",")
	wantsAll := false
	for _, e := range exporters {
		if strings.TrimSpace(e) == "all" {
			wantsAll = true
			break
		}
	}

	hasExporter := func(name string) bool {
		if wantsAll {
			return true
		}
		for _, e := range exporters {
			if strings.TrimSpace(e) == name {
				return true
			}
		}
		return false
	}

	// 1. Console Exporter
	if hasExporter("console") {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create stdout trace exporter")
		} else {
			spanProcessors = append(spanProcessors, sdktrace.NewBatchSpanProcessor(exp))
		}
	}

	// 2. File Exporter
	if hasExporter("file") {
		filePath := cfg.TraceFile
		if filePath == "" {
			filePath = "./logs/traces.json"
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err == nil {
			f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Warn().Err(err).Str("file", filePath).Msg("Failed to open trace file")
			} else {
				openFiles = append(openFiles, f)
				exp, err := stdouttrace.New(stdouttrace.WithWriter(f))
				if err != nil {
					log.Warn().Err(err).Msg("Failed to create file trace exporter")
				} else {
					spanProcessors = append(spanProcessors, sdktrace.NewBatchSpanProcessor(exp))
				}
			}
		}
	}

	// 3. OTLP Exporter
	if hasExporter("otlp") && cfg.OTelEndpoint != "" {
		dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
		defer dialCancel()

		otlpExp, err := otlptracegrpc.New(dialCtx,
			otlptracegrpc.WithEndpoint(cfg.OTelEndpoint),
			otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()),
			otlptracegrpc.WithDialOption(grpc.WithBlock()),
		)
		if err != nil {
			log.Warn().Err(err).Str("endpoint", cfg.OTelEndpoint).Msg("Could not connect to OTLP collector, skipping OTLP export")
		} else {
			spanProcessors = append(spanProcessors, sdktrace.NewBatchSpanProcessor(otlpExp))
		}
	}

	// If no span processor succeeded, fallback to noop or simple console
	if len(spanProcessors) == 0 {
		exp, _ := stdouttrace.New()
		spanProcessors = append(spanProcessors, sdktrace.NewSimpleSpanProcessor(exp))
	}

	var opts []sdktrace.TracerProviderOption
	opts = append(opts, sdktrace.WithResource(res))
	for _, sp := range spanProcessors {
		opts = append(opts, sdktrace.WithSpanProcessor(sp))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	tpInstance = tp
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	globalTracer = tp.Tracer(tracerName)

	log.Info().Str("exporters", exporterType).Msg("OpenTelemetry tracing initialized")
	return tp, nil
}

// Tracer returns the global harness tracer.
func Tracer() trace.Tracer {
	mu.RLock()
	defer mu.RUnlock()
	if globalTracer == nil {
		return otel.Tracer(tracerName)
	}
	return globalTracer
}

// Shutdown flushes and closes open trace exporters and file handles.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	var err error
	if tpInstance != nil {
		err = tpInstance.Shutdown(ctx)
		tpInstance = nil
	}

	for _, f := range openFiles {
		_ = f.Close()
	}
	openFiles = nil
	return err
}
