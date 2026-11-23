package audit

import (
	"testing"

	"hornerodb/internal/database"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// IsSensitiveKey / SanitizeMetadata
// ---------------------------------------------------------------------------

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"secret",
		"client_secret",
		"password",
		"api_password",
		"token",
		"access_token",
		"refresh_token",
		"key_hash",
		"KEY_HASH",
		"Api_Key_Hash",
	}
	for _, key := range sensitive {
		assert.True(t, IsSensitiveKey(key), "expected %q to be sensitive", key)
	}

	safe := []string{
		"name",
		"prefix",
		"role_id",
		"notification_url",
		"change_type",
		"self_service",
		"attempts",
	}
	for _, key := range safe {
		assert.False(t, IsSensitiveKey(key), "expected %q to be safe", key)
	}
}

func TestSanitizeMetadata_DropsSensitiveKeys(t *testing.T) {
	meta := map[string]interface{}{
		"name":        "my-key",
		"secret":      "super-secret",
		"password":    "hunter2",
		"token":       "abc123",
		"key_hash":    "deadbeef",
		"clientState": "should-stay",
	}

	result := SanitizeMetadata(meta)

	assert.Equal(t, "my-key", result["name"])
	assert.Equal(t, "should-stay", result["clientState"])
	assert.NotContains(t, result, "secret")
	assert.NotContains(t, result, "password")
	assert.NotContains(t, result, "token")
	assert.NotContains(t, result, "key_hash")
	assert.Len(t, result, 2)
}

func TestSanitizeMetadata_NestedContainers(t *testing.T) {
	meta := map[string]interface{}{
		"user": map[string]interface{}{
			"id":     "user-1",
			"secret": "leak",
		},
		"items": []interface{}{
			map[string]interface{}{
				"name":      "item",
				"api_token": "leak",
			},
		},
	}

	result := SanitizeMetadata(meta)

	user, ok := result["user"].(map[string]interface{})
	require.True(t, ok, "nested map must stay a map")
	assert.Equal(t, "user-1", user["id"])
	assert.NotContains(t, user, "secret")

	items, ok := result["items"].([]interface{})
	require.True(t, ok, "nested slice must stay a slice")
	require.Len(t, items, 1)

	item, ok := items[0].(map[string]interface{})
	require.True(t, ok, "nested map inside slice must stay a map")
	assert.Equal(t, "item", item["name"])
	assert.NotContains(t, item, "api_token")
}

func TestSanitizeMetadata_NilAndEmpty(t *testing.T) {
	assert.NotNil(t, SanitizeMetadata(nil))
	assert.Empty(t, SanitizeMetadata(nil))

	assert.Empty(t, SanitizeMetadata(map[string]interface{}{}))
}

func TestSanitizeMetadata_DoesNotMutateInput(t *testing.T) {
	meta := map[string]interface{}{
		"token":  "leak",
		"amount": 10,
	}

	SanitizeMetadata(meta)

	assert.Contains(t, meta, "token", "the original map must not be mutated")
	assert.Contains(t, meta, "amount")
}

// ---------------------------------------------------------------------------
// Log
// ---------------------------------------------------------------------------

func TestLog_WithoutDatabase_DoesNotPanic(t *testing.T) {
	original := database.DB
	database.DB = nil
	defer func() { database.DB = original }()

	wsID := uuid.New()

	assert.NotPanics(t, func() {
		Log(Entry{
			WorkspaceID:  &wsID,
			ActorID:      "user-1",
			ActorEmail:   "user@example.com",
			AuthSource:   "oidc",
			Action:       "api_key.create",
			ResourceType: "api_key",
			ResourceID:   "key-1",
			Metadata:     map[string]interface{}{"name": "test", "token": "leak"},
			IPAddress:    "127.0.0.1",
		})
	})
}

func TestLog_WithoutDatabase_EmptyEntry(t *testing.T) {
	original := database.DB
	database.DB = nil
	defer func() { database.DB = original }()

	assert.NotPanics(t, func() {
		Log(Entry{})
	})
}

func TestLog_WithoutDatabase_InvalidMetadataValue(t *testing.T) {
	original := database.DB
	database.DB = nil
	defer func() { database.DB = original }()

	// A value that cannot be marshalled must not blow up the caller.
	assert.NotPanics(t, func() {
		Log(Entry{
			Action:   "test.action",
			Metadata: map[string]interface{}{"broken": make(chan int)},
		})
	})
}

// ---------------------------------------------------------------------------
// Entry construction
// ---------------------------------------------------------------------------

func TestEntry_Construction(t *testing.T) {
	wsID := uuid.New()

	entry := Entry{
		WorkspaceID:  &wsID,
		ActorID:      "user-1",
		ActorEmail:   "user@example.com",
		AuthSource:   "apikey",
		Action:       "webhook_event.retry",
		ResourceType: "webhook_event",
		ResourceID:   "event-1",
		Metadata:     map[string]interface{}{"attempts": 3},
		IPAddress:    "10.0.0.1",
	}

	require.NotNil(t, entry.WorkspaceID)
	assert.Equal(t, wsID, *entry.WorkspaceID)
	assert.Equal(t, "user-1", entry.ActorID)
	assert.Equal(t, "user@example.com", entry.ActorEmail)
	assert.Equal(t, "apikey", entry.AuthSource)
	assert.Equal(t, "webhook_event.retry", entry.Action)
	assert.Equal(t, "webhook_event", entry.ResourceType)
	assert.Equal(t, "event-1", entry.ResourceID)
	assert.Equal(t, "10.0.0.1", entry.IPAddress)
	assert.Equal(t, 3, entry.Metadata["attempts"])
}

func TestEntry_NilWorkspaceIsAllowed(t *testing.T) {
	entry := Entry{
		Action:       "system.action",
		ResourceType: "system",
	}

	assert.Nil(t, entry.WorkspaceID)
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_WithoutDatabase_ReturnsError(t *testing.T) {
	original := database.DB
	database.DB = nil
	defer func() { database.DB = original }()

	logs, total, err := List(uuid.New(), Filter{}, 10, 0)

	require.Error(t, err)
	assert.Nil(t, logs)
	assert.Zero(t, total)
}

func TestFilter_Fields(t *testing.T) {
	filter := Filter{
		Action:       "api_key.create",
		ResourceType: "api_key",
		ActorID:      "user-1",
	}

	assert.Equal(t, "api_key.create", filter.Action)
	assert.Equal(t, "api_key", filter.ResourceType)
	assert.Equal(t, "user-1", filter.ActorID)
}
