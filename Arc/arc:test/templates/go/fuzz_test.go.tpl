package {{PACKAGE_NAME}}

import "testing"

func Fuzz{{FUNCTION_NAME}}(f *testing.F) {
	f.Add({{SEED_CORPUS}})
	f.Fuzz(func(t *testing.T, {{FUZZ_PARAMS}}) {
		// TODO: call {{FUNCTION_NAME}} and check invariants
		result := {{FUNCTION_NAME}}({{FUZZ_ARGS}})
		_ = result
	})
}
