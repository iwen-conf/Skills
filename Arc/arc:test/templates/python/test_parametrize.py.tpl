"""Parameterized tests for {{MODULE_NAME}}."""

import pytest

from {{MODULE_PATH}} import {{FUNCTION_NAME}}


@pytest.mark.parametrize(
    ("input_val", "expected"),
    [
        ({{NOMINAL_VALUE}}, {{NOMINAL_EXPECTED}}),
        ({{MIN_VALUE}}, {{MIN_EXPECTED}}),
        ({{MAX_VALUE}}, {{MAX_EXPECTED}}),
        ({{ZERO_VALUE}}, {{ZERO_EXPECTED}}),
        # FILL_IN: add equivalence partitioning cases
    ],
)
def test_{{FUNCTION_NAME}}_parametrize(input_val: {{INPUT_TYPE}}, expected: {{OUTPUT_TYPE}}) -> None:
    assert {{FUNCTION_NAME}}(input_val) == expected
