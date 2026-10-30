import Foundation

public enum DataOrigin: String, Codable, Equatable, Sendable {
    case remote
    case diskCache
    case seed
}

public enum FreshnessLevel: String, Codable, Equatable, Sendable {
    case fresh
    case stale
    case unknown
}

public struct FreshnessState: Equatable, Sendable {
    public let level: FreshnessLevel
    public let origin: DataOrigin
    public let generatedAt: Date?
    public let cachedAt: Date?

    public init(
        level: FreshnessLevel,
        origin: DataOrigin,
        generatedAt: Date?,
        cachedAt: Date?
    ) {
        self.level = level
        self.origin = origin
        self.generatedAt = generatedAt
        self.cachedAt = cachedAt
    }
}

public struct FreshnessPolicy: Sendable {
    public let staleAfter: TimeInterval

    public init(staleAfter: TimeInterval = 24 * 60 * 60) {
        self.staleAfter = staleAfter
    }

    public func evaluate(
        generatedAt: Date?,
        origin: DataOrigin,
        cachedAt: Date?,
        now: Date = Date()
    ) -> FreshnessState {
        let level: FreshnessLevel
        if let generatedAt {
            level = now.timeIntervalSince(generatedAt) >= staleAfter ? .stale : .fresh
        } else {
            level = .unknown
        }

        return FreshnessState(
            level: level,
            origin: origin,
            generatedAt: generatedAt,
            cachedAt: cachedAt
        )
    }
}
