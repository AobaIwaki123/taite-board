import SwiftUI
import TaiteBoardCore

struct DataStatusView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        Group {
            if let snapshot = model.snapshot {
                List {
                    Section("表示中のデータ") {
                        LabeledContent("取得元", value: snapshot.origin.displayName)
                        LabeledContent("Schema", value: "v\(snapshot.document.schemaVersion)")
                        LabeledContent("生成時刻", value: snapshot.document.generatedAt)
                        if let cachedAt = snapshot.cachedAt {
                            LabeledContent("端末取得時刻", value: cachedAt.formatted(date: .abbreviated, time: .shortened))
                        }
                    }

                    if let diagnostics = snapshot.diagnostics {
                        Section("通信診断") {
                            LabeledContent("URL", value: diagnostics.url.absoluteString)
                            LabeledContent("HTTP", value: diagnostics.statusCode.map(String.init) ?? "-")
                            LabeledContent("応答サイズ", value: ByteCountFormatter.string(fromByteCount: Int64(diagnostics.payloadBytes), countStyle: .file))
                            LabeledContent("取得時間", value: String(format: "%.0f ms", diagnostics.duration * 1_000))
                            if let contentType = diagnostics.contentType {
                                LabeledContent("Content-Type", value: contentType)
                            }
                            if let etag = diagnostics.etag {
                                LabeledContent("ETag", value: etag)
                            }
                            if let lastModified = diagnostics.lastModified {
                                LabeledContent("Last-Modified", value: lastModified)
                            }
                        }
                    }

                    if let errorMessage = model.errorMessage {
                        Section("直近の更新エラー") {
                            Text(errorMessage)
                                .foregroundStyle(.red)
                        }
                    }

                    Section {
                        Button {
                            Task { await model.refresh() }
                        } label: {
                            Label("リモートから更新", systemImage: "arrow.clockwise")
                        }
                        .disabled(model.isLoading)
                    } footer: {
                        Text("更新に失敗しても、現在表示している正常なデータは維持されます。")
                    }
                }
            } else {
                ContentUnavailableView(
                    "予定表を読み込めません",
                    systemImage: "calendar.badge.exclamationmark",
                    description: Text("同梱データまたはリモート接続を確認してください。")
                )
            }
        }
        .navigationTitle("データ状態")
    }
}
