import XCTest
@testable import TaiteBoardCore

final class DiskScheduleCacheTests: XCTestCase {
    func testRoundTripKeepsRawDataAndDiagnostics() async throws {
        let directory = TestSupport.temporaryDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }

        let document = try TestSupport.document()
        let fetchedAt = TestSupport.fixedDate()
        let diagnostics = RequestDiagnostics(
            url: URL(string: "https://example.com/schedule.json")!,
            statusCode: 200,
            contentType: "application/json",
            contentLength: TestSupport.validJSON.count,
            etag: "\"test-etag\"",
            lastModified: nil,
            duration: 0.12,
            payloadBytes: TestSupport.validJSON.count,
            fetchedAt: fetchedAt
        )
        let payload = SchedulePayload(
            rawData: TestSupport.validJSON,
            document: document,
            diagnostics: diagnostics
        )
        let cache = DiskScheduleCache(directory: directory)

        try await cache.save(payload, cachedAt: fetchedAt)
        let loadedOptional = try await cache.load()
        let loaded = try XCTUnwrap(loadedOptional)

        XCTAssertEqual(loaded.rawData, TestSupport.validJSON)
        XCTAssertEqual(loaded.document, document)
        XCTAssertEqual(loaded.metadata.cachedAt, fetchedAt)
        XCTAssertEqual(loaded.metadata.diagnostics, diagnostics)
    }
}
