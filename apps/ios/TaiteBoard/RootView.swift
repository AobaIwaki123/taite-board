import SwiftUI
import TaiteBoardCore

enum RibbonTheme {
    static let background = Color(red: 0.047, green: 0.039, blue: 0.035)
    static let surface = Color(red: 0.110, green: 0.098, blue: 0.090)
    static let surfaceElevated = Color(red: 0.145, green: 0.129, blue: 0.118)
    static let rose = Color(red: 0.984, green: 0.443, blue: 0.506)
    static let roseStrong = Color(red: 0.956, green: 0.247, blue: 0.369)
    static let purple = Color(red: 0.659, green: 0.333, blue: 0.969)
    static let textPrimary = Color(red: 0.980, green: 0.976, blue: 0.965)
    static let textSecondary = Color(red: 0.690, green: 0.667, blue: 0.639)
    static let border = Color.white.opacity(0.08)
    static let shadow = Color.black.opacity(0.24)
}

enum MainTab: Int, CaseIterable, Hashable {
    case next
    case list
    case calendar

    var icon: String {
        switch self {
        case .next: "sparkles"
        case .list: "list.bullet"
        case .calendar: "calendar"
        }
    }
}

struct RootView: View {
    @ObservedObject var model: AppModel
    @Environment(\.scenePhase) private var scenePhase

    @State private var activeTab: MainTab = .next
    @State private var selectedSessionID: String?
    @State private var previousTab: MainTab = .list
    @State private var selectedImageIndex = 0
    @State private var isDataStatusPresented = false

    private var nextSession: Session? {
        guard let snapshot = model.snapshot else { return nil }
        return NextSessionResolver().resolve(in: snapshot.document)
    }

    private var currentSession: Session? {
        guard let snapshot = model.snapshot else { return nil }
        if let selectedSessionID {
            return snapshot.document.sessions.first { $0.id == selectedSessionID }
        }
        return nextSession ?? snapshot.document.sessions.first
    }

    private var isViewingSelectedSession: Bool {
        guard let selectedSessionID, let nextSession else { return false }
        return selectedSessionID != nextSession.id
    }

    var body: some View {
        ZStack {
            RibbonTheme.background.ignoresSafeArea()

            if let snapshot = model.snapshot {
                VStack(spacing: 0) {
                    RibbonHeaderView(
                        freshnessLabel: freshnessLabel(for: snapshot),
                        onLogoClick: resetToNextLive,
                        onDataStatusClick: { isDataStatusPresented = true }
                    )

                    TabView(selection: $activeTab) {
                        NextView(
                            snapshot: snapshot,
                            session: currentSession,
                            isNextLive: !isViewingSelectedSession,
                            selectedImageIndex: selectedImageIndex,
                            onSelectImageIndex: { selectedImageIndex = $0 },
                            onSelectSession: selectSession,
                            onResetToNextLive: resetToNextLive,
                            onBackToList: { activeTab = previousTab }
                        )
                        .tag(MainTab.next)

                        ScheduleListView(
                            snapshot: snapshot,
                            onSelectSession: selectSession
                        )
                        .tag(MainTab.list)

                        CalendarView(
                            snapshot: snapshot,
                            onSelectSession: selectSession
                        )
                        .tag(MainTab.calendar)
                    }
                    .tabViewStyle(.page(indexDisplayMode: .never))
                    .background(RibbonTheme.background)

                    RibbonNavigationBar(
                        activeTab: activeTab,
                        isViewingSelectedLive: isViewingSelectedSession,
                        onSelectTab: { activeTab = $0 }
                    )
                }
                .padding(.horizontal, 14)
            } else {
                ContentUnavailableView(
                    "予定表を読み込めません",
                    systemImage: "calendar.badge.exclamationmark",
                    description: Text("同梱データを確認して、もう一度お試しください。")
                )
                .foregroundStyle(RibbonTheme.textPrimary)
            }
        }
        .preferredColorScheme(.dark)
        .overlay(alignment: .top) {
            if model.isLoading {
                ProgressView()
                    .tint(RibbonTheme.rose)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 8)
                    .background(RibbonTheme.surfaceElevated, in: Capsule())
                    .overlay(Capsule().stroke(RibbonTheme.border, lineWidth: 1))
                    .padding(.top, 8)
            }
        }
        .safeAreaInset(edge: .bottom) {
            if let errorMessage = model.errorMessage {
                HStack(spacing: 8) {
                    Image(systemName: "exclamationmark.triangle.fill")
                    Text(errorMessage)
                        .lineLimit(2)
                    Spacer(minLength: 0)
                    Button("閉じる") {
                        model.clearError()
                    }
                    .buttonStyle(.borderless)
                }
                .font(.footnote)
                .foregroundStyle(.white)
                .padding(.horizontal, 16)
                .padding(.vertical, 10)
                .background(Color.red.opacity(0.9))
            }
        }
        .task {
            await model.bootstrapIfNeeded()
        }
        .onChange(of: scenePhase) { _, phase in
            guard phase == .active, model.snapshot != nil else { return }
            Task {
                await model.refreshIfNeeded(reason: .foreground)
            }
        }
        .sheet(isPresented: $isDataStatusPresented) {
            NavigationStack {
                DataStatusView(model: model)
            }
        }
    }

    private func selectSession(_ id: String) {
        previousTab = activeTab == .next ? previousTab : activeTab
        selectedSessionID = id
        selectedImageIndex = 0
        activeTab = .next
    }

    private func resetToNextLive() {
        selectedSessionID = nil
        selectedImageIndex = 0
        activeTab = .next
    }

    private func freshnessLabel(for snapshot: RepositorySnapshot) -> String? {
        switch snapshot.origin {
        case .remote:
            return "オンライン"
        case .diskCache, .seed:
            return "オフライン保存版を表示中"
        }
    }
}

struct RibbonHeaderView: View {
    let freshnessLabel: String?
    let onLogoClick: () -> Void
    let onDataStatusClick: () -> Void

    @State private var isPopping = false

    var body: some View {
        HStack(spacing: 8) {
            Button {
                isPopping = true
                onLogoClick()
                Task { @MainActor in
                    try? await Task.sleep(for: .milliseconds(450))
                    isPopping = false
                }
            } label: {
                HStack(spacing: 8) {
                    Image("RibbonClockIcon")
                        .resizable()
                        .scaledToFit()
                        .frame(width: 30, height: 30)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                        .scaleEffect(isPopping ? 1.18 : 1)
                        .rotationEffect(.degrees(isPopping ? -8 : 0))
                        .accessibilityLabel("リボンクロック")
                    Text("リボンクロック")
                        .font(.headline.weight(.heavy))
                        .tracking(0.4)
                        .foregroundStyle(RibbonTheme.textPrimary)
                }
                .padding(.vertical, 5)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)

            Spacer(minLength: 8)

            if let freshnessLabel {
                HStack(spacing: 6) {
                    Circle()
                        .fill(Color(red: 0.984, green: 0.749, blue: 0.141))
                        .frame(width: 6, height: 6)
                    Text(freshnessLabel)
                        .font(.caption2.weight(.bold))
                        .lineLimit(1)
                }
                .foregroundStyle(Color(red: 1.0, green: 0.827, blue: 0.361))
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
                .background(Color.orange.opacity(0.15), in: Capsule())
                .overlay(Capsule().stroke(Color.orange.opacity(0.30), lineWidth: 1))
            }

            Button(action: onDataStatusClick) {
                Image(systemName: "info.circle")
                    .font(.subheadline.weight(.bold))
                    .foregroundStyle(RibbonTheme.textSecondary)
                    .frame(width: 32, height: 32)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("データ状態")
        }
        .padding(.vertical, 4)
        .overlay(alignment: .bottom) {
            Rectangle()
                .fill(RibbonTheme.border)
                .frame(height: 1)
        }
    }
}

struct RibbonNavigationBar: View {
    let activeTab: MainTab
    let isViewingSelectedLive: Bool
    let onSelectTab: (MainTab) -> Void

    var body: some View {
        GeometryReader { proxy in
            let horizontalPadding: CGFloat = 4
            let tabWidth = (proxy.size.width - horizontalPadding * 2) / 3

            ZStack(alignment: .leading) {
                RoundedRectangle(cornerRadius: 16)
                    .fill(RibbonTheme.surface)
                    .overlay(RoundedRectangle(cornerRadius: 16).stroke(RibbonTheme.border, lineWidth: 1))

                RoundedRectangle(cornerRadius: 12)
                    .fill(
                        LinearGradient(
                            colors: [RibbonTheme.roseStrong, RibbonTheme.rose],
                            startPoint: .leading,
                            endPoint: .trailing
                        )
                    )
                    .frame(width: tabWidth, height: proxy.size.height - 8)
                    .offset(x: horizontalPadding + CGFloat(activeTab.rawValue) * tabWidth)
                    .animation(.easeOut(duration: 0.2), value: activeTab)

                HStack(spacing: 0) {
                    navigationButton(
                        tab: .next,
                        title: isViewingSelectedLive ? "公演詳細" : "直近ライブ"
                    )
                    navigationButton(tab: .list, title: "全日程")
                    navigationButton(tab: .calendar, title: "カレンダー")
                }
                .padding(.horizontal, horizontalPadding)
            }
        }
        .frame(height: 56)
        .padding(.top, 8)
        .padding(.bottom, 10)
        .background(RibbonTheme.background.opacity(0.92))
    }

    private func navigationButton(tab: MainTab, title: String) -> some View {
        Button {
            onSelectTab(tab)
        } label: {
            Label(title, systemImage: tab.icon)
                .font(.caption.weight(.bold))
                .labelStyle(.titleAndIcon)
                .foregroundStyle(activeTab == tab ? RibbonTheme.background : RibbonTheme.textSecondary)
                .frame(maxWidth: .infinity)
                .frame(height: 48)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

extension View {
    func ribbonCard(cornerRadius: CGFloat = 16) -> some View {
        background(
            RoundedRectangle(cornerRadius: cornerRadius)
                .fill(RibbonTheme.surface.opacity(0.92))
        )
        .overlay(
            RoundedRectangle(cornerRadius: cornerRadius)
                .stroke(RibbonTheme.border, lineWidth: 1)
        )
        .clipShape(RoundedRectangle(cornerRadius: cornerRadius))
        .shadow(color: RibbonTheme.shadow, radius: 8, y: 4)
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
