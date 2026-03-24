//! Integration tests for {{CRATE_NAME}}.
//!
//! Rust convention: files under `tests/` are integration tests that
//! access only the public API of the crate.

use {{CRATE_NAME}}::{{FUNCTION_NAME}};

#[test]
fn integration_{{FUNCTION_NAME}}_nominal() {
    let result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}});
    assert_eq!(result, {{EXPECTED_OUTPUT}});
}

#[test]
fn integration_{{FUNCTION_NAME}}_cross_module() {
    // TODO: test interaction between multiple public modules
    let result = {{FUNCTION_NAME}}({{NOMINAL_VALUE}});
    assert_eq!(result, {{NOMINAL_EXPECTED}});
}

#[test]
fn integration_{{FUNCTION_NAME}}_error_propagation() {
    // TODO: verify errors propagate correctly across module boundaries
    let result = {{FUNCTION_NAME}}({{INVALID_INPUT}});
    assert_eq!(result, {{ZERO_EXPECTED}});
}
