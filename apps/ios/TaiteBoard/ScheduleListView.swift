import SwiftUI
import TaiteBoardCore

struct ScheduleListView: View {
    let snapshot: RepositorySnapshot
    let onSelectSession: (String) -> Void

    private var upcomingSessions: [Session] {
        snapshot.document.sessions
            .filter { $0.date >= todayString }
            .sorted { "\($0.date)-\($0.live.startTime)-\($0.id)" < "\($1.date)-\($1.live.startTime)-\($1.id)" }
    }

    private var todayString: String {
        let components = Calendar.current.dateComponents([.year, .month, .day], from: Date())
        return String(
            format: "%04d-%02d-%02d",
            components.year ?? 0,
            components.month ?? 0,
            components.day ?? 0
        )
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Text("開催予定の公演")
                    Spacer()
                    Text("\(upcomingSessions.count)公演")
                        .fontWeight(.bold)
                        .foregroundStyle(RibbonTheme.rose)
                        .padding(.horizontal, 9)
                        .padding(.vertical, 4)
                        .background(RibbonTheme.rose.opacity(0.10), in: Capsule())
                        .overlay(Capsule().stroke(RibbonTheme.roseStrong.opacity(0.22), lineWidth: 1))
                }
                .font(.caption)
                .foregroundStyle(RibbonTheme.textSecondary)
                .padding(.horizontal, 2)

                LazyVStack(spacing: 10) {
                    ForEach(upcomingSessions, id: \.id) { session in
                        Button {
                            onSelectSession(session.id)
                        } label: {
                            sessionRow(session)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
            .padding(.top, 10)
            .padding(.bottom, 12)
        }
        .scrollIndicators(.hidden)
    }

    private func sessionRow(_ session: Session) -> some View {
        let parts = session.date.split(separator: "-")
        let month = parts.count > 1 ? Int(parts[1]) ?? 0 : 0
        let day = parts.count > 2 ? Int(parts[2]) ?? 0 : 0
        let sameDayCount = snapshot.document.sessions.filter { $0.date == session.date }.count
        let sessionTag: String? = if sameDayCount > 1 {
            session.sessionName?.contains("昼") == true ? "昼" : "夜"
        } else {
            nil
        }

        return HStack(spacing: 14) {
            VStack(spacing: 2) {
                Text("\(month)月")
                    .font(.caption2.weight(.bold))
                    .foregroundStyle(RibbonTheme.textSecondary)
                Text("\(day)")
                    .font(.title3.monospacedDigit().weight(.black))
                    .foregroundStyle(RibbonTheme.textPrimary)
                Text(session.dayOfWeek ?? "")
                    .font(.caption2.weight(.bold))
                    .foregroundStyle(dayColor(for: session))
            }
            .frame(width: 56, height: 60)
            .background(RibbonTheme.background, in: RoundedRectangle(cornerRadius: 12))
            .overlay(RoundedRectangle(cornerRadius: 12).stroke(RibbonTheme.border, lineWidth: 1))

            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .top, spacing: 6) {
                    Label(session.venue.name, systemImage: "mappin.and.ellipse")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(RibbonTheme.textSecondary)
                        .lineLimit(1)
                    Spacer(minLength: 4)
                    if let sessionTag {
                        Text("[\(sessionTag)]")
                            .font(.caption2.weight(.bold))
                            .foregroundStyle(RibbonTheme.rose)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 3)
                            .background(RibbonTheme.rose.opacity(0.18), in: RoundedRectangle(cornerRadius: 5))
                    }
                }

                Text(session.title)
                    .font(.subheadline.weight(.heavy))
                    .foregroundStyle(RibbonTheme.textPrimary)
                    .multilineTextAlignment(.leading)
                    .lineLimit(2)

                Divider()
                    .overlay(RibbonTheme.border)

                HStack {
                    Label("出番 \(session.live.startTime)〜", systemImage: "mic.fill")
                        .foregroundStyle(RibbonTheme.textPrimary)
                    Spacer(minLength: 8)
                    Text("特典会 \(session.benefit.startTime)〜")
                        .foregroundStyle(RibbonTheme.textSecondary)
                }
                .font(.caption2.monospacedDigit())
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .ribbonCard(cornerRadius: 16)
    }

    private func dayColor(for session: Session) -> Color {
        let day = session.dayOfWeek ?? ""
        if day == "土" { return .blue }
        if day == "日" || day.contains("祝") { return RibbonTheme.rose }
        return RibbonTheme.textSecondary
    }
}
