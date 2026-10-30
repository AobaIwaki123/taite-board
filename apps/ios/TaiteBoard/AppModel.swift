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
        guard snapshot != nil else { return }
        await refresh(reason: .launch)
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
        await refresh(reason: .manual)
    }

    func refreshIfNeeded(reason: RefreshReason = .foreground) async {
        guard !isLoading else { return }
        isLoading = true
        defer { isLoading = false }

        do {
            snapshot = try await repository.refreshIfNeeded(reason: reason)
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func refresh(reason: RefreshReason) async {
        guard !isLoading else { return }
        isLoading = true
        defer { isLoading = false }

        do {
            snapshot = try await repository.refresh(reason: reason)
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func clearError() {
        errorMessage = nil
    }
}
