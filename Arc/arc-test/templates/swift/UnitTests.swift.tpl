import XCTest
@testable import {{MODULE_NAME}}

final class {{CLASS_NAME}}Tests: XCTestCase {

    func test{{FUNCTION_NAME_PASCAL}}Nominal() {
        let result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
        XCTAssertEqual(result, {{EXPECTED_OUTPUT}})
    }

    func test{{FUNCTION_NAME_PASCAL}}BoundaryMin() {
        let result = {{FUNCTION_NAME}}({{MIN_VALUE}})
        XCTAssertEqual(result, {{MIN_EXPECTED}})
    }

    func test{{FUNCTION_NAME_PASCAL}}BoundaryMax() {
        let result = {{FUNCTION_NAME}}({{MAX_VALUE}})
        XCTAssertEqual(result, {{MAX_EXPECTED}})
    }

    func test{{FUNCTION_NAME_PASCAL}}Performance() {
        measure {
            _ = {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
        }
    }
}
