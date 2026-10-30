import Foundation

public struct ScheduleDocument: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let generatedAt: String
    public let timezone: String
    public let currentNextId: String?
    public let targetGroup: TargetGroup
    public let featuredMember: FeaturedMember?
    public let promotions: [Promotion]?
    public let sessions: [Session]
}

public struct TargetGroup: Codable, Equatable, Sendable {
    public let canonicalId: String?
    public let name: String
    public let shortName: String?
    public let officialX: String?
    public let officialTiktok: String?
}

public struct FeaturedMember: Codable, Equatable, Sendable {
    public let canonicalId: String?
    public let name: String?
    public let memberColor: String?
    public let colorCode: String?
    public let birthdayTitle: String?
    public let crowdfundingUrl: String?
    public let bannerImage: String?
}

public struct Promotion: Codable, Equatable, Sendable {
    public let id: String
    public let type: String?
    public let badge: String?
    public let title: String
    public let subtitle: String?
    public let ctaLabel: String?
    public let ctaUrl: String
    public let imageUrl: String?
    public let deadline: String?
    public let priority: Int?
    public let active: Bool?
}

public struct Venue: Codable, Equatable, Sendable {
    public let canonicalId: String?
    public let name: String
    public let mapUrl: String?
    public let mapQuery: String?
    public let officialUrl: String?
}

public struct LivePerformance: Codable, Equatable, Sendable {
    public let startTime: String
    public let endTime: String?
    public let stage: String?
}

public struct BenefitEvent: Codable, Equatable, Sendable {
    public let type: String?
    public let startTime: String
    public let endTime: String?
    public let booth: String?
    public let note: String?
}

public struct PhotoRule: Codable, Equatable, Sendable {
    public let available: Bool?
    public let rule: String?
    public let rawText: String?
}

public struct TicketPrice: Codable, Equatable, Sendable {
    public let tier: String?
    public let price: String?
}

public struct TicketInfo: Codable, Equatable, Sendable {
    public let platform: String?
    public let url: String?
    public let prices: [TicketPrice]?
}

public struct SessionImage: Codable, Equatable, Sendable {
    public let type: String
    public let label: String?
    public let url: String
}

public struct SourceReference: Codable, Equatable, Sendable {
    public let type: String
    public let sourceId: String?
    public let url: String
}

public struct Session: Codable, Equatable, Sendable {
    public let id: String
    public let title: String
    public let date: String
    public let dayOfWeek: String?
    public let sessionName: String?
    public let venue: Venue
    public let live: LivePerformance
    public let benefit: BenefitEvent
    public let photo: PhotoRule?
    public let specialNotes: String?
    public let admissionBonus: String?
    public let ticket: TicketInfo?
    public let imageUrl: String?
    public let sourceTweetUrl: String?
    public let images: [SessionImage]?
    public let sourceRefs: [SourceReference]?
}

public struct ScheduleDocumentDecoder: Sendable {
    public let supportedSchemaVersions: Set<Int>

    public init(supportedSchemaVersions: Set<Int> = [1]) {
        self.supportedSchemaVersions = supportedSchemaVersions
    }

    public func decode(_ data: Data) throws -> ScheduleDocument {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        let document = try decoder.decode(ScheduleDocument.self, from: data)
        try document.validate(supportedSchemaVersions: supportedSchemaVersions)
        return document
    }
}

public enum ScheduleValidationError: Error, Equatable, LocalizedError, Sendable {
    case unsupportedSchemaVersion(Int)
    case invalidGeneratedAt(String)
    case invalidTimezone(String)
    case emptyTargetGroupName
    case invalidSessionId
    case duplicateSessionId(String)
    case invalidSessionDate(String)

    public var errorDescription: String? {
        switch self {
        case let .unsupportedSchemaVersion(version):
            return "Unsupported schedule schema version: \(version)"
        case let .invalidGeneratedAt(value):
            return "Invalid generated_at: \(value)"
        case let .invalidTimezone(value):
            return "Invalid timezone: \(value)"
        case .emptyTargetGroupName:
            return "target_group.name must not be empty"
        case .invalidSessionId:
            return "Session id must not be empty"
        case let .duplicateSessionId(id):
            return "Duplicate session id: \(id)"
        case let .invalidSessionDate(value):
            return "Invalid session date: \(value)"
        }
    }
}

extension ScheduleDocument {
    public func validate(supportedSchemaVersions: Set<Int> = [1]) throws {
        guard supportedSchemaVersions.contains(schemaVersion) else {
            throw ScheduleValidationError.unsupportedSchemaVersion(schemaVersion)
        }

        guard parseISO8601(generatedAt) != nil else {
            throw ScheduleValidationError.invalidGeneratedAt(generatedAt)
        }

        guard TimeZone(identifier: timezone) != nil else {
            throw ScheduleValidationError.invalidTimezone(timezone)
        }

        guard !targetGroup.name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw ScheduleValidationError.emptyTargetGroupName
        }

        var ids = Set<String>()
        for session in sessions {
            guard !session.id.isEmpty else {
                throw ScheduleValidationError.invalidSessionId
            }
            guard ids.insert(session.id).inserted else {
                throw ScheduleValidationError.duplicateSessionId(session.id)
            }
            guard isValidDateString(session.date) else {
                throw ScheduleValidationError.invalidSessionDate(session.date)
            }
        }
    }
}

private func parseISO8601(_ value: String) -> Date? {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let date = formatter.date(from: value) {
        return date
    }

    formatter.formatOptions = [.withInternetDateTime]
    return formatter.date(from: value)
}

private func isValidDateString(_ value: String) -> Bool {
    let parts = value.split(separator: "-")
    guard parts.count == 3,
          parts[0].count == 4,
          parts[1].count == 2,
          parts[2].count == 2,
          let year = Int(parts[0]),
          let month = Int(parts[1]),
          let day = Int(parts[2]) else {
        return false
    }

    var calendar = Calendar(identifier: .gregorian)
    calendar.timeZone = TimeZone(secondsFromGMT: 0)!
    let components = DateComponents(year: year, month: month, day: day)
    guard let date = calendar.date(from: components) else {
        return false
    }

    let normalized = calendar.dateComponents([.year, .month, .day], from: date)
    return normalized.year == year && normalized.month == month && normalized.day == day
}
