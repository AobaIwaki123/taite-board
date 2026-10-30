import XCTest
@testable import TaiteBoardCore

final class FreshnessTests: XCTestCase {
    func testDataWithinThresholdIsFresh() {
        let now = Date(timeIntervalSince1970: 100_000)
        let state = FreshnessPolicy(staleAfter: 86_400).evaluate(
            generatedAt: now.addingTimeInterval(-86_399),
            origin: .remote,
            cachedAt: now.addingTimeInterval(-10),
            now: now
        )

        XCTAssertEqual(state.level, .fresh)
        XCTAssertEqual(state.origin, .remote)
    }

    func testDataAtThresholdIsStale() {
        let now = Date(timeIntervalSince1970: 100_000)
        let state = FreshnessPolicy(staleAfter: 86_400).evaluate(
            generatedAt: now.addingTimeInterval(-86_400),
            origin: .diskCache,
            cachedAt: now.addingTimeInterval(-20),
            now: now
        )

        XCTAssertEqual(state.level, .stale)
        XCTAssertEqual(state.origin, .diskCache)
    }

    func testMissingGeneratedAtIsUnknown() {
        let state = FreshnessPolicy().evaluate(
            generatedAt: nil,
            origin: .seed,
            cachedAt: nil,
            now: Date()
        )

        XCTAssertEqual(state.level, .unknown)
        XCTAssertEqual(state.origin, .seed)
    }
}
