import XCTest
@testable import TaiteBoardCore

private actor CountingScheduleSource: ScheduleSource {
    private let payload: SchedulePayload
    private var loads = 0

    init(payload: SchedulePayload) {
        self.payload = payload
    }

    func load() async throws -> SchedulePayload {
        loads += 1
        return payload
    }

    func loadCount() -> Int {
        loads
    }
}

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

    func testForegroundRefreshIsThrottledAfterRemoteSuccess() async throws {
        let document = try TestSupport.document()
        let payload = SchedulePayload(
            rawData: TestSupport.validJSON,
            document: document
        )
        let remote = CountingScheduleSource(payload: payload)
        let fixedDate = TestSupport.fixedDate()
        let repository = ScheduleRepository(
            seed: SeedScheduleSource(data: TestSupport.validJSON),
            remote: remote,
            now: { fixedDate }
        )

        _ = try await repository.bootstrap()
        _ = try await repository.refresh(reason: .launch)
        let snapshot = try await repository.refreshIfNeeded(
            reason: .foreground,
            minimumInterval: 15 * 60
        )
        let loadCount = await remote.loadCount()

        XCTAssertEqual(snapshot.origin, .remote)
        XCTAssertEqual(loadCount, 1)
    }
}
