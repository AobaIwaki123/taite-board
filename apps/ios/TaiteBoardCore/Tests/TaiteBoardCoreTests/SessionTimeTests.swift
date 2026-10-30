import XCTest
@testable import TaiteBoardCore

final class SessionTimeTests: XCTestCase {
    func testUnknownTimeDoesNotBecomeMidnight() {
        let time = SessionTime("--:--")

        XCTAssertTrue(time.isUnknown)
        XCTAssertNil(time.dateComponents)
    }

    func testParsesKnownTime() {
        let time = SessionTime("12:45")

        XCTAssertFalse(time.isUnknown)
        XCTAssertEqual(time.dateComponents?.hour, 12)
        XCTAssertEqual(time.dateComponents?.minute, 45)
    }

    func testResolverFallsBackFromPastCurrentNextId() throws {
        let data = TestSupport.validJSONString.replacingOccurrences(
            of: "\"current_next_id\": \"future\"",
            with: "\"current_next_id\": \"past\""
        ).data(using: .utf8)!
        let document = try TestSupport.document(from: data)
        let resolved = NextSessionResolver().resolve(
            in: document,
            now: TestSupport.fixedDate()
        )

        XCTAssertEqual(resolved?.id, "future")
    }

    func testResolverUsesCurrentNextIdWhenItIsUpcoming() throws {
        let document = try TestSupport.document()
        let resolved = NextSessionResolver().resolve(
            in: document,
            now: TestSupport.fixedDate()
        )

        XCTAssertEqual(resolved?.id, "future")
    }
}
