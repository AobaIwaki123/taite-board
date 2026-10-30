import SwiftUI
import TaiteBoardCore

struct NextView: View {
    let snapshot: RepositorySnapshot

    private var nextSession: Session? {
        NextSessionResolver().resolve(in: snapshot.document)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                if let nextSession {
                    VStack(alignment: .leading, spacing: 14) {
                        Text("NEXT LIVE")
                            .font(.caption.weight(.bold))
                            .foregroundStyle(.secondary)

                        Text(nextSession.title)
                            .font(.title2.weight(.bold))
                            .fixedSize(horizontal: false, vertical: true)

                        Label(nextSession.date, systemImage: "calendar")
                        Label(nextSession.venue.name, systemImage: "mappin.and.ellipse")
                        Label(nextSession.live.startTime, systemImage: "clock")

                        NavigationLink {
                            SessionDetailView(session: nextSession)
                        } label: {
                            Label("公演詳細を見る", systemImage: "arrow.forward")
                                .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(.borderedProminent)
                    }
                    .padding(20)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(.tint.opacity(0.12), in: RoundedRectangle(cornerRadius: 24))
                } else {
                    ContentUnavailableView("次回公演はありません", systemImage: "calendar.badge.checkmark")
                }

                DataSummary(snapshot: snapshot)
            }
            .padding()
        }
        .navigationTitle(snapshot.document.targetGroup.name)
    }
}

private struct DataSummary: View {
    let snapshot: RepositorySnapshot

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("データ状態")
                .font(.headline)
            LabeledContent("取得元", value: snapshot.origin.displayName)
            LabeledContent("生成時刻", value: snapshot.document.generatedAt)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.quaternary.opacity(0.5), in: RoundedRectangle(cornerRadius: 18))
    }
}

extension DataOrigin {
    var displayName: String {
        switch self {
        case .remote: "リモート"
        case .diskCache: "ディスクキャッシュ"
        case .seed: "アプリ内 seed"
        }
    }
}
