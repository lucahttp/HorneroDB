package permission

import (
	"reflect"
	"testing"
)

func TestGetAccessLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected AccessLevel
	}{
		{"nil value", nil, AccessNone},
		{"string own", "own", AccessOwn},
		{"string all", "all", AccessAll},
		{"string none", "none", AccessNone},
		{"string inherit", "inherit", AccessInherit},
		{"string invalid", "invalid", AccessNone},
		{"empty string", "", AccessNone},
		{"int value", 1, AccessNone},
		{"bool value", true, AccessNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAccessLevel(tt.input)
			if got != tt.expected {
				t.Errorf("getAccessLevel(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetSystemAccessLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected AccessLevel
	}{
		{"nil value", nil, AccessNone},
		{"string none", "none", AccessNone},
		{"string view", "view", AccessView},
		{"string manage", "manage", AccessManage},
		{"string all", "all", AccessAll},
		{"string own is not a system level", "own", AccessNone},
		{"string inherit is not a system level", "inherit", AccessNone},
		{"string invalid", "invalid", AccessNone},
		{"empty string", "", AccessNone},
		{"int value", 1, AccessNone},
		{"bool value", true, AccessNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSystemAccessLevel(tt.input)
			if got != tt.expected {
				t.Errorf("getSystemAccessLevel(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSystemAccessRankOrdering(t *testing.T) {
	if !(systemAccessRank(AccessNone) < systemAccessRank(AccessView) &&
		systemAccessRank(AccessView) < systemAccessRank(AccessManage) &&
		systemAccessRank(AccessManage) < systemAccessRank(AccessAll)) {
		t.Fatalf("system access levels are not ordered none < view < manage < all: %d %d %d %d",
			systemAccessRank(AccessNone),
			systemAccessRank(AccessView),
			systemAccessRank(AccessManage),
			systemAccessRank(AccessAll),
		)
	}

	if systemAccessRank(AccessNone) != 0 {
		t.Errorf("systemAccessRank(AccessNone) = %d, want 0", systemAccessRank(AccessNone))
	}
	if systemAccessRank(AccessView) != 1 {
		t.Errorf("systemAccessRank(AccessView) = %d, want 1", systemAccessRank(AccessView))
	}
	if systemAccessRank(AccessManage) != 2 {
		t.Errorf("systemAccessRank(AccessManage) = %d, want 2", systemAccessRank(AccessManage))
	}
	if systemAccessRank(AccessAll) != 3 {
		t.Errorf("systemAccessRank(AccessAll) = %d, want 3", systemAccessRank(AccessAll))
	}
}

func TestGetColumns(t *testing.T) {
	t.Run("nil means no restriction", func(t *testing.T) {
		got := getColumns(nil)
		if got != nil {
			t.Errorf("getColumns(nil) = %v, want nil", got)
		}
	})

	t.Run("empty array means zero columns", func(t *testing.T) {
		got := getColumns([]interface{}{})
		if got == nil {
			t.Fatal("getColumns([]interface{}{}) = nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Errorf("getColumns([]interface{}{}) = %v, want empty slice", got)
		}
	})

	t.Run("string array", func(t *testing.T) {
		got := getColumns([]interface{}{"name", "email"})
		want := []string{"name", "email"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("getColumns([name email]) = %v, want %v", got, want)
		}
	})

	t.Run("mixed array skips non strings", func(t *testing.T) {
		got := getColumns([]interface{}{"name", 123})
		want := []string{"name"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("getColumns([name 123]) = %v, want %v", got, want)
		}
	})

	t.Run("not an array", func(t *testing.T) {
		got := getColumns("string")
		if got != nil {
			t.Errorf("getColumns(\"string\") = %v, want nil", got)
		}
	})
}

func TestResolveOperationLevel(t *testing.T) {
	tests := []struct {
		name        string
		permObj     map[string]interface{}
		operation   string
		wantLevel   AccessLevel
		wantApplies bool
	}{
		{"read own", map[string]interface{}{"read": "own"}, "read", AccessOwn, true},
		{"read all", map[string]interface{}{"read": "all"}, "read", AccessAll, true},
		{"read none", map[string]interface{}{"read": "none"}, "read", AccessNone, true},
		{"inherit does not apply", map[string]interface{}{"read": "inherit"}, "read", AccessNone, false},
		{"empty value does not apply", map[string]interface{}{"read": ""}, "read", AccessNone, false},
		{"blank value does not apply", map[string]interface{}{"read": "   "}, "read", AccessNone, false},
		{"missing operation denies", map[string]interface{}{"read": "own"}, "delete", AccessNone, true},
		{"null value denies", map[string]interface{}{"read": nil}, "delete", AccessNone, true},
		{"unknown operation does not apply", map[string]interface{}{"read": "own"}, "truncate", AccessNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, applies := resolveOperationLevel(tt.permObj, tt.operation)
			if level != tt.wantLevel {
				t.Errorf("resolveOperationLevel(%v, %q) level = %v, want %v", tt.permObj, tt.operation, level, tt.wantLevel)
			}
			if applies != tt.wantApplies {
				t.Errorf("resolveOperationLevel(%v, %q) applies = %v, want %v", tt.permObj, tt.operation, applies, tt.wantApplies)
			}
		})
	}
}

func TestResolveTableAccessPrecedence(t *testing.T) {
	permMap := map[string]interface{}{
		"*": map[string]interface{}{
			"create": "all",
			"read":   "all",
			"update": "all",
			"delete": "all",
		},
		"orders": map[string]interface{}{
			"read":   "own",
			"update": "inherit",
		},
		"empty_rule": map[string]interface{}{
			"read": "",
		},
	}

	tests := []struct {
		name      string
		permMap   map[string]interface{}
		tableSlug string
		operation string
		expected  AccessLevel
	}{
		{
			name:      "table rule wins over wildcard",
			permMap:   permMap,
			tableSlug: "orders",
			operation: "read",
			expected:  AccessOwn,
		},
		{
			name:      "inherit falls back to wildcard",
			permMap:   permMap,
			tableSlug: "orders",
			operation: "update",
			expected:  AccessAll,
		},
		{
			name:      "empty value falls back to wildcard",
			permMap:   permMap,
			tableSlug: "empty_rule",
			operation: "read",
			expected:  AccessAll,
		},
		{
			name:      "table rule without the operation denies",
			permMap:   permMap,
			tableSlug: "orders",
			operation: "create",
			expected:  AccessNone,
		},
		{
			name:      "table rule without the operation denies delete",
			permMap:   permMap,
			tableSlug: "orders",
			operation: "delete",
			expected:  AccessNone,
		},
		{
			name:      "unknown table uses wildcard",
			permMap:   permMap,
			tableSlug: "products",
			operation: "read",
			expected:  AccessAll,
		},
		{
			name:      "inherit without wildcard denies",
			permMap:   map[string]interface{}{"orders": map[string]interface{}{"read": "inherit"}},
			tableSlug: "orders",
			operation: "read",
			expected:  AccessNone,
		},
		{
			name:      "no rules at all denies",
			permMap:   map[string]interface{}{},
			tableSlug: "orders",
			operation: "read",
			expected:  AccessNone,
		},
		{
			name:      "nil permission map denies",
			permMap:   nil,
			tableSlug: "orders",
			operation: "read",
			expected:  AccessNone,
		},
		{
			name:      "system key is not a table rule",
			permMap:   map[string]interface{}{SystemAccessKey: map[string]interface{}{"read": "all"}},
			tableSlug: "orders",
			operation: "read",
			expected:  AccessNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTableAccess(tt.permMap, tt.tableSlug, tt.operation)
			if got != tt.expected {
				t.Errorf("resolveTableAccess(%v, %q, %q) = %v, want %v",
					tt.permMap, tt.tableSlug, tt.operation, got, tt.expected)
			}
		})
	}
}

func TestResolveColumnsPrecedence(t *testing.T) {
	permMap := map[string]interface{}{
		"*": map[string]interface{}{
			"columns": map[string]interface{}{
				"read": []interface{}{"id", "title"},
			},
		},
		"orders": map[string]interface{}{
			"columns": map[string]interface{}{
				"read": []interface{}{"id"},
			},
		},
		"locked": map[string]interface{}{
			"columns": map[string]interface{}{
				"read": []interface{}{},
			},
		},
		"no_columns": map[string]interface{}{
			"read": "own",
		},
	}

	t.Run("table rule wins over wildcard", func(t *testing.T) {
		got := resolveColumns(permMap, "orders", "read")
		want := []string{"id"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("resolveColumns(orders, read) = %v, want %v", got, want)
		}
	})

	t.Run("explicit empty list means zero columns", func(t *testing.T) {
		got := resolveColumns(permMap, "locked", "read")
		if got == nil {
			t.Fatal("resolveColumns(locked, read) = nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Errorf("resolveColumns(locked, read) = %v, want empty slice", got)
		}
	})

	t.Run("table without column restriction falls back to wildcard", func(t *testing.T) {
		got := resolveColumns(permMap, "no_columns", "read")
		want := []string{"id", "title"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("resolveColumns(no_columns, read) = %v, want %v", got, want)
		}
	})

	t.Run("unknown table uses wildcard", func(t *testing.T) {
		got := resolveColumns(permMap, "products", "read")
		want := []string{"id", "title"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("resolveColumns(products, read) = %v, want %v", got, want)
		}
	})

	t.Run("operation without restriction returns nil", func(t *testing.T) {
		got := resolveColumns(permMap, "orders", "update")
		if got != nil {
			t.Errorf("resolveColumns(orders, update) = %v, want nil", got)
		}
	})

	t.Run("no rules at all returns nil", func(t *testing.T) {
		got := resolveColumns(map[string]interface{}{}, "orders", "read")
		if got != nil {
			t.Errorf("resolveColumns(empty, read) = %v, want nil", got)
		}
	})
}

func TestHasAccess(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name     string
		access   AccessLevel
		isOwner  bool
		expected bool
	}{
		{"all access", AccessAll, false, true},
		{"all access owner", AccessAll, true, true},
		{"own access owner", AccessOwn, true, true},
		{"own access not owner", AccessOwn, false, false},
		{"none access", AccessNone, true, false},
		{"none access false", AccessNone, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.HasAccess(tt.access, tt.isOwner)
			if got != tt.expected {
				t.Errorf("HasAccess(%v, %v) = %v, want %v", tt.access, tt.isOwner, got, tt.expected)
			}
		})
	}
}
