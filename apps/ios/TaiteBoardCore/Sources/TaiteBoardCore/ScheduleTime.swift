import Foundation

public struct SessionTime: Equatable, Sendable {
    public let rawValue: String

    public init(_ rawValue: String) {
        self.rawValue = rawValue
    }

    public var isUnknown: Bool {
        rawValue.isEmpty || rawValue == "--:--"
    }

    public var dateComponents: DateComponents? {
        guard !isUnknown else { return nil }
        let parts = rawValue.split(separator: ":")
        guard parts.count == 2,
              let hour = Int(parts[0]),
              let minute = Int(parts[1]),
              (0...23).contains(hour),
              (0...59).contains(minute) else {
            return nil
        }
        return DateComponents(hour: hour, minute: minute)
    }
}

public struct NextSessionResolver: Sendable {
    public init() {}

    public func resolve(in document: ScheduleDocument, now: Date = Date()) -> Session? {
        let timezone = TimeZone(identifier: document.timezone) ?? .autoupdatingCurrent
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timezone

        if let currentNextId = document.currentNextId,
           let current = document.sessions.first(where: { $0.id == currentNextId }),
           !isPast(current, now: now, calendar: calendar) {
            return current
        }

        return document.sessions
            .filter { !isPast($0, now: now, calendar: calendar) }
            .sorted { sortKey($0) < sortKey($1) }
            .first
    }

    private func isPast(_ session: Session, now: Date, calendar: Calendar) -> Bool {
        guard let sessionDate = date(for: session.date, time: nil, calendar: calendar) else {
            return true
        }

        let today = calendar.startOfDay(for: now)
        if sessionDate < today {
            return true
        }
        if sessionDate > today {
            return false
        }

        let endTimes = [session.live.endTime, session.benefit.endTime].compactMap { $0 }
        let parsedEnds = endTimes.compactMap { date(for: session.date, time: $0, calendar: calendar) }
        guard !parsedEnds.isEmpty else {
            return false
        }

        guard let latestEnd = parsedEnds.max() else {
            return false
        }
        return latestEnd < now
    }

    private func sortKey(_ session: Session) -> String {
        let time = SessionTime(session.live.startTime).dateComponents
            .flatMap { components in
                guard let hour = components.hour, let minute = components.minute else { return nil }
                return String(format: "%02d:%02d", hour, minute)
            } ?? "99:99"
        return "\(session.date)T\(time)-\(session.id)"
    }

    private func date(for dateString: String, time: String?, calendar: Calendar) -> Date? {
        let dateParts = dateString.split(separator: "-").compactMap { Int($0) }
        guard dateParts.count == 3 else { return nil }

        let timeComponents = time.map(SessionTime.init)?.dateComponents
        var components = DateComponents(
            calendar: calendar,
            timeZone: calendar.timeZone,
            year: dateParts[0],
            month: dateParts[1],
            day: dateParts[2]
        )
        components.hour = timeComponents?.hour ?? 0
        components.minute = timeComponents?.minute ?? 0
        components.second = 0
        return calendar.date(from: components)
    }
}
