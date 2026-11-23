package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// capturingHandler records the attributes of every log record.
type capturingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *capturingHandler) attrs() []slog.Attr {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		return nil
	}

	collected := make([]slog.Attr, 0, 8)
	h.records[len(h.records)-1].Attrs(func(attr slog.Attr) bool {
		collected = append(collected, attr)
		return true
	})
	return collected
}

func (h *capturingHandler) attrValue(name string) (string, bool) {
	for _, attr := range h.attrs() {
		if attr.Key == name {
			return attr.Value.String(), true
		}
	}
	return "", false
}

// TestStructuredLogger_OmitsQueryString locks in that the raw query string
// (which routinely carries tokens and secrets) is never logged.
func TestStructuredLogger_OmitsQueryString(t *testing.T) {
	handler := &capturingHandler{}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(original)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StructuredLogger())
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test?token=super-secret-value&code=abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	path, ok := handler.attrValue("path")
	assert.True(t, ok, "the path attribute must be present")
	assert.Equal(t, "/api/test", path)
	assert.NotContains(t, path, "super-secret-value")
	assert.NotContains(t, path, "code=")
}

// TestStructuredLogger_IncludesUserAndRequestID verifies the correlation data.
func TestStructuredLogger_IncludesUserAndRequestID(t *testing.T) {
	handler := &capturingHandler{}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(original)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user-123")
		c.Next()
	})
	r.Use(StructuredLogger())
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-ID", "req-abc-123")
	r.ServeHTTP(w, req)

	userID, ok := handler.attrValue("user_id")
	assert.True(t, ok, "the user_id attribute must be present")
	assert.Equal(t, "user-123", userID)

	requestID, ok := handler.attrValue("request_id")
	assert.True(t, ok, "the request_id attribute must be present")
	assert.Equal(t, "req-abc-123", requestID)
	assert.Equal(t, "req-abc-123", w.Header().Get("X-Request-ID"))
}

// TestStructuredLogger_GeneratesRequestID verifies a request id is generated
// when the client does not provide one.
func TestStructuredLogger_GeneratesRequestID(t *testing.T) {
	handler := &capturingHandler{}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(original)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StructuredLogger())
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)

	requestID, ok := handler.attrValue("request_id")
	assert.True(t, ok, "a request id must be generated")
	assert.NotEmpty(t, requestID)
	assert.Equal(t, requestID, w.Header().Get("X-Request-ID"))
}

// TestStructuredLogger_IncludesTraceAndSpanID verifies trace_id and span_id are
// appended to log attributes when a valid OpenTelemetry span is active in context.
func TestStructuredLogger_IncludesTraceAndSpanID(t *testing.T) {
	handler := &capturingHandler{}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(original)

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "http-request")
	defer span.End()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StructuredLogger())
	r.GET("/api/traced", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/api/traced", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	traceID, ok := handler.attrValue("trace_id")
	assert.True(t, ok, "trace_id attribute must be present when span is active")
	assert.Equal(t, span.SpanContext().TraceID().String(), traceID)

	spanID, ok := handler.attrValue("span_id")
	assert.True(t, ok, "span_id attribute must be present when span is active")
	assert.Equal(t, span.SpanContext().SpanID().String(), spanID)
}

// TestSetSecureCookie_PreservesPreviousCookies is the regression test for the
// use of c.Header("Set-Cookie", ...) which wiped previously set cookies.
func TestSetSecureCookie_PreservesPreviousCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/test", func(c *gin.Context) {
		c.Writer.Header().Add("Set-Cookie", "first=1; Path=/")
		SetSecureCookie(c, "oidc_state", "state-value", 3600, true)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 2, "both cookies must be present")

	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	assert.Contains(t, names, "first")
	assert.Contains(t, names, "oidc_state")
}

// TestSetSecureCookie_SetsSameSite verifies the SameSite attribute survives.
func TestSetSecureCookie_SetsSameSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/test", func(c *gin.Context) {
		SetSecureCookie(c, "oidc_state", "state-value", 3600, true)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)

	raw := w.Header().Get("Set-Cookie")
	assert.True(t, strings.Contains(raw, "SameSite=Strict"), "oidc_state must use SameSite=Strict, got %q", raw)
	assert.True(t, cookies[0].HttpOnly, "the cookie must be HttpOnly")
}
