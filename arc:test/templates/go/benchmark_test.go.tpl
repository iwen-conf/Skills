package {{PACKAGE_NAME}}

import "testing"

func Benchmark{{FUNCTION_NAME}}(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		{{FUNCTION_NAME}}({{SAMPLE_INPUT}})
	}
}
