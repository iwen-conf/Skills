"""Integration tests for {{MODULE_NAME}}.

Run with: pytest -m integration
"""

import pytest

from {{MODULE_PATH}} import {{FUNCTION_NAME}}


pytestmark = pytest.mark.integration


@pytest.fixture(scope="module")
def setup_dependencies():
    """Module-level setup for shared dependencies."""
    # TODO: initialize DB connections, HTTP clients, external services
    resource = None
    yield resource
    # TODO: teardown — close connections, clean state


class TestIntegration{{FUNCTION_NAME_PASCAL}}:
    """Integration tests for {{FUNCTION_NAME}} with collaborating modules."""

    def test_cross_module_nominal(self, setup_dependencies) -> None:
        result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
        assert result == {{EXPECTED_OUTPUT}}

    def test_dependency_error_propagation(self, setup_dependencies) -> None:
        # TODO: verify error handling across module boundaries
        with pytest.raises(Exception):
            {{FUNCTION_NAME}}({{INVALID_INPUT}})

    def test_data_flow_through_layers(self, setup_dependencies) -> None:
        # TODO: trace data from input through processing to output
        result = {{FUNCTION_NAME}}({{NOMINAL_VALUE}})
        assert result == {{NOMINAL_EXPECTED}}
