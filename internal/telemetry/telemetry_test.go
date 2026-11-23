package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hornerodb/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestInitNoopWhenDisabledOrEmptyEndpoint(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.TelemetryConfig
	}{
		{
			name: "disabled",
			cfg: config.TelemetryConfig{
				Enabled:      false,
				OTLPEndpoint: "localhost:5081",
			},
		},
		{
			name: "empty endpoint",
			cfg: config.TelemetryConfig{
				Enabled:      true,
				OTLPEndpoint: "   ",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enabled.Store(true)
			t.Cleanup(func() { enabled.Store(false) })

			shutdown, err := Init(context.Background(), tc.cfg)
			if err != nil {
				t.Fatalf("Init() error = %v, want nil", err)
			}
			if Enabled() {
				t.Fatal("Enabled() = true, want false")
			}
			if err := shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown() error = %v, want nil", err)
			}
			if otel.GetTextMapPropagator() == nil {
				t.Fatal("expected global TextMapPropagator to be configured")
			}
		})
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		insecure     bool
		wantEndpoint string
		wantInsecure bool
	}{
		{
			name:         "strips http and uses insecure",
			raw:          "  http://openobserve:5081/  ",
			insecure:     false,
			wantEndpoint: "openobserve:5081",
			wantInsecure: true,
		},
		{
			name:         "strips https and enables TLS even when default insecure is true",
			raw:          "https://o2.example.com:4317/",
			insecure:     true,
			wantEndpoint: "o2.example.com:4317",
			wantInsecure: false,
		},
		{
			name:         "preserves host port and insecure flag",
			raw:          "localhost:4317",
			insecure:     true,
			wantEndpoint: "localhost:4317",
			wantInsecure: true,
		},
		{
			name:         "preserves host port and TLS flag",
			raw:          "otel.internal:4317",
			insecure:     false,
			wantEndpoint: "otel.internal:4317",
			wantInsecure: false,
		},
		{
			name:         "empty string stays empty",
			raw:          "   ",
			insecure:     true,
			wantEndpoint: "",
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEndpoint, gotInsecure := normalizeEndpoint(tt.raw, tt.insecure)
			if gotEndpoint != tt.wantEndpoint || gotInsecure != tt.wantInsecure {
				t.Fatalf("normalizeEndpoint(%q, %v) = (%q, %v), want (%q, %v)",
					tt.raw, tt.insecure, gotEndpoint, gotInsecure, tt.wantEndpoint, tt.wantInsecure)
			}
		})
	}
}

func TestInjectAndExtractTraceContextRoundTrip(t *testing.T) {
	_, err := Init(context.Background(), config.TelemetryConfig{Enabled: false})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "parent-op")
	defer span.End()

	traceparent := InjectTraceParent(ctx)
	if traceparent == "" {
		t.Fatal("InjectTraceParent() = empty, want valid W3C traceparent")
	}

	extractedCtx, sc := ExtractTraceContext(context.Background(), traceparent)
	if !sc.IsValid() {
		t.Fatalf("ExtractTraceContext() returned invalid SpanContext: %+v", sc)
	}
	if sc.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("extracted TraceID = %s, want %s", sc.TraceID(), span.SpanContext().TraceID())
	}
	if sc.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("extracted SpanID = %s, want %s", sc.SpanID(), span.SpanContext().SpanID())
	}
	if !sc.IsRemote() {
		t.Fatal("expected extracted SpanContext to be marked remote")
	}

	gotTraceID, gotSpanID := TraceIDFromContext(extractedCtx)
	if gotTraceID != span.SpanContext().TraceID().String() || gotSpanID != span.SpanContext().SpanID().String() {
		t.Fatalf("TraceIDFromContext(extractedCtx) = (%q, %q), want (%q, %q)",
			gotTraceID, gotSpanID, span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String())
	}
}

func TestExtractTraceContextHandlesEmptyAndNil(t *testing.T) {
	ctx, sc := ExtractTraceContext(nil, "")
	if ctx == nil {
		t.Fatal("ExtractTraceContext(nil, \"\") returned nil context")
	}
	if sc.IsValid() {
		t.Fatalf("expected invalid SpanContext, got %+v", sc)
	}
	if got := InjectTraceParent(nil); got != "" {
		t.Fatalf("InjectTraceParent(nil) = %q, want empty", got)
	}
}

func TestTraceIDFromContext(t *testing.T) {
	if tid, sid := TraceIDFromContext(nil); tid != "" || sid != "" {
		t.Fatalf("TraceIDFromContext(nil) = (%q, %q), want empty", tid, sid)
	}
	if tid, sid := TraceIDFromContext(context.Background()); tid != "" || sid != "" {
		t.Fatalf("TraceIDFromContext(background) = (%q, %q), want empty", tid, sid)
	}

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	tid, sid := TraceIDFromContext(ctx)
	if tid != span.SpanContext().TraceID().String() || sid != span.SpanContext().SpanID().String() {
		t.Fatalf("TraceIDFromContext(ctx) = (%q, %q), want (%q, %q)",
			tid, sid, span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String())
	}
}

func TestWrapTransportAndNewHTTPClientPropagateTrace(t *testing.T) {
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})

	var receivedTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceparent = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if WrapTransport(nil) == nil {
		t.Fatal("WrapTransport(nil) = nil, want wrapped default transport")
	}

	client := NewHTTPClient(2 * time.Second)
	if client.Timeout != 2*time.Second {
		t.Fatalf("client.Timeout = %v, want 2s", client.Timeout)
	}

	ctx, span := Tracer("test-http").Start(context.Background(), "outgoing")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	_ = resp.Body.Close()

	if receivedTraceparent == "" {
		t.Fatal("expected traceparent header to be propagated by NewHTTPClient")
	}
	_, remoteSC := ExtractTraceContext(context.Background(), receivedTraceparent)
	if remoteSC.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("propagated TraceID = %s, want %s", remoteSC.TraceID(), span.SpanContext().TraceID())
	}
}
