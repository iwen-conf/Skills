package {{PACKAGE_NAME}}

import "testing"

func Test{{FUNCTION_NAME}}Table(t *testing.T) {
	tests := []struct {
		name string
		input {{INPUT_TYPE}}
		want  {{OUTPUT_TYPE}}
	}{
		{"zero value", {{ZERO_VALUE}}, {{ZERO_EXPECTED}}},
		{"nominal", {{NOMINAL_VALUE}}, {{NOMINAL_EXPECTED}}},
		{"boundary min", {{MIN_VALUE}}, {{MIN_EXPECTED}}},
		{"boundary max", {{MAX_VALUE}}, {{MAX_EXPECTED}}},
		// FILL_IN: add equivalence partitioning cases
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
