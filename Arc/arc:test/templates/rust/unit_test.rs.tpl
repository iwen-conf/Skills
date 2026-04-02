#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_{{FUNCTION_NAME}}() {
        // FILL_IN: implement test for {{FUNCTION_NAME}}
        let result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}});
        assert_eq!(result, {{EXPECTED_OUTPUT}});
    }

    #[test]
    fn test_{{FUNCTION_NAME}}_boundary() {
        // boundary value analysis: min, max, off-boundary
        let min_result = {{FUNCTION_NAME}}({{MIN_VALUE}});
        assert_eq!(min_result, {{MIN_EXPECTED}});

        let max_result = {{FUNCTION_NAME}}({{MAX_VALUE}});
        assert_eq!(max_result, {{MAX_EXPECTED}});
    }

    #[test]
    #[should_panic]
    fn test_{{FUNCTION_NAME}}_invalid_input() {
        // error path: invalid input
        {{FUNCTION_NAME}}({{INVALID_INPUT}});
    }
}
