package {{PACKAGE_NAME}}

import "testing"

func Test{{FUNCTION_NAME}}(t *testing.T) {
	// TODO: implement test for {{FUNCTION_NAME}}
	got := {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
	want := {{EXPECTED_OUTPUT}}
	if got != want {
		t.Errorf("{{FUNCTION_NAME}}() = %v, want %v", got, want)
	}
}
