import Foundation

public struct CacheMetadata: Codable, Equatable, Sendable {
    public let cachedAt: Date
    public let schemaVersion: Int
    public let diagnostics: RequestDiagnostics?

    public init(
        cachedAt: Date,
        schemaVersion: Int,
        diagnostics: RequestDiagnostics?
    ) {
        self.cachedAt = cachedAt
        self.schemaVersion = schemaVersion
        self.diagnostics = diagnostics
    }
}

public struct CachedSchedule: Sendable {
    public let rawData: Data
    public let document: ScheduleDocument
    public let metadata: CacheMetadata

    public init(rawData: Data, document: ScheduleDocument, metadata: CacheMetadata) {
        self.rawData = rawData
        self.document = document
        self.metadata = metadata
    }
}

public actor DiskScheduleCache {
    private let directory: URL
    private let scheduleURL: URL
    private let metadataURL: URL
    private let decoder: ScheduleDocumentDecoder

    public init(directory: URL, decoder: ScheduleDocumentDecoder = .init()) {
        self.directory = directory
        self.scheduleURL = directory.appendingPathComponent("schedule.json")
        self.metadataURL = directory.appendingPathComponent("cache-metadata.json")
        self.decoder = decoder
    }

    public func load() throws -> CachedSchedule? {
        guard FileManager.default.fileExists(atPath: scheduleURL.path),
              FileManager.default.fileExists(atPath: metadataURL.path) else {
            return nil
        }

        let rawData = try Data(contentsOf: scheduleURL)
        let document = try decoder.decode(rawData)
        let metadata = try decodeMetadata(Data(contentsOf: metadataURL))
        guard metadata.schemaVersion == document.schemaVersion else {
            return nil
        }

        return CachedSchedule(rawData: rawData, document: document, metadata: metadata)
    }

    public func save(_ payload: SchedulePayload, cachedAt: Date = Date()) throws {
        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true
        )

        let metadata = CacheMetadata(
            cachedAt: cachedAt,
            schemaVersion: payload.document.schemaVersion,
            diagnostics: payload.diagnostics
        )

        try payload.rawData.write(to: scheduleURL, options: .atomic)
        try encodeMetadata(metadata).write(to: metadataURL, options: .atomic)
    }

    private func encodeMetadata(_ metadata: CacheMetadata) throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        encoder.dateEncodingStrategy = .iso8601
        return try encoder.encode(metadata)
    }

    private func decodeMetadata(_ data: Data) throws -> CacheMetadata {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(CacheMetadata.self, from: data)
    }
}
