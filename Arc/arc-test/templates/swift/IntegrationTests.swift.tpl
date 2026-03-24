import XCTest
@testable import {{MODULE_NAME}}

final class {{CLASS_NAME}}IntegrationTests: XCTestCase {

    // Shared dependency state
    // private var connection: Connection?

    override func setUp() {
        super.setUp()
        // FILL_IN: initialize cross-module dependencies
    }

    override func tearDown() {
        // FILL_IN: release shared resources
        super.tearDown()
    }

    func testIntegration{{FUNCTION_NAME_PASCAL}}Nominal() {
        let result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
        XCTAssertEqual(result, {{EXPECTED_OUTPUT}})
    }

    func testIntegration{{FUNCTION_NAME_PASCAL}}ErrorPropagation() {
        // FILL_IN: verify errors propagate correctly across modules
        XCTAssertThrowsError(try {{FUNCTION_NAME}}({{INVALID_INPUT}}))
    }

    func testIntegration{{FUNCTION_NAME_PASCAL}}DataFlow() {
        let result = {{FUNCTION_NAME}}({{NOMINAL_VALUE}})
        XCTAssertEqual(result, {{NOMINAL_EXPECTED}})
    }
}
