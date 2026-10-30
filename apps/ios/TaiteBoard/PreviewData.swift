import Foundation
import SwiftUI
import TaiteBoardCore

enum PreviewData {
    static let rawData = """
    {
      "schema_version": 1,
      "generated_at": "2026-10-30T00:00:00+09:00",
      "timezone": "Asia/Tokyo",
      "current_next_id": "preview-next",
      "target_group": { "name": "リボンクロック" },
      "sessions": [
        {
          "id": "preview-next",
          "title": "Preview Live 2026",
          "date": "2026-10-31",
          "venue": { "name": "Preview Hall" },
          "live": { "start_time": "12:00", "end_time": "13:00" },
          "benefit": { "start_time": "13:10", "end_time": "14:00" },
          "ticket": { "platform": "Preview Ticket", "url": "https://example.com/ticket" }
        },
        {
          "id": "preview-next-2",
          "title": "Evening Preview",
          "date": "2026-11-01",
          "venue": { "name": "Second Hall" },
          "live": { "start_time": "18:00", "end_time": "19:00" },
          "benefit": { "start_time": "19:10", "end_time": "20:00" },
          "ticket": { "platform": "Door" }
        }
      ]
    }
    """.data(using: .utf8)!

    static let document = try! ScheduleDocumentDecoder().decode(rawData)
    static let snapshot = RepositorySnapshot(
        document: document,
        rawData: rawData,
        origin: .seed,
        cachedAt: nil,
        diagnostics: nil
    )

    @MainActor
    static func model() -> AppModel {
        AppModel(
            repository: ScheduleRepository(seed: SeedScheduleSource(data: rawData)),
            initialSnapshot: snapshot
        )
    }
}

#Preview("Next") {
    RootView(model: PreviewData.model())
}

#Preview("Data Status") {
    DataStatusView(model: PreviewData.model())
}
