import SwiftUI
import TaiteBoardCore

struct CalendarView: View {
    let snapshot: RepositorySnapshot
    let onSelectSession: (String) -> Void

    @State private var displayedMonth = Date()
    @State private var selectedDate: String?

    private var calendar: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.locale = Locale(identifier: "ja_JP")
        if let timezone = TimeZone(identifier: snapshot.document.timezone) {
            calendar.timeZone = timezone
        }
        return calendar
    }

    private var monthComponents: DateComponents {
        calendar.dateComponents([.year, .month], from: displayedMonth)
    }

    private var monthTitle: String {
        "\(monthComponents.year ?? 0)年 \(monthComponents.month ?? 0)月"
    }

    private var daysInMonth: Int {
        guard let date = calendar.date(from: monthComponents),
              let range = calendar.range(of: .day, in: .month, for: date) else {
            return 0
        }
        return range.count
    }

    private var firstDayOffset: Int {
        guard let firstDay = calendar.date(from: DateComponents(
            year: monthComponents.year,
            month: monthComponents.month,
            day: 1
        )) else {
            return 0
        }
        let weekday = calendar.component(.weekday, from: firstDay)
        return (weekday - calendar.firstWeekday + 7) % 7
    }

    private var selectedDaySessions: [Session] {
        guard let selectedDate else { return [] }
        return snapshot.document.sessions.filter { $0.date == selectedDate }
    }

    var body: some View {
        ScrollView {
            VStack(spacing: 14) {
                HStack {
                    monthButton(direction: .previous)
                    Spacer()
                    Text(monthTitle)
                        .font(.subheadline.weight(.black))
                        .foregroundStyle(RibbonTheme.textPrimary)
                    Spacer()
                    monthButton(direction: .next)
                }

                HStack(spacing: 0) {
                    ForEach(Array(["日", "月", "火", "水", "木", "金", "土"].enumerated()), id: \.offset) { index, title in
                        Text(title)
                            .font(.caption2.weight(.bold))
                            .foregroundStyle(index == 0 ? RibbonTheme.rose : index == 6 ? .blue : RibbonTheme.textSecondary)
                            .frame(maxWidth: .infinity)
                    }
                }
                .padding(.vertical, 9)
                .overlay(alignment: .bottom) {
                    Rectangle()
                        .fill(RibbonTheme.border)
                        .frame(height: 1)
                }

                LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 6), count: 7), spacing: 6) {
                    ForEach(0..<firstDayOffset, id: \.self) { _ in
                        Color.clear
                            .frame(height: 46)
                    }

                    ForEach(1...max(daysInMonth, 1), id: \.self) { day in
                        dayCell(day)
                    }
                }

                VStack(alignment: .leading, spacing: 8) {
                    if selectedDate != nil {
                        if selectedDaySessions.isEmpty {
                            Text("この日の公演はありません")
                                .font(.caption)
                                .foregroundStyle(RibbonTheme.textSecondary)
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, 14)
                        } else {
                            ForEach(selectedDaySessions, id: \.id) { session in
                                sessionSummary(session)
                            }
                        }
                    } else {
                        Text("日付をタップすると詳細が表示されます")
                            .font(.caption)
                            .foregroundStyle(RibbonTheme.textSecondary)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.top, 4)
                .overlay(alignment: .top) {
                    Rectangle()
                        .fill(RibbonTheme.border)
                        .frame(height: 1)
                }
            }
            .padding(.top, 10)
            .padding(.bottom, 12)
        }
        .scrollIndicators(.hidden)
    }

    private enum MonthDirection {
        case previous
        case next
    }

    private func monthButton(direction: MonthDirection) -> some View {
        let monthDelta = direction == .previous ? -1 : 1
        let adjacentMonth = calendar.date(byAdding: .month, value: monthDelta, to: displayedMonth)
        let adjacentTitle = adjacentMonth.map {
            "\(calendar.component(.month, from: $0))月"
        } ?? ""

        return Button {
            if let adjacentMonth {
                displayedMonth = adjacentMonth
                selectedDate = nil
            }
        } label: {
            HStack(spacing: 4) {
                if direction == .previous {
                    Image(systemName: "chevron.left")
                }
                Text(adjacentTitle)
                if direction == .next {
                    Image(systemName: "chevron.right")
                }
            }
            .font(.caption2.weight(.bold))
            .foregroundStyle(RibbonTheme.textSecondary)
            .padding(.horizontal, 11)
            .padding(.vertical, 7)
            .background(RibbonTheme.surface, in: RoundedRectangle(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10).stroke(RibbonTheme.border, lineWidth: 1))
        }
        .buttonStyle(.plain)
    }

    private func dayCell(_ day: Int) -> some View {
        let date = dateString(day: day)
        let daySessions = snapshot.document.sessions.filter { $0.date == date }
        let hasLive = !daySessions.isEmpty
        let isSelected = selectedDate == date

        return Button {
            if hasLive {
                selectedDate = date
            }
        } label: {
            VStack(spacing: 4) {
                Text("\(day)")
                    .font(.caption.monospacedDigit().weight(isSelected || hasLive ? .bold : .regular))
                Circle()
                    .fill(isSelected ? RibbonTheme.background : hasLive ? RibbonTheme.rose : .clear)
                    .frame(width: 6, height: 6)
            }
            .foregroundStyle(isSelected ? RibbonTheme.background : hasLive ? RibbonTheme.textPrimary : RibbonTheme.textSecondary.opacity(0.55))
            .frame(maxWidth: .infinity)
            .frame(height: 46)
            .background(
                isSelected
                    ? AnyShapeStyle(RibbonTheme.rose)
                    : hasLive
                        ? AnyShapeStyle(Color(red: 0.231, green: 0.039, blue: 0.078).opacity(0.72))
                        : AnyShapeStyle(RibbonTheme.surface.opacity(0.30)),
                in: RoundedRectangle(cornerRadius: 11)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 11)
                    .stroke(hasLive && !isSelected ? RibbonTheme.roseStrong.opacity(0.60) : .clear, lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
        .disabled(!hasLive)
    }

    private func dateString(day: Int) -> String {
        String(
            format: "%04d-%02d-%02d",
            monthComponents.year ?? 0,
            monthComponents.month ?? 0,
            day
        )
    }

    private func sessionSummary(_ session: Session) -> some View {
        Button {
            onSelectSession(session.id)
        } label: {
            VStack(alignment: .leading, spacing: 7) {
                HStack {
                    Text("\(session.date) \(session.sessionName.map { "[\($0)]" } ?? "")")
                        .font(.caption.monospacedDigit().weight(.bold))
                        .foregroundStyle(RibbonTheme.rose)
                    Spacer()
                    Text(session.venue.name)
                        .font(.caption2)
                        .foregroundStyle(RibbonTheme.textSecondary)
                        .lineLimit(1)
                }

                Text(session.title)
                    .font(.subheadline.weight(.bold))
                    .foregroundStyle(RibbonTheme.textPrimary)
                    .multilineTextAlignment(.leading)

                Divider()
                    .overlay(RibbonTheme.border)

                HStack {
                    Text("出演: \(session.live.startTime)〜")
                    Spacer()
                    Text("特典会: \(session.benefit.startTime)〜")
                }
                .font(.caption2.monospacedDigit())
                .foregroundStyle(RibbonTheme.textSecondary)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(12)
            .overlay(alignment: .leading) {
                Rectangle()
                    .fill(RibbonTheme.rose)
                    .frame(width: 4)
            }
            .ribbonCard(cornerRadius: 12)
        }
        .buttonStyle(.plain)
    }
}
