//go:build integration

package {{PACKAGE_NAME}}

import (
	"testing"
)

// TestMain provides setup/teardown for integration tests.
// Run with: go test -tags=integration -run TestIntegration ./...
func TestMain(m *testing.M) {
	// TODO: setup — initialize dependencies (DB, HTTP client, etc.)
	// code := m.Run()
	// TODO: teardown — close connections, clean state
	// os.Exit(code)
	m.Run()
}

func TestIntegration{{FUNCTION_NAME_PASCAL}}(t *testing.T) {
	tests := []struct {
		name    string
		input   {{INPUT_TYPE}}
		want    {{OUTPUT_TYPE}}
		wantErr bool
	}{
		{"cross-module nominal", {{NOMINAL_VALUE}}, {{NOMINAL_EXPECTED}}, false},
		{"dependency error path", {{INVALID_INPUT}}, {{ZERO_EXPECTED}}, true},
		// TODO: add cases that exercise real module interactions
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := {{FUNCTION_NAME}}(tt.input)
			if got != tt.want {
				t.Errorf("{{FUNCTION_NAME}}(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
