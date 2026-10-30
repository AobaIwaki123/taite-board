import SwiftUI
import TaiteBoardCore

struct ScheduleListView: View {
    let snapshot: RepositorySnapshot

    var body: some View {
        List {
            ForEach(snapshot.document.sessions, id: \.id) { session in
                NavigationLink {
                    SessionDetailView(session: session)
                } label: {
                    VStack(alignment: .leading, spacing: 5) {
                        Text(session.date)
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(.secondary)
                        Text(session.title)
                            .font(.headline)
                            .lineLimit(2)
                        Text("\(session.live.startTime)  ·  \(session.venue.name)")
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                    }
                    .padding(.vertical, 5)
                }
            }
        }
        .navigationTitle("全日程")
    }
}
