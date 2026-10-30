import SwiftUI
import TaiteBoardCore

struct SessionDetailView: View {
    let session: Session

    var body: some View {
        List {
            Section("公演") {
                LabeledContent("タイトル", value: session.title)
                LabeledContent("日付", value: session.date)
                LabeledContent("会場", value: session.venue.name)
            }

            Section("タイムテーブル") {
                LabeledContent("ライブ", value: timeRange(start: session.live.startTime, end: session.live.endTime))
                LabeledContent("特典会", value: timeRange(start: session.benefit.startTime, end: session.benefit.endTime))
            }

            if let ticket = session.ticket {
                Section("チケット") {
                    if let platform = ticket.platform {
                        LabeledContent("販売", value: platform)
                    }
                    if let urlString = ticket.url, let url = URL(string: urlString) {
                        Link("チケットページを開く", destination: url)
                    }
                }
            }

            if let notes = session.specialNotes, !notes.isEmpty {
                Section("メモ") {
                    Text(notes)
                }
            }
        }
        .navigationTitle("公演詳細")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func timeRange(start: String, end: String?) -> String {
        guard let end, !end.isEmpty else { return start }
        return "\(start)–\(end)"
    }
}
