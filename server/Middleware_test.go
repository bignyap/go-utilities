package server

import (
	"testing"
)

func TestRedactSensitiveQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "redact token parameter",
			input:    "token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9&foo=bar",
			expected: "foo=bar&token=%5BREDACTED%5D",
		},
		{
			name:     "redact api_key parameter",
			input:    "api_key=secret123&limit=10",
			expected: "api_key=%5BREDACTED%5D&limit=10",
		},
		{
			name:     "redact password parameter",
			input:    "username=john&password=secret&email=test@example.com",
			expected: "email=test%40example.com&password=%5BREDACTED%5D&username=john",
		},
		{
			name:     "redact multiple sensitive params",
			input:    "token=abc123&api_key=xyz789&user=john",
			expected: "api_key=%5BREDACTED%5D&token=%5BREDACTED%5D&user=john",
		},
		{
			name:     "no sensitive params",
			input:    "limit=10&offset=20&sort=name",
			expected: "limit=10&offset=20&sort=name",
		},
		{
			name:     "empty query string",
			input:    "",
			expected: "",
		},
		{
			name:     "case insensitive matching",
			input:    "TOKEN=abc&API_KEY=xyz",
			expected: "API_KEY=%5BREDACTED%5D&TOKEN=%5BREDACTED%5D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveQueryParams(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveQueryParams() = %v, want %v", result, tt.expected)
			}
		})
	}
}

