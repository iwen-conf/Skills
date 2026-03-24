import XCTest
@testable import {{MODULE_NAME}}

final class {{CLASS_NAME}}IntegrationTests: XCTestCase {

    // Shared dependency state
    // private var connection: Connection?

    override func setUp() {
        super.setUp()
        // TODO: initialize cross-module dependencies
    }

    override func tearDown() {
        // TODO: release shared resources
        super.tearDown()
    }

    func testIntegration{{FUNCTION_NAME_PASCAL}}Nominal() {
        let result = {{FUNCTION_NAME}}({{SAMPLE_INPUT}})
        XCTAssertEqual(result, {{EXPECTED_OUTPUT}})
    }

    func testIntegration{{FUNCTION_NAME_PASCAL}}ErrorPropagation() {
        // TODO: verify errors propagate correctly across modules
        XCTAssertThrowsError(try {{FUNCTION_NAME}}({{INVALID_INPUT}}))
    }

    func testIntegration{{FUNCTION_NAME_PASCAL}}DataFlow() {
        let result = {{FUNCTION_NAME}}({{NOMINAL_VALUE}})
        XCTAssertEqual(result, {{NOMINAL_EXPECTED}})
    }
}
