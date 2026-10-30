import SwiftUI
import TaiteBoardCore

struct RootView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        Group {
            if let snapshot = model.snapshot {
                TabView {
                    NavigationStack {
                        NextView(snapshot: snapshot)
                    }
                    .tabItem {
                        Label("次回", systemImage: "clock")
                    }

                    NavigationStack {
                        ScheduleListView(snapshot: snapshot)
                    }
                    .tabItem {
                        Label("日程", systemImage: "list.bullet")
                    }

                    NavigationStack {
                        CalendarView(snapshot: snapshot)
                    }
                    .tabItem {
                        Label("カレンダー", systemImage: "calendar")
                    }

                    NavigationStack {
                        DataStatusView(model: model, snapshot: snapshot)
                    }
                    .tabItem {
                        Label("データ", systemImage: "doc.text.magnifyingglass")
                    }
                }
            } else {
                ContentUnavailableView(
                    "予定表を読み込めません",
                    systemImage: "calendar.badge.exclamationmark",
                    description: Text("同梱データを確認して、もう一度お試しください。")
                )
            }
        }
        .overlay(alignment: .top) {
            if model.isLoading {
                ProgressView()
                    .padding(.horizontal, 14)
                    .padding(.vertical, 8)
                    .background(.regularMaterial, in: Capsule())
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
                .background(.red, in: Rectangle())
            }
        }
        .task {
            await model.bootstrapIfNeeded()
        }
    }
}
