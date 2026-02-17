package permission

import (
	"testing"
)

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "id", true},
		{"valid with underscore", "created_by", true},
		{"valid alphanumeric", "user123", true},
		{"valid mixed", "UserName", true},
		{"invalid with dash", "user-id", false},
		{"invalid with dot", "user.name", false},
		{"invalid with space", "user name", false},
		{"empty", "", false},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

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
		{"string invalid", "invalid", AccessNone},
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

func TestGetColumns(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{"nil", nil, nil},
		{"empty array", []interface{}{}, []string{}},
		{"string array", []interface{}{"name", "email"}, []string{"name", "email"}},
		{"mixed array", []interface{}{"name", 123}, []string{"name", ""}},
		{"not an array", "string", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getColumns(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("getColumns(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
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
