import SwiftUI
import TaiteBoardCore

struct CalendarView: View {
    let snapshot: RepositorySnapshot

    private var sessionsByDate: [String: [Session]] {
        Dictionary(grouping: snapshot.document.sessions, by: \.date)
    }

    var body: some View {
        List {
            ForEach(sessionsByDate.keys.sorted(), id: \.self) { date in
                Section(date) {
                    ForEach(sessionsByDate[date] ?? [], id: \.id) { session in
                        NavigationLink {
                            SessionDetailView(session: session)
                        } label: {
                            VStack(alignment: .leading, spacing: 4) {
                                Text(session.title)
                                    .font(.headline)
                                Text("\(session.live.startTime)  ·  \(session.venue.name)")
                                    .font(.subheadline)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
        }
        .navigationTitle("カレンダー")
    }
}
