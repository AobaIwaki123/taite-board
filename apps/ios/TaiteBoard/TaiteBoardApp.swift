import SwiftUI

@main
struct TaiteBoardApp: App {
    @StateObject private var model: AppModel

    init() {
        _model = StateObject(wrappedValue: AppModel(repository: AppEnvironment.liveRepository()))
    }

    var body: some Scene {
        WindowGroup {
            RootView(model: model)
        }
    }
}
