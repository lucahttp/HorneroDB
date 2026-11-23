package workers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"
	"hornerodb/internal/telemetry"
)

// ---------------------------------------------------------------------------
// Tunables
// ---------------------------------------------------------------------------

const (
	// webhookPollInterval is how often the outbox is polled for pending events.
	webhookPollInterval = 5 * time.Second
	// webhookBatchSize is the maximum number of events claimed per cycle.
	webhookBatchSize = 50
	// webhookMaxConcurrent caps the number of simultaneous HTTP deliveries.
	webhookMaxConcurrent = 20
	// webhookMaxAttempts is the number of delivery attempts before an event is
	// marked as permanently failed.
	webhookMaxAttempts = 5
	// webhookBaseBackoff is the base delay of the exponential retry backoff.
	webhookBaseBackoff = 15 * time.Second
	// webhookMaxBackoff caps the exponential retry backoff.
	webhookMaxBackoff = time.Hour
	// webhookJitterMax is the maximum random jitter added to each backoff.
	webhookJitterMax = 5 * time.Second
	// webhookProcessingTimeout is the lease held on a claimed event. When it
	// expires the reaper returns the event to the pending state.
	webhookProcessingTimeout = 2 * time.Minute
	// webhookMaxRedirects is the maximum number of redirects followed on delivery.
	webhookMaxRedirects = 3
	// webhookMaxResponseBytes is the maximum number of bytes read from a response
	// body (the body is discarded, but it must be drained to reuse the connection).
	webhookMaxResponseBytes = 64 << 10
	// webhookRequestTimeout is the total timeout of a single delivery attempt.
	webhookRequestTimeout = 10 * time.Second
)

// WebhookPayload represents an MS Graph API-style webhook event payload
type WebhookPayload struct {
	Value []WebhookEvent `json:"value"`
}

type WebhookEvent struct {
	SubscriptionID string                 `json:"subscriptionId"`
	ClientState    string                 `json:"clientState,omitempty"`
	ChangeType     string                 `json:"changeType"`
	Resource       string                 `json:"resource"`
	EventDateTime  time.Time              `json:"eventDateTime"`
	ResourceData   map[string]interface{} `json:"resourceData"`
}

// httpClient is the HTTP client used for webhook deliveries. It is hardened
// against SSRF: redirects are re-validated and every connection is dialled
// through a resolver that refuses private/loopback/link-local addresses.
var httpClient = &http.Client{
	Timeout: webhookRequestTimeout,
	Transport: telemetry.WrapTransport(&http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           safeDialContext,
		MaxIdleConns:          webhookMaxConcurrent,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}),
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= webhookMaxRedirects {
			return fmt.Errorf("stopped after %d redirects", webhookMaxRedirects)
		}
		// Every hop is validated again: a redirect must not be able to reach
		// an internal address that the original URL could not.
		if err := ValidateWebhookURL(req.URL.String()); err != nil {
			return fmt.Errorf("redirect target rejected: %w", err)
		}
		return nil
	},
}

var permService = permission.NewService()

// ---------------------------------------------------------------------------
// Outbox (enqueue)
// ---------------------------------------------------------------------------

func extractTxTraceparent(tx *gorm.DB) string {
	if tx == nil || tx.Statement == nil || tx.Statement.Context == nil {
		return ""
	}
	return telemetry.InjectTraceParent(tx.Statement.Context)
}

// EnqueueWebhookTx finds the webhooks registered for the given table/change type
// and stores one outbox event per matching webhook.
//
// The whole operation runs inside the caller supplied transaction so that the
// event is committed atomically with the data change that triggered it. This is
// the contract expected by the data layer: a rolled back transaction must not
// leave a webhook event behind, and a committed one must never be lost.
//
// Rows are filtered with the read permissions of the webhook creator and the
// payload only contains the columns that creator is allowed to read.
func EnqueueWebhookTx(tx *gorm.DB, workspaceID, tableID uuid.UUID, tableSlug, changeType string, recordData map[string]interface{}) error {
	if tx == nil {
		return errors.New("a transaction is required to enqueue webhook events")
	}

	var webhooks []metadata.Webhook
	if err := tx.Table("_hornero_webhooks").
		Where("workspace_id = ?", workspaceID).
		Where("resource = ?", tableID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Find(&webhooks).Error; err != nil {
		return fmt.Errorf("failed to query webhooks: %w", err)
	}

	if len(webhooks) == 0 {
		slog.Debug("no webhooks found for table", "table_slug", tableSlug, "workspace_id", workspaceID)
		return nil
	}

	// The workspace owner is needed to resolve the role of the webhook creator.
	var workspace metadata.Workspace
	if err := tx.Table("_hornero_workspaces").
		Select("id, owner_id").
		First(&workspace, "id = ?", workspaceID).Error; err != nil {
		// An inconsistent workspace must not break the write that triggered the
		// webhook: log it and skip the enqueue instead of failing the transaction.
		slog.Error("failed to fetch workspace for webhook dispatch",
			"workspace_id", workspaceID, "error", err)
		return nil
	}

	traceparent := extractTxTraceparent(tx)
	now := time.Now()
	events := make([]metadata.WebhookOutboxEvent, 0, len(webhooks))

	for _, wh := range webhooks {
		if !webhookMatchesChangeType(wh.ChangeType, changeType) {
			continue
		}

		roleName, isOwner := resolveCreatorRoleTx(tx, wh.CreatedBy, workspaceID, workspace.OwnerID.String())
		if roleName == "" {
			slog.Warn("could not resolve role for webhook creator", "webhook_id", wh.ID, "creator_id", wh.CreatedBy)
			continue
		}

		accessLevel, err := permService.CheckTableAccess(workspaceID, roleName, tableSlug, "read")
		if err != nil {
			slog.Error("failed to check webhook creator permissions",
				"webhook_id", wh.ID, "role", roleName, "table", tableSlug, "error", err)
			continue
		}
		if accessLevel == permission.AccessNone {
			slog.Debug("webhook skipped: creator lacks read access",
				"webhook_id", wh.ID, "role", roleName, "table", tableSlug)
			continue
		}

		// If the creator only has AccessOwn, only forward rows they created.
		if accessLevel == permission.AccessOwn && !isOwner {
			createdBy, ok := recordData["created_by"].(string)
			if !ok || createdBy != wh.CreatedBy {
				slog.Debug("webhook skipped: creator only has AccessOwn and is not the row owner",
					"webhook_id", wh.ID, "role", roleName)
				continue
			}
		}

		// Clone the row and strip every column the creator cannot read.
		dataClone := cloneData(recordData)
		allowedColumns, err := permService.GetColumnsForOperation(workspaceID, roleName, tableSlug, "read")
		if err != nil {
			slog.Error("failed to resolve allowed columns for webhook creator",
				"webhook_id", wh.ID, "role", roleName, "table", tableSlug, "error", err)
			continue
		}
		if allowedColumns != nil {
			filterRecordColumns(dataClone, allowedColumns)
		}

		payload, err := json.Marshal(WebhookPayload{Value: []WebhookEvent{{
			SubscriptionID: wh.ID.String(),
			ClientState:    wh.ClientState,
			ChangeType:     changeType,
			Resource:       tableID.String(),
			EventDateTime:  now.UTC(),
			ResourceData:   dataClone,
		}}})
		if err != nil {
			slog.Error("failed to marshal webhook payload", "webhook_id", wh.ID, "error", err)
			continue
		}

		events = append(events, metadata.WebhookOutboxEvent{
			ID:              uuid.New(),
			WebhookID:       wh.ID,
			NotificationURL: wh.NotificationURL,
			Payload:         metadata.JSON(payload),
			Status:          "pending",
			Attempts:        0,
			NextAttemptAt:   now,
			Traceparent:     traceparent,
		})
	}

	if len(events) == 0 {
		return nil
	}

	if err := tx.Table("_hornero_webhook_events").Create(&events).Error; err != nil {
		return fmt.Errorf("failed to create webhook outbox events: %w", err)
	}

	slog.Info("webhook events enqueued",
		"count", len(events), "workspace_id", workspaceID, "table_slug", tableSlug, "change_type", changeType)
	return nil
}

// DispatchWebhookAsync is the non-transactional wrapper around EnqueueWebhookTx.
// It is kept for callers that are not able to provide a transaction; the events
// are enqueued on the default connection.
func DispatchWebhookAsync(workspaceID, tableID uuid.UUID, tableSlug, changeType string, originalData map[string]interface{}) {
	slog.Info("starting webhook dispatcher async", "table_slug", tableSlug, "change_type", changeType)
	go func() {
		if err := EnqueueWebhookTx(database.DB, workspaceID, tableID, tableSlug, changeType, originalData); err != nil {
			slog.Error("failed to enqueue webhook events",
				"error", err, "workspace_id", workspaceID, "table_slug", tableSlug, "change_type", changeType)
		}
	}()
}

// webhookMatchesChangeType reports whether a webhook change_type expression
// matches the given change type.
//
// The expression is a comma separated list of exact tokens ("created,updated",
// "deleted" or "*"). Matching is done on whole tokens: a substring match would
// wrongly route "create" to a webhook registered for "created".
func webhookMatchesChangeType(expression, changeType string) bool {
	expr := strings.TrimSpace(expression)
	if expr == "" {
		return false
	}
	if expr == "*" {
		return true
	}
	for _, token := range strings.Split(expr, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == "*" {
			return true
		}
		if strings.EqualFold(token, changeType) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Processor
// ---------------------------------------------------------------------------

// StartWebhookProcessor polls the outbox and dispatches webhooks with retries.
func StartWebhookProcessor() {
	StartWebhookProcessorWithContext(context.Background())
}

// StartWebhookProcessorWithContext starts the outbox processor and stops it
// gracefully when ctx is cancelled: the ticker is stopped and the in-flight
// deliveries are awaited before returning, so no delivery is left half done.
func StartWebhookProcessorWithContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	slog.Info("starting webhook outbox processor")
	processor := newWebhookProcessor(ctx)
	go processor.run()
}

type webhookProcessor struct {
	ctx context.Context
	wg  sync.WaitGroup
	sem chan struct{}
}

func newWebhookProcessor(ctx context.Context) *webhookProcessor {
	return &webhookProcessor{
		ctx: ctx,
		sem: make(chan struct{}, webhookMaxConcurrent),
	}
}

func (p *webhookProcessor) run() {
	ticker := time.NewTicker(webhookPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			slog.Info("webhook outbox processor: graceful stop requested, waiting for in-flight deliveries")
			p.wg.Wait()
			slog.Info("webhook outbox processor: stopped")
			return
		case <-ticker.C:
			p.processOutboxEvents()
		}
	}
}

// processOutboxEvents reaps stuck events, claims a batch of pending events
// atomically and dispatches them.
func (p *webhookProcessor) processOutboxEvents() {
	p.reapStuckEvents()

	var events []metadata.WebhookOutboxEvent

	// Use a silent session to avoid polluting logs with polling queries when nothing is found.
	silentDB := database.DB.Session(&gorm.Session{Logger: database.DB.Logger.LogMode(logger.Silent)})
	err := silentDB.Transaction(func(tx *gorm.DB) error {
		// Claim the batch: SELECT ... FOR UPDATE SKIP LOCKED and the UPDATE that
		// flips the rows to "processing" happen in the SAME transaction, so two
		// replicas can never claim the same event.
		if err := tx.Raw(`
			SELECT * FROM _hornero_webhook_events
			WHERE status = ? AND next_attempt_at <= ?
			ORDER BY next_attempt_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED`,
			"pending", time.Now(), webhookBatchSize,
		).Scan(&events).Error; err != nil {
			return err
		}

		if len(events) == 0 {
			return nil
		}

		ids := make([]uuid.UUID, 0, len(events))
		for _, e := range events {
			ids = append(ids, e.ID)
		}

		deadline := time.Now().Add(webhookProcessingTimeout)
		res := tx.Table("_hornero_webhook_events").
			Where("id IN ? AND status = ?", ids, "pending").
			Updates(map[string]interface{}{
				"status":             "processing",
				"processing_timeout": deadline,
				"next_attempt_at":    time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != int64(len(events)) {
			return fmt.Errorf("claimed %d of %d webhook events", res.RowsAffected, len(events))
		}
		return nil
	})

	if err != nil {
		slog.Error("failed to claim webhook outbox events", "error", err)
		return
	}
	if len(events) == 0 {
		return
	}

	slog.Info("processing webhook outbox events", "count", len(events))

	// Resolve the client states (HMAC secrets) of the involved webhooks once.
	clientStates := p.loadClientStates(events)

	for _, event := range events {
		e := event
		state := clientStates[e.WebhookID]
		p.wg.Add(1)
		go p.deliver(e, state)
	}
}

// reapStuckEvents returns events whose processing lease expired to the pending
// state. Without this, a crash of the replica holding the event would strand
// the delivery forever.
func (p *webhookProcessor) reapStuckEvents() {
	res := database.DB.Exec(`
		UPDATE _hornero_webhook_events
		SET status = 'pending', processing_timeout = NULL
		WHERE status = 'processing' AND processing_timeout IS NOT NULL AND processing_timeout < NOW()`)
	if res.Error != nil {
		slog.Error("failed to reap stuck webhook events", "error", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		slog.Warn("requeued stuck webhook events", "count", res.RowsAffected)
	}
}

// loadClientStates returns the HMAC secret of every webhook involved in the
// batch, indexed by webhook id.
func (p *webhookProcessor) loadClientStates(events []metadata.WebhookOutboxEvent) map[uuid.UUID]string {
	states := make(map[uuid.UUID]string)
	ids := make([]uuid.UUID, 0, len(events))
	seen := make(map[uuid.UUID]bool, len(events))
	for _, e := range events {
		if !seen[e.WebhookID] {
			seen[e.WebhookID] = true
			ids = append(ids, e.WebhookID)
		}
	}
	if len(ids) == 0 {
		return states
	}

	type webhookSecret struct {
		ID          uuid.UUID `gorm:"column:id"`
		ClientState string    `gorm:"column:client_state"`
	}
	var rows []webhookSecret
	if err := database.DB.Table("_hornero_webhooks").
		Select("id, client_state").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		slog.Error("failed to load webhook client states", "error", err)
		return states
	}
	for _, r := range rows {
		states[r.ID] = r.ClientState
	}
	return states
}

// deliver performs a single delivery attempt and updates the outbox row with
// the outcome.
func (p *webhookProcessor) deliver(e metadata.WebhookOutboxEvent, clientState string) {
	defer p.wg.Done()

	// Bounded concurrency: at most webhookMaxConcurrent deliveries in flight.
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-p.ctx.Done():
		p.releaseEvent(e)
		return
	}

	statusCode, err := sendWebhookPayloadWithContext(p.ctx, e, clientState)
	now := time.Now()

	if err == nil {
		database.DB.Table("_hornero_webhook_events").Where("id = ?", e.ID).Updates(map[string]interface{}{
			"status":             "completed",
			"completed_at":       now,
			"processing_timeout": nil,
			"last_status_code":   statusCode,
			"last_attempt_at":    now,
			"last_error":         "",
		})
		return
	}

	attempts := e.Attempts + 1
	lastError := err.Error()
	if len(lastError) > 1000 {
		lastError = lastError[:1000]
	}

	slog.Warn("webhook delivery failed",
		"event_id", e.ID, "webhook_id", e.WebhookID, "attempts", attempts,
		"status_code", statusCode, "error", lastError)

	if attempts >= webhookMaxAttempts {
		database.DB.Table("_hornero_webhook_events").Where("id = ?", e.ID).Updates(map[string]interface{}{
			"status":             "failed",
			"attempts":           attempts,
			"failed_at":          now,
			"processing_timeout": nil,
			"last_status_code":   statusCode,
			"last_attempt_at":    now,
			"last_error":         lastError,
		})
		return
	}

	delay := computeBackoff(attempts)
	database.DB.Table("_hornero_webhook_events").Where("id = ?", e.ID).Updates(map[string]interface{}{
		"status":             "pending",
		"attempts":           attempts,
		"next_attempt_at":    now.Add(delay),
		"processing_timeout": nil,
		"last_status_code":   statusCode,
		"last_attempt_at":    now,
		"last_error":         lastError,
	})
}

// releaseEvent puts a claimed event back to pending (used when the processor
// is shutting down before the delivery started).
func (p *webhookProcessor) releaseEvent(e metadata.WebhookOutboxEvent) {
	database.DB.Table("_hornero_webhook_events").
		Where("id = ? AND status = ?", e.ID, "processing").
		Updates(map[string]interface{}{
			"status":             "pending",
			"processing_timeout": nil,
			"next_attempt_at":    time.Now(),
		})
}

// computeBackoff returns the retry delay for the given attempt number:
// 15s * 2^(attempt-1) capped at 1h, plus a random jitter of up to 5s.
func computeBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exp := attempt - 1
	if exp > 20 {
		exp = 20
	}
	delay := webhookBaseBackoff * time.Duration(int64(1)<<exp)
	if delay <= 0 || delay > webhookMaxBackoff {
		delay = webhookMaxBackoff
	}
	return delay + randomJitter(webhookJitterMax)
}

func randomJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}

// ---------------------------------------------------------------------------
// Delivery
// ---------------------------------------------------------------------------

// signWebhookPayload computes the signature header value of a delivery.
//
// The signed message is "<unix_timestamp>.<raw_payload>" and the key is the
// webhook client state. The header format is "t=<unix>,v1=<hex-digest>".
func signWebhookPayload(clientState string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(clientState))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, payload)))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

// sendWebhookPayload performs the HTTP delivery of a single outbox event.
// It returns the HTTP status code (0 when the request never reached the
// target) and the error, if any.
func sendWebhookPayload(e metadata.WebhookOutboxEvent, clientState string) (int, error) {
	return sendWebhookPayloadWithContext(context.Background(), e, clientState)
}

func sendWebhookPayloadWithContext(ctx context.Context, e metadata.WebhookOutboxEvent, clientState string) (int, error) {
	// Defense in depth: the target is validated again right before the
	// delivery, because the URL could have been changed since it was queued.
	if err := ValidateWebhookURL(e.NotificationURL); err != nil {
		return 0, fmt.Errorf("webhook target rejected: %w", err)
	}

	return deliverWebhookHTTP(ctx, httpClient, e, clientState)
}

func startWebhookDeliverySpan(ctx context.Context, e metadata.WebhookOutboxEvent) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}

	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("webhook.id", e.WebhookID.String()),
			attribute.Int("webhook.attempt", e.Attempts+1),
		),
	}

	if e.Traceparent != "" {
		if _, linkSC := telemetry.ExtractTraceContext(ctx, e.Traceparent); linkSC.IsValid() {
			opts = append(opts, trace.WithLinks(trace.Link{SpanContext: linkSC}))
		}
	}

	return telemetry.Tracer("hornerodb/webhooks").Start(ctx, "webhook.deliver", opts...)
}

func deliverWebhookHTTP(ctx context.Context, client *http.Client, e metadata.WebhookOutboxEvent, clientState string) (int, error) {
	spanCtx, span := startWebhookDeliverySpan(ctx, e)
	defer span.End()

	payload := []byte(e.Payload)
	req, err := http.NewRequestWithContext(spanCtx, http.MethodPost, e.NotificationURL, bytes.NewReader(payload))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("failed to build webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "HorneroDB-Webhooks/1.0")
	// Every delivery carries a unique id so the receiver can deduplicate.
	req.Header.Set("X-Hornero-Delivery", e.ID.String())
	if clientState != "" {
		req.Header.Set("X-Hornero-Signature", signWebhookPayload(clientState, time.Now().Unix(), payload))
	}

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("failed to deliver webhook: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// Drain (bounded) the body so the connection can be reused, but never read
	// more than a small amount of data from an untrusted peer.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, webhookMaxResponseBytes))

	if resp.StatusCode >= 400 {
		err := fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp.StatusCode, err
	}

	return resp.StatusCode, nil
}

// ---------------------------------------------------------------------------
// SSRF protection
// ---------------------------------------------------------------------------

// blockedRanges lists the network ranges a webhook target must never resolve
// to: private, loopback, link-local (cloud metadata), CGNAT, benchmarking and
// other reserved ranges.
var blockedRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this" network / unspecified
	netip.MustParsePrefix("10.0.0.0/8"),      // private
	netip.MustParsePrefix("100.64.0.0/10"),   // CGNAT (RFC 6598)
	netip.MustParsePrefix("127.0.0.0/8"),     // loopback
	netip.MustParsePrefix("169.254.0.0/16"),  // link-local (cloud metadata)
	netip.MustParsePrefix("172.16.0.0/12"),   // private
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
	netip.MustParsePrefix("192.88.99.0/24"),  // 6to4 relay anycast
	netip.MustParsePrefix("192.168.0.0/16"),  // private
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking (RFC 2544)
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("224.0.0.0/4"),     // multicast
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved (includes broadcast)
	netip.MustParsePrefix("::/128"),          // unspecified
	netip.MustParsePrefix("::1/128"),         // loopback
	netip.MustParsePrefix("100::/64"),        // discard-only
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
	netip.MustParsePrefix("2002::/16"),       // 6to4
	netip.MustParsePrefix("64:ff9b::/96"),    // NAT64
	netip.MustParsePrefix("fc00::/7"),        // unique local
	netip.MustParsePrefix("fe80::/10"),       // link-local
	netip.MustParsePrefix("ff00::/8"),        // multicast
}

// isBlockedAddr reports whether an address belongs to a range that must never
// be reached by a webhook delivery.
//
// IPv4-mapped IPv6 addresses are unmapped first, so "::ffff:169.254.169.254"
// is rejected exactly like "169.254.169.254".
func isBlockedAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap()

	if addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() {
		return true
	}

	for _, p := range blockedRanges {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// isProduction reports whether the instance runs in production mode.
func isProduction() bool {
	return strings.EqualFold(os.Getenv("HORNERO_ENV"), "production")
}

// ValidateWebhookURL validates a webhook notification URL against SSRF.
//
// Only http/https are accepted (https only in production). The host is
// resolved and every resulting address is checked against the blocked ranges,
// so a hostname pointing to 127.0.0.1 or to the cloud metadata address
// 169.254.169.254 is rejected.
func ValidateWebhookURL(rawURL string) error {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return errors.New("notification URL is empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid notification URL: %w", err)
	}

	switch parsed.Scheme {
	case "https":
		// always allowed
	case "http":
		if isProduction() {
			return errors.New("http notification URLs are not allowed in production, use https")
		}
	default:
		return fmt.Errorf("unsupported notification URL scheme %q: only http and https are allowed", parsed.Scheme)
	}

	if parsed.User != nil {
		return errors.New("notification URL must not contain credentials")
	}

	host := parsed.Hostname()
	if host == "" {
		return errors.New("invalid notification URL: missing host")
	}

	// Literal IP: no DNS resolution needed.
	if addr, err := netip.ParseAddr(host); err == nil {
		if isBlockedAddr(addr) {
			return fmt.Errorf("notification URL host %s is not allowed", addr)
		}
		return nil
	}

	// Hostname: every resolved address must be publicly routable.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("hostname %q did not resolve to any address", host)
	}
	for _, addr := range addrs {
		if isBlockedAddr(addr) {
			return fmt.Errorf("hostname %q resolves to a blocked address (%s)", host, addr)
		}
	}
	return nil
}

// resolvePublicIPs resolves a host and returns only the addresses that are not
// blocked. It is used by the custom dialer, so that a DNS answer that changed
// between the validation and the connection (DNS rebinding) cannot reach an
// internal address.
func resolvePublicIPs(ctx context.Context, host string) ([]netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		if isBlockedAddr(addr) {
			return nil, fmt.Errorf("address %s is not allowed", addr)
		}
		return []netip.Addr{addr}, nil
	}

	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname %q: %w", host, err)
	}

	public := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		if isBlockedAddr(addr) {
			continue
		}
		public = append(public, addr)
	}
	if len(public) == 0 {
		return nil, fmt.Errorf("hostname %q does not resolve to a public address", host)
	}
	return public, nil
}

// safeDialContext is the DialContext of the webhook HTTP client. It resolves
// the target host and refuses to connect to blocked addresses.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	// A configured proxy may legitimately live on a private address.
	if isProxyHost(host) {
		dialer := &net.Dialer{Timeout: webhookRequestTimeout, KeepAlive: 30 * time.Second}
		return dialer.DialContext(ctx, network, addr)
	}

	ips, err := resolvePublicIPs(ctx, host)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: webhookRequestTimeout, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable address for %q", host)
	}
	return nil, lastErr
}

// isProxyHost reports whether the host is the configured HTTP proxy.
func isProxyHost(host string) bool {
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		raw := os.Getenv(key)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if u.Hostname() != "" && strings.EqualFold(u.Hostname(), host) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func cloneData(original map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{}, len(original))
	for k, v := range original {
		clone[k] = v
	}
	return clone
}

func filterRecordColumns(record map[string]interface{}, allowedColumns []string) {
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return
		}
		allowedMap[col] = true
	}

	allowedMap["id"] = true
	allowedMap["created_at"] = true
	allowedMap["updated_at"] = true
	allowedMap["created_by"] = true

	for key := range record {
		if !allowedMap[key] {
			delete(record, key)
		}
	}
}

// resolveCreatorRole resolves the role of the webhook creator inside the
// workspace. It uses the default connection.
func resolveCreatorRole(creatorID string, workspaceID uuid.UUID, ownerID string) (roleName string, isOwner bool) {
	return resolveCreatorRoleTx(database.DB, creatorID, workspaceID, ownerID)
}

// resolveCreatorRoleTx resolves the role of the webhook creator inside the
// workspace using the supplied transaction (or connection).
func resolveCreatorRoleTx(tx *gorm.DB, creatorID string, workspaceID uuid.UUID, ownerID string) (roleName string, isOwner bool) {
	if tx == nil {
		tx = database.DB
	}
	if creatorID == "" {
		return "", false
	}
	if creatorID == ownerID {
		return "admin", true
	}

	// Maybe it's a User
	var userRole metadata.UserRole
	err := tx.Table("_hornero_user_roles").
		Where("workspace_id = ? AND user_id = ?", workspaceID, creatorID).
		First(&userRole).Error

	if err == nil {
		var role metadata.Role
		if tx.Table("_hornero_roles").First(&role, "id = ?", userRole.RoleID).Error == nil {
			return role.Name, false
		}
	}

	// Maybe it's an API Key
	var apiKey metadata.APIKey
	err = tx.Table("_hornero_api_keys").
		Where("workspace_id = ? AND id = ?", workspaceID, creatorID).
		First(&apiKey).Error

	if err == nil && apiKey.RoleID != uuid.Nil {
		var role metadata.Role
		if tx.Table("_hornero_roles").First(&role, "id = ?", apiKey.RoleID).Error == nil {
			return role.Name, false
		}
	}

	slog.Warn("could not resolve role for creator", "creator_id", creatorID, "workspace_id", workspaceID)
	return "", false
}
