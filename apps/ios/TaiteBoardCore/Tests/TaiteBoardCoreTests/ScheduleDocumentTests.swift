import XCTest
@testable import TaiteBoardCore

final class ScheduleDocumentTests: XCTestCase {
    func testDecodesPromotionMetadataAndSnakeCaseFields() throws {
        let document = try TestSupport.document()

        XCTAssertEqual(document.schemaVersion, 1)
        XCTAssertEqual(document.currentNextId, "future")
        XCTAssertEqual(document.promotions?.first?.ctaLabel, "詳細を見る")
        XCTAssertEqual(document.promotions?.first?.priority, 10)
        XCTAssertEqual(document.promotions?.first?.active, true)
        XCTAssertNil(document.promotions?[1].imageUrl)
        XCTAssertEqual(document.sessions.first?.live.startTime, "18:00")
    }

    func testRejectsUnsupportedSchemaVersion() {
        let data = TestSupport.validJSONString.replacingOccurrences(
            of: "\"schema_version\": 1",
            with: "\"schema_version\": 2"
        ).data(using: .utf8)!

        XCTAssertThrowsError(try TestSupport.decoder.decode(data)) { error in
            XCTAssertEqual(error as? ScheduleValidationError, .unsupportedSchemaVersion(2))
        }
    }

    func testRejectsDuplicateSessionIds() {
        let data = TestSupport.validJSONString.replacingOccurrences(
            of: "\"id\": \"future\"",
            with: "\"id\": \"past\""
        ).data(using: .utf8)!

        XCTAssertThrowsError(try TestSupport.decoder.decode(data)) { error in
            XCTAssertEqual(error as? ScheduleValidationError, .duplicateSessionId("past"))
        }
    }

    func testRejectsDuplicatePromotionIds() {
        let data = TestSupport.validJSONString.replacingOccurrences(
            of: "\"id\": \"promotion-2\"",
            with: "\"id\": \"promotion-1\""
        ).data(using: .utf8)!

        XCTAssertThrowsError(try TestSupport.decoder.decode(data)) { error in
            XCTAssertEqual(error as? ScheduleValidationError, .duplicatePromotionId("promotion-1"))
        }
    }

    func testRejectsInvalidDate() {
        let data = TestSupport.validJSONString.replacingOccurrences(
            of: "\"date\": \"2026-10-31\"",
            with: "\"date\": \"2026-02-30\""
        ).data(using: .utf8)!

        XCTAssertThrowsError(try TestSupport.decoder.decode(data)) { error in
            XCTAssertEqual(error as? ScheduleValidationError, .invalidSessionDate("2026-02-30"))
        }
    }
}
