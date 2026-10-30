import XCTest
@testable import TaiteBoardCore

final class RepositoryTests: XCTestCase {
    func testBootstrapUsesSeedThenRefreshPersistsRemotePayload() async throws {
        let directory = TestSupport.temporaryDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }

        let seed = SeedScheduleSource(data: TestSupport.validJSON)
        let cache = DiskScheduleCache(directory: directory)
        let fixedDate = TestSupport.fixedDate()
        let repository = ScheduleRepository(
            seed: seed,
            remote: seed,
            cache: cache,
            now: { fixedDate }
        )

        let initial = try await repository.bootstrap()
        XCTAssertEqual(initial.origin, .seed)

        let refreshed = try await repository.refresh(reason: .manual)
        XCTAssertEqual(refreshed.origin, .remote)
        XCTAssertEqual(refreshed.cachedAt, fixedDate)

        let restarted = ScheduleRepository(
            seed: seed,
            cache: cache,
            now: { fixedDate }
        )
        let cached = try await restarted.bootstrap()
        XCTAssertEqual(cached.origin, .diskCache)
        XCTAssertEqual(cached.document, initial.document)
        XCTAssertEqual(cached.cachedAt, fixedDate)
    }

    func testInvalidCacheFallsBackToSeed() async throws {
        let directory = TestSupport.temporaryDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }

        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true
        )
        try Data("not-json".utf8).write(
            to: directory.appendingPathComponent("schedule.json")
        )
        try Data("not-json".utf8).write(
            to: directory.appendingPathComponent("cache-metadata.json")
        )

        let repository = ScheduleRepository(
            seed: SeedScheduleSource(data: TestSupport.validJSON),
            cache: DiskScheduleCache(directory: directory)
        )

        let snapshot = try await repository.bootstrap()
        XCTAssertEqual(snapshot.origin, .seed)
    }
}
