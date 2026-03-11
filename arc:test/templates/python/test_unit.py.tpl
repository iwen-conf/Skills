"""Tests for {{MODULE_NAME}}."""

from {{MODULE_PATH}} import {{FUNCTION_NAME}}


class TestCase{{FUNCTION_NAME_PASCAL}}:
    """Unit tests for {{FUNCTION_NAME}}."""

    def test_nominal(self) -> None:
        result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
        assert result == {{EXPECTED_OUTPUT}}

    def test_boundary_min(self) -> None:
        result = {{FUNCTION_NAME}}({{MIN_VALUE}})
        assert result == {{MIN_EXPECTED}}

    def test_boundary_max(self) -> None:
        result = {{FUNCTION_NAME}}({{MAX_VALUE}})
        assert result == {{MAX_EXPECTED}}

    def test_error_path(self) -> None:
        # TODO: test invalid input raises expected exception
        pass
