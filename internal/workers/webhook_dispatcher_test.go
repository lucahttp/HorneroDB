package workers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"

	"hornerodb/internal/models/metadata"
	"hornerodb/internal/telemetry"
)

// ---------------------------------------------------------------------------
// SSRF validation
// ---------------------------------------------------------------------------
func TestValidateWebhookURL_RejectsBlockedTargets(t *testing.T) {
	t.Setenv("HORNERO_ENV", "development")
	cases := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"ftp scheme", "ftp://example.com/hook"},
		{"file scheme", "file:///etc/passwd"},
		{"gopher scheme", "gopher://example.com/hook"},
		{"missing host", "https:///hook"},
		{"embedded credentials", "https://user:pass@example.com/hook"},
		{"loopback ipv4", "http://127.0.0.1/hook"},
		{"loopback ipv4 alt", "http://127.1.2.3/hook"},
		{"loopback ipv6", "http://[::1]/hook"},
		{"cloud metadata", "http://169.254.169.254/latest/meta-data"},
		{"cloud metadata ipv4 mapped", "http://[::ffff:169.254.169.254]/latest/meta-data"},
		{"private 10/8", "http://10.0.0.1/hook"},
		{"private 172.16/12", "http://172.16.5.4/hook"},
		{"private 192.168/16", "http://192.168.1.10/hook"},
		{"cgnat", "http://100.64.0.1/hook"},
		{"benchmarking", "http://198.18.0.1/hook"},
		{"unspecified", "http://0.0.0.0/hook"},
		{"ipv6 unique local", "http://[fd00::1]/hook"},
		{"ipv6 link local", "http://[fe80::1]/hook"},
		{"ipv6 mapped loopback", "http://[::ffff:127.0.0.1]/hook"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateWebhookURL(c.url); err == nil {
				t.Fatalf("expected %q to be rejected, got no error", c.url)
			}
		})
	}
}

func TestValidateWebhookURL_AllowsPublicAddresses(t *testing.T) {
	t.Setenv("HORNERO_ENV", "development")
	cases := []string{
		"https://8.8.8.8/hook",
		"http://1.1.1.1/hook",
		"https://93.184.216.34:8443/webhooks/hornero",
		"http://[2606:4700:4700::1111]/hook",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateWebhookURL(raw); err != nil {
				t.Fatalf("expected %q to be accepted, got error: %v", raw, err)
			}
		})
	}
}

func TestValidateWebhookURL_ProductionRequiresHTTPS(t *testing.T) {
	t.Setenv("HORNERO_ENV", "production")
	if err := ValidateWebhookURL("http://8.8.8.8/hook"); err == nil {
		t.Fatal("expected plain http to be rejected in production")
	}
	if err := ValidateWebhookURL("https://8.8.8.8/hook"); err != nil {
		t.Fatalf("expected https to be accepted in production, got: %v", err)
	}
}

func TestIsBlockedAddr(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"::1",
		"::ffff:169.254.169.254",
		"169.254.169.254",
		"10.1.2.3",
		"172.20.0.1",
		"192.168.0.1",
		"100.64.0.1",
		"198.18.0.1",
		"0.0.0.0",
		"::",
		"fd00::1",
		"fe80::1",
		"255.255.255.255",
	}
	for _, raw := range blocked {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("bad test address %q: %v", raw, err)
		}
		if !isBlockedAddr(addr) {
			t.Errorf("expected %s to be blocked", raw)
		}
	}
	allowed := []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
		"2606:4700:4700::1111",
	}
	for _, raw := range allowed {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("bad test address %q: %v", raw, err)
		}
		if isBlockedAddr(addr) {
			t.Errorf("expected %s to be allowed", raw)
		}
	}
}

// ---------------------------------------------------------------------------
// HMAC signature
// ---------------------------------------------------------------------------
func TestSignWebhookPayload(t *testing.T) {
	const (
		clientState = "s3cr3t-client-state"
		timestamp   = int64(1700000000)
		payload     = `{"event":"created"}`
	)
	const want = "t=1700000000,v1=94e2f4e677af927d5d92b84ff621a01a328b2a2a2399d468bd0521a1c4cc009e"
	got := signWebhookPayload(clientState, timestamp, []byte(payload))
	if got != want {
		t.Fatalf("signature mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestSignWebhookPayload_IsHMACSHA256OverTimestampDotPayload(t *testing.T) {
	const (
		clientState = "another-secret"
		timestamp   = int64(1730000000)
		payload     = `[{"value":[]}]`
	)
	got := signWebhookPayload(clientState, timestamp, []byte(payload))
	mac := hmac.New(sha256.New, []byte(clientState))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, payload)))
	wantDigest := hex.EncodeToString(mac.Sum(nil))
	if !strings.HasPrefix(got, fmt.Sprintf("t=%d,v1=", timestamp)) {
		t.Fatalf("unexpected signature format: %s", got)
	}
	if !strings.HasSuffix(got, wantDigest) {
		t.Fatalf("expected digest %s in %s", wantDigest, got)
	}
}

func TestSignWebhookPayload_DifferentSecretsProduceDifferentSignatures(t *testing.T) {
	payload := []byte(`{"x":1}`)
	a := signWebhookPayload("secret-a", 1700000000, payload)
	b := signWebhookPayload("secret-b", 1700000000, payload)
	if a == b {
		t.Fatal("expected different signatures for different client states")
	}
}

// ---------------------------------------------------------------------------
// change_type matching
// ---------------------------------------------------------------------------
func TestWebhookMatchesChangeType(t *testing.T) {
	cases := []struct {
		expression string
		changeType string
		want       bool
	}{
		{"created", "created", true},
		{"created,updated", "created", true},
		{"created,updated", "updated", true},
		{"created,updated", "deleted", false},
		{"*", "created", true},
		{"created, *", "deleted", true},
		{" created , updated ", "updated", true},
		{"", "created", false},
		{"   ", "created", false},
		{"created", "", false},
		// A substring must never match ("create" != "created").
		{"created", "create", false},
		{"recreated", "created", false},
	}
	for _, c := range cases {
		if got := webhookMatchesChangeType(c.expression, c.changeType); got != c.want {
			t.Errorf("webhookMatchesChangeType(%q, %q) = %v, want %v",
				c.expression, c.changeType, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Backoff
// ---------------------------------------------------------------------------
func TestComputeBackoff_IsExponentialAndCapped(t *testing.T) {
	cases := []struct {
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{1, 15 * time.Second, 20 * time.Second},
		{2, 30 * time.Second, 35 * time.Second},
		{3, 60 * time.Second, 65 * time.Second},
		{4, 120 * time.Second, 125 * time.Second},
		{20, time.Hour, time.Hour + webhookJitterMax},
		{50, time.Hour, time.Hour + webhookJitterMax},
	}
	for _, c := range cases {
		got := computeBackoff(c.attempt)
		if got < c.minDelay || got > c.maxDelay {
			t.Errorf("computeBackoff(%d) = %s, want between %s and %s",
				c.attempt, got, c.minDelay, c.maxDelay)
		}
	}
}

func TestComputeBackoff_JitterVaries(t *testing.T) {
	seen := make(map[time.Duration]bool)
	for i := 0; i < 20; i++ {
		seen[computeBackoff(1)] = true
	}
	if len(seen) < 2 {
		t.Fatal("expected jitter to produce different delays")
	}
}

// ---------------------------------------------------------------------------
// Distributed tracing & SSRF preservation
// ---------------------------------------------------------------------------

func setupTestTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})
	return sr
}

func TestExtractTxTraceparent(t *testing.T) {
	setupTestTracer(t)

	if got := extractTxTraceparent(nil); got != "" {
		t.Fatalf("extractTxTraceparent(nil) = %q, want empty", got)
	}
	if got := extractTxTraceparent(&gorm.DB{}); got != "" {
		t.Fatalf("extractTxTraceparent(no statement) = %q, want empty", got)
	}
	if got := extractTxTraceparent(&gorm.DB{Statement: &gorm.Statement{}}); got != "" {
		t.Fatalf("extractTxTraceparent(nil context) = %q, want empty", got)
	}

	ctx, span := telemetry.Tracer("test-write").Start(context.Background(), "record.create")
	defer span.End()

	tx := &gorm.DB{Statement: &gorm.Statement{Context: ctx}}
	got := extractTxTraceparent(tx)
	if got == "" {
		t.Fatal("extractTxTraceparent() = empty, want W3C traceparent")
	}
	_, sc := telemetry.ExtractTraceContext(context.Background(), got)
	if sc.TraceID() != span.SpanContext().TraceID() || sc.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("extracted SpanContext = (%s, %s), want (%s, %s)",
			sc.TraceID(), sc.SpanID(), span.SpanContext().TraceID(), span.SpanContext().SpanID())
	}
}

func TestDeliverWebhookHTTP_LinksUpstreamTraceAndPropagatesTraceparent(t *testing.T) {
	sr := setupTestTracer(t)

	// Simulate an originating write request span whose traceparent was saved in the outbox.
	origCtx, origSpan := telemetry.Tracer("hornerodb/api").Start(context.Background(), "POST /records")
	savedTraceparent := telemetry.InjectTraceParent(origCtx)
	origSpan.End()
	if savedTraceparent == "" {
		t.Fatal("expected non-empty saved traceparent")
	}

	var receivedTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceparent = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	event := metadata.WebhookOutboxEvent{
		ID:              uuid.New(),
		WebhookID:       uuid.New(),
		NotificationURL: srv.URL,
		Payload:         metadata.JSON(`{"value":[]}`),
		Attempts:        1,
		Traceparent:     savedTraceparent,
	}

	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: telemetry.WrapTransport(http.DefaultTransport),
	}
	statusCode, err := deliverWebhookHTTP(context.Background(), client, event, "secret")
	if err != nil {
		t.Fatalf("deliverWebhookHTTP() unexpected error: %v", err)
	}
	if statusCode != http.StatusAccepted {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusAccepted)
	}

	if receivedTraceparent == "" {
		t.Fatal("expected outbound webhook request to include traceparent header")
	}

	var deliverSpan sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Name() == "webhook.deliver" {
			deliverSpan = s
			break
		}
	}
	if deliverSpan == nil {
		t.Fatal("expected webhook.deliver span to be recorded")
	}

	// Verify trace link points back to the originating write span.
	links := deliverSpan.Links()
	if len(links) != 1 {
		t.Fatalf("expected 1 span link on webhook.deliver, got %d", len(links))
	}
	if links[0].SpanContext.TraceID() != origSpan.SpanContext().TraceID() ||
		links[0].SpanContext.SpanID() != origSpan.SpanContext().SpanID() {
		t.Fatalf("span link = (%s, %s), want (%s, %s)",
			links[0].SpanContext.TraceID(), links[0].SpanContext.SpanID(),
			origSpan.SpanContext().TraceID(), origSpan.SpanContext().SpanID())
	}

	// Verify outbound traceparent belongs to the delivery trace (not the old root trace, since it's linked).
	_, outboundSC := telemetry.ExtractTraceContext(context.Background(), receivedTraceparent)
	if !outboundSC.IsValid() {
		t.Fatalf("outbound traceparent %q produced invalid SpanContext", receivedTraceparent)
	}
	if outboundSC.TraceID() != deliverSpan.SpanContext().TraceID() {
		t.Fatalf("outbound TraceID = %s, want delivery span TraceID %s",
			outboundSC.TraceID(), deliverSpan.SpanContext().TraceID())
	}

	// Verify expected span attributes.
	attrs := make(map[attribute.Key]attribute.Value)
	for _, a := range deliverSpan.Attributes() {
		attrs[a.Key] = a.Value
	}
	if attrs["webhook.id"].AsString() != event.WebhookID.String() {
		t.Fatalf("webhook.id = %q, want %q", attrs["webhook.id"].AsString(), event.WebhookID.String())
	}
	if attrs["webhook.attempt"].AsInt64() != 2 {
		t.Fatalf("webhook.attempt = %d, want 2", attrs["webhook.attempt"].AsInt64())
	}
	if attrs["http.status_code"].AsInt64() != http.StatusAccepted {
		t.Fatalf("http.status_code = %d, want %d", attrs["http.status_code"].AsInt64(), http.StatusAccepted)
	}
}

func TestHTTPClient_PreservesSSRFProtectionWithWrappedTransport(t *testing.T) {
	t.Setenv("HORNERO_ENV", "development")

	// 1) sendWebhookPayload rejects blocked targets before dialing.
	event := metadata.WebhookOutboxEvent{
		ID:              uuid.New(),
		WebhookID:       uuid.New(),
		NotificationURL: "http://169.254.169.254/latest/meta-data",
		Payload:         metadata.JSON(`{"value":[]}`),
	}
	if status, err := sendWebhookPayload(event, ""); err == nil || status != 0 || !strings.Contains(err.Error(), "webhook target rejected") {
		t.Fatalf("expected SSRF rejection error, got status=%d err=%v", status, err)
	}

	// 2) httpClient's wrapped transport still dials via safeDialContext and blocks loopback/private IPs.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://127.0.0.1:1/hook", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	if _, err := httpClient.Do(req); err == nil || !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("expected safeDialContext to block 127.0.0.1 through wrapped transport, got err=%v", err)
	}
}
