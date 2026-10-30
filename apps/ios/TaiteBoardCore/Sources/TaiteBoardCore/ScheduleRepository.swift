import Foundation

public enum RefreshReason: String, Sendable {
    case launch
    case foreground
    case manual
}

public enum ScheduleRepositoryError: Error, Equatable, LocalizedError, Sendable {
    case noDataAvailable
    case remoteSourceUnavailable

    public var errorDescription: String? {
        switch self {
        case .noDataAvailable:
            return "No valid schedule data is available from cache or seed"
        case .remoteSourceUnavailable:
            return "A remote schedule source is not configured"
        }
    }
}

public struct RepositorySnapshot: Sendable {
    public let document: ScheduleDocument
    public let rawData: Data
    public let origin: DataOrigin
    public let cachedAt: Date?
    public let diagnostics: RequestDiagnostics?

    public init(
        document: ScheduleDocument,
        rawData: Data,
        origin: DataOrigin,
        cachedAt: Date?,
        diagnostics: RequestDiagnostics?
    ) {
        self.document = document
        self.rawData = rawData
        self.origin = origin
        self.cachedAt = cachedAt
        self.diagnostics = diagnostics
    }
}

public actor ScheduleRepository {
    private let seed: any ScheduleSource
    private let remote: (any ScheduleSource)?
    private let cache: DiskScheduleCache?
    private let now: @Sendable () -> Date
    private var currentSnapshot: RepositorySnapshot?

    public init(
        seed: any ScheduleSource,
        remote: (any ScheduleSource)? = nil,
        cache: DiskScheduleCache? = nil,
        now: @escaping @Sendable () -> Date = Date.init
    ) {
        self.seed = seed
        self.remote = remote
        self.cache = cache
        self.now = now
    }

    public func bootstrap() async throws -> RepositorySnapshot {
        if let cache,
           let cached = try? await cache.load() {
            let snapshot = RepositorySnapshot(
                document: cached.document,
                rawData: cached.rawData,
                origin: .diskCache,
                cachedAt: cached.metadata.cachedAt,
                diagnostics: cached.metadata.diagnostics
            )
            currentSnapshot = snapshot
            return snapshot
        }

        if let seeded = try? await seed.load() {
            let snapshot = RepositorySnapshot(
                document: seeded.document,
                rawData: seeded.rawData,
                origin: .seed,
                cachedAt: nil,
                diagnostics: seeded.diagnostics
            )
            currentSnapshot = snapshot
            return snapshot
        }

        throw ScheduleRepositoryError.noDataAvailable
    }

    public func refresh(reason _: RefreshReason) async throws -> RepositorySnapshot {
        guard let remote else {
            throw ScheduleRepositoryError.remoteSourceUnavailable
        }

        let payload = try await remote.load()
        let fetchedAt = payload.diagnostics?.fetchedAt ?? now()
        if let cache {
            try? await cache.save(payload, cachedAt: fetchedAt)
        }

        let snapshot = RepositorySnapshot(
            document: payload.document,
            rawData: payload.rawData,
            origin: .remote,
            cachedAt: fetchedAt,
            diagnostics: payload.diagnostics
        )
        currentSnapshot = snapshot
        return snapshot
    }

    public func snapshot() -> RepositorySnapshot? {
        currentSnapshot
    }
}
