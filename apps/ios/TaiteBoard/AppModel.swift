import Foundation
import SwiftUI
import TaiteBoardCore

@MainActor
final class AppModel: ObservableObject {
    @Published private(set) var snapshot: RepositorySnapshot?
    @Published private(set) var errorMessage: String?
    @Published private(set) var isLoading = false

    let repository: ScheduleRepository
    private var hasBootstrapped = false

    init(repository: ScheduleRepository, initialSnapshot: RepositorySnapshot? = nil) {
        self.repository = repository
        self.snapshot = initialSnapshot
    }

    func bootstrapIfNeeded() async {
        guard !hasBootstrapped else { return }
        hasBootstrapped = true
        await bootstrap()
    }

    func bootstrap() async {
        guard !isLoading else { return }
        isLoading = true
        defer { isLoading = false }

        do {
            snapshot = try await repository.bootstrap()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func refresh() async {
        guard !isLoading else { return }
        isLoading = true
        defer { isLoading = false }

        do {
            snapshot = try await repository.refresh(reason: .manual)
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func clearError() {
        errorMessage = nil
    }
}
