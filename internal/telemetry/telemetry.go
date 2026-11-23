package telemetry

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"hornerodb/internal/config"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

var enabled atomic.Bool

func init() {
	otel.SetTextMapPropagator(newPropagator())
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// Init configures the global OpenTelemetry trace and metric providers along with
// W3C traceparent/baggage propagation and Go runtime metrics.
// When telemetry is disabled or no endpoint is configured, it registers the W3C
// propagator and returns a no-op shutdown function.
func Init(ctx context.Context, cfg config.TelemetryConfig) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(newPropagator())

	endpoint, useInsecure := normalizeEndpoint(cfg.OTLPEndpoint, cfg.OTLPInsecure)
	if !cfg.Enabled || endpoint == "" {
		enabled.Store(false)
		return func(context.Context) error { return nil }, nil
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tp, err := initTracerProvider(ctx, endpoint, useInsecure, cfg.SampleRatio, res)
	if err != nil {
		return nil, err
	}

	mp, err := initMeterProvider(ctx, endpoint, useInsecure, res)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, err
	}

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		_ = errors.Join(mp.Shutdown(ctx), tp.Shutdown(ctx))
		return nil, err
	}

	enabled.Store(true)

	return func(shutdownCtx context.Context) error {
		enabled.Store(false)
		return errors.Join(mp.Shutdown(shutdownCtx), tp.Shutdown(shutdownCtx))
	}, nil
}

func buildResource(ctx context.Context, cfg config.TelemetryConfig) (*resource.Resource, error) {
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "hornerodb"
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
	}
	if env := strings.TrimSpace(cfg.RUM.Env); env != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentName(env))
	}
	if version := strings.TrimSpace(cfg.RUM.Version); version != "" {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}

	return resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
	)
}

func initTracerProvider(
	ctx context.Context,
	endpoint string,
	useInsecure bool,
	sampleRatio float64,
	res *resource.Resource,
) (*sdktrace.TracerProvider, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if useInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})))
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	ratio := sampleRatio
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	), nil
}

func initMeterProvider(
	ctx context.Context,
	endpoint string,
	useInsecure bool,
	res *resource.Resource,
) (*sdkmetric.MeterProvider, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
	}
	if useInsecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	} else {
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second))),
	), nil
}

// normalizeEndpoint strips http:// or https:// prefixes for gRPC WithEndpoint.
// An explicit https:// scheme selects TLS even when OTLPInsecure holds its
// default value of true; http:// selects insecure plaintext gRPC.
func normalizeEndpoint(raw string, insecure bool) (endpoint string, useInsecure bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", insecure
	}

	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "https://"):
		return strings.TrimRight(trimmed[len("https://"):], "/"), false
	case strings.HasPrefix(lower, "http://"):
		return strings.TrimRight(trimmed[len("http://"):], "/"), true
	default:
		return strings.TrimRight(trimmed, "/"), insecure
	}
}

// Enabled reports whether OpenTelemetry export is actively initialized.
func Enabled() bool {
	return enabled.Load()
}

// Tracer returns an OpenTelemetry Tracer for the given instrumentation scope.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// NewHTTPClient returns an *http.Client instrumented with otelhttp transport.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: WrapTransport(http.DefaultTransport),
	}
}

// WrapTransport wraps base with otelhttp.NewTransport, falling back to
// http.DefaultTransport if base is nil.
func WrapTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

// InjectTraceParent extracts the W3C traceparent header value from ctx, or ""
// if ctx has no valid span context.
func InjectTraceParent(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// ExtractTraceContext parses a W3C traceparent header into a child context
// derived from parent and returns the extracted SpanContext.
func ExtractTraceContext(parent context.Context, traceparent string) (context.Context, trace.SpanContext) {
	if parent == nil {
		parent = context.Background()
	}
	tp := strings.TrimSpace(traceparent)
	if tp == "" {
		return parent, trace.SpanContextFromContext(parent)
	}
	carrier := propagation.MapCarrier{"traceparent": tp}
	ctx := otel.GetTextMapPropagator().Extract(parent, carrier)
	return ctx, trace.SpanContextFromContext(ctx)
}

// TraceIDFromContext returns the hex trace ID and span ID from ctx, or "", ""
// when no valid span context is present.
func TraceIDFromContext(ctx context.Context) (traceID string, spanID string) {
	if ctx == nil {
		return "", ""
	}
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}
