import Foundation

public struct RequestDiagnostics: Codable, Equatable, Sendable {
    public let url: URL
    public let statusCode: Int?
    public let contentType: String?
    public let contentLength: Int?
    public let etag: String?
    public let lastModified: String?
    public let duration: TimeInterval
    public let payloadBytes: Int
    public let fetchedAt: Date

    public init(
        url: URL,
        statusCode: Int?,
        contentType: String?,
        contentLength: Int?,
        etag: String?,
        lastModified: String?,
        duration: TimeInterval,
        payloadBytes: Int,
        fetchedAt: Date
    ) {
        self.url = url
        self.statusCode = statusCode
        self.contentType = contentType
        self.contentLength = contentLength
        self.etag = etag
        self.lastModified = lastModified
        self.duration = duration
        self.payloadBytes = payloadBytes
        self.fetchedAt = fetchedAt
    }
}

public struct SchedulePayload: Sendable {
    public let rawData: Data
    public let document: ScheduleDocument
    public let diagnostics: RequestDiagnostics?

    public init(
        rawData: Data,
        document: ScheduleDocument,
        diagnostics: RequestDiagnostics? = nil
    ) {
        self.rawData = rawData
        self.document = document
        self.diagnostics = diagnostics
    }
}

public protocol ScheduleSource: Sendable {
    func load() async throws -> SchedulePayload
}

public enum ScheduleSourceError: Error, Equatable, LocalizedError, Sendable {
    case resourceNotFound(String)
    case invalidHTTPResponse
    case unexpectedHTTPStatus(Int)

    public var errorDescription: String? {
        switch self {
        case let .resourceNotFound(name):
            return "Seed resource not found: \(name)"
        case .invalidHTTPResponse:
            return "The schedule response was not an HTTP response"
        case let .unexpectedHTTPStatus(status):
            return "Unexpected schedule HTTP status: \(status)"
        }
    }
}

public struct SeedScheduleSource: ScheduleSource, Sendable {
    private let data: Data
    private let decoder: ScheduleDocumentDecoder

    public init(data: Data, decoder: ScheduleDocumentDecoder = .init()) {
        self.data = data
        self.decoder = decoder
    }

    public init(
        bundle: Bundle,
        resourceName: String = "schedule",
        decoder: ScheduleDocumentDecoder = .init()
    ) throws {
        guard let url = bundle.url(forResource: resourceName, withExtension: "json") else {
            throw ScheduleSourceError.resourceNotFound(resourceName)
        }
        self.init(data: try Data(contentsOf: url), decoder: decoder)
    }

    public func load() async throws -> SchedulePayload {
        SchedulePayload(rawData: data, document: try decoder.decode(data))
    }
}

public actor RemoteScheduleSource: ScheduleSource {
    private let url: URL
    private let session: URLSession
    private let decoder: ScheduleDocumentDecoder
    private let timeout: TimeInterval

    public init(
        url: URL,
        session: URLSession = .shared,
        decoder: ScheduleDocumentDecoder = .init(),
        timeout: TimeInterval = 10
    ) {
        self.url = url
        self.session = session
        self.decoder = decoder
        self.timeout = timeout
    }

    public func load() async throws -> SchedulePayload {
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.timeoutInterval = timeout
        request.cachePolicy = .reloadIgnoringLocalCacheData
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue("no-cache", forHTTPHeaderField: "Cache-Control")

        let startedAt = Date()
        let (data, response) = try await session.data(for: request)
        let fetchedAt = Date()

        guard let httpResponse = response as? HTTPURLResponse else {
            throw ScheduleSourceError.invalidHTTPResponse
        }
        guard (200..<300).contains(httpResponse.statusCode) else {
            throw ScheduleSourceError.unexpectedHTTPStatus(httpResponse.statusCode)
        }

        let diagnostics = RequestDiagnostics(
            url: url,
            statusCode: httpResponse.statusCode,
            contentType: httpResponse.value(forHTTPHeaderField: "Content-Type"),
            contentLength: httpResponse.value(forHTTPHeaderField: "Content-Length").flatMap(Int.init),
            etag: httpResponse.value(forHTTPHeaderField: "ETag"),
            lastModified: httpResponse.value(forHTTPHeaderField: "Last-Modified"),
            duration: fetchedAt.timeIntervalSince(startedAt),
            payloadBytes: data.count,
            fetchedAt: fetchedAt
        )

        return SchedulePayload(
            rawData: data,
            document: try decoder.decode(data),
            diagnostics: diagnostics
        )
    }
}
