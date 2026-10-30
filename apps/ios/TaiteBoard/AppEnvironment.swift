import Foundation
import TaiteBoardCore

enum AppEnvironment {
    static func liveRepository() -> ScheduleRepository {
        guard let seed = try? SeedScheduleSource(bundle: .main) else {
            fatalError("schedule.json is missing from the application bundle")
        }

        let remote = remoteSource()
        let applicationSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!.appendingPathComponent("TaiteBoard", isDirectory: true)

        return ScheduleRepository(
            seed: seed,
            remote: remote,
            cache: DiskScheduleCache(directory: applicationSupport)
        )
    }

    private static func remoteSource() -> (any ScheduleSource)? {
        guard let value = Bundle.main.object(forInfoDictionaryKey: "ScheduleFeedURL") as? String,
              let url = URL(string: value) else {
            return nil
        }
        return RemoteScheduleSource(url: url)
    }
}
