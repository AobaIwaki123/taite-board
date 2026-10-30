import Foundation
import XCTest
@testable import TaiteBoardCore

enum TestSupport {
    static let validJSONString = """
    {
      "schema_version": 1,
      "generated_at": "2026-10-30T00:00:00+09:00",
      "timezone": "Asia/Tokyo",
      "current_next_id": "future",
      "target_group": {
        "canonical_id": "group:test",
        "name": "テストグループ"
      },
      "promotions": [
        {
          "id": "promotion-1",
          "type": "hero",
          "badge": "TEST",
          "title": "テスト告知",
          "subtitle": "サブタイトル",
          "cta_label": "詳細を見る",
          "image_url": "https://example.com/image.jpg",
          "cta_url": "https://example.com/promotion",
          "priority": 10,
          "active": true
        },
        {
          "id": "promotion-2",
          "title": "画像なしのお知らせ",
          "cta_url": "https://example.com/announcement"
        }
      ],
      "sessions": [
        {
          "id": "past",
          "title": "過去公演",
          "date": "2026-10-29",
          "venue": { "name": "過去会場" },
          "live": { "start_time": "18:00", "end_time": "19:00" },
          "benefit": { "start_time": "19:10", "end_time": "20:00" }
        },
        {
          "id": "future",
          "title": "次回公演",
          "date": "2026-10-31",
          "venue": { "name": "未来会場" },
          "live": { "start_time": "12:00", "end_time": "13:00" },
          "benefit": { "start_time": "13:10", "end_time": "14:00" }
        }
      ]
    }
    """

    static let validJSON = validJSONString.data(using: .utf8)!

    static let decoder = ScheduleDocumentDecoder()

    static func document(
        from data: Data = validJSON,
        file: StaticString = #filePath,
        line: UInt = #line
    ) throws -> ScheduleDocument {
        do {
            return try decoder.decode(data)
        } catch {
            XCTFail("Fixture should decode: \(error)", file: file, line: line)
            throw error
        }
    }

    static func fixedDate() -> Date {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.date(from: "2026-10-30T10:00:00+09:00")!
    }

    static func temporaryDirectory() -> URL {
        FileManager.default.temporaryDirectory
            .appendingPathComponent("taite-board-test-\(UUID().uuidString)")
    }
}
