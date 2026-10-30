# iOS ネイティブクライアント設計案

## 位置づけ

- ステータス: Proposed
- 作成日: 2026-10-30
- 対象: `apps/ios/` に追加する Ribbon Clock の iOS クライアント
- 前提: `schedule.json` を配信する既存のデータパイプラインと Web クライアントは存続させる

この文書は、現在進行中の Web フロントエンドの Preact 移行とは独立して、iOS ネイティブ版の責務・データ境界・段階的な実装順序を定義する。iOS 版は Web の DOM や CSS を再利用せず、既存の公開データ契約だけを共有する。

## 1. 設計の結論

1. UI は SwiftUI で実装し、WebView や既存 Web UI のラッパーにはしない。
2. iOS クライアントは `schedule.json` の読み取り専用クライアントとする。X の取得、画像解析、名寄せ、キュレーションはサーバー側の既存パイプラインの責務に残す。
3. 起動直後の表示は、ディスクキャッシュまたはアプリ内同梱の seed を使って通信完了を待たずに行う。オンライン更新は表示後に行う。
4. キャッシュは小さな JSON スナップショットとして `Application Support` に保存し、SwiftData やデータベースは導入しない。
5. Web と iOS で共有するのは JSON Schema、fixture、公開 URL、鮮度の意味だけとし、UI コードや状態管理コードは共有しない。
6. `schema_version` と `generated_at` を必ず検査し、未知のスキーマや不正な更新で既存の正常なキャッシュを上書きしない。
7. メイン画面には「どのデータを表示しているか」「いつ生成されたか」を表示し、詳細画面では HTTP 状態、取得時間、レスポンス情報、検証結果、JSON 本文を端末内だけで確認できるようにする。

SwiftUI のアプリ入口には `App` プロトコルを使い、フォアグラウンド復帰は `ScenePhase` で検知する。リモート取得は `URLSession` の async API、キャッシュは `FileManager` の `Application Support` を使う。これらは Apple の公式ドキュメントに記載された標準 API である。

- [App | Apple Developer Documentation](https://developer.apple.com/documentation/swiftui/app)
- [ScenePhase | Apple Developer Documentation](https://developer.apple.com/documentation/swiftui/scenephase)
- [URLSession.data(for:) | Apple Developer Documentation](https://developer.apple.com/documentation/foundation/urlsession/data%28for%3A%29)
- [URL.applicationSupportDirectory | Apple Developer Documentation](https://developer.apple.com/documentation/foundation/url/applicationsupportdirectory)

## 2. 現状の把握と維持する境界

### 2.1 既存の構成

| 領域 | 現在の正本 | iOS 版での扱い |
| --- | --- | --- |
| 外部取得 | `internal/source/x/`, `cmd/taite-fetch` | iOS から直接呼ばない |
| raw データ | `data/raw/` と取得実装 | iOS から直接読まない |
| 名寄せ・補正 | `data/curated/` | iOS から直接読まない |
| 公開契約 | `schema/schedule.schema.json` | Swift の wire model と検証 fixture の正本 |
| 公開データ | `data/generated/schedule.json` | seed とテスト入力の基準 |
| 配信 | 静的 JSON / Nginx / Cloudflare | `RemoteScheduleSource` が読む |
| Web UI | `apps/web/` | UI コードは共有しない |

既存の ADR-0001、ADR-0005、ADR-0008 が決めている「公開データ契約」「静的配信」「キャッシュ・フォールバック・鮮度表示」は iOS 版にも適用する。

### 2.2 Web リファクタとの関係

現在のメイン作業ツリーでは、旧 DOM 操作型の画面から Preact の TSX コンポーネントへの移行が進んでいる。ただし、現時点では状態管理、データ契約、カレンダー基準日、Promotion のフィールドが整理途中であり、iOS 版を Web の実装状態に結合させない。

iOS 着手前に、少なくとも次の契約差分を解消する。

- `data/generated/schedule.json` の `promotions` にある `type`、`badge`、`cta_label`、`priority`、`active` を JSON Schema と各クライアント型へ反映する。
- Web の `Promotion` 型と `PromotionBanner` に残るハードコード値を公開データの値へ寄せる。
- Web の `TODAY_STR` やカレンダー初期月の固定値を、データの `timezone` と端末時刻から計算する。
- iOS では `current_next_id` が欠落・不整合の場合のフォールバック規則を、Web と共通の fixture で固定する。

## 3. スコープ

### 3.1 iOS v1 の対象

- 直近ライブの計器盤表示
- 全日程一覧
- 月間カレンダーと日付ごとの公演一覧
- 公演詳細、フライヤー・タイテ画像の切り替えと拡大表示
- 会場マップ、チケット、出典 URL の外部アプリ起動
- オフライン時の最後に有効だったデータ表示
- seed、キャッシュ、リモート更新のデータ状態表示
- Dynamic Type、VoiceOver、外部リンクのアクセシビリティ対応

### 3.2 初期対象外

- ログイン、ユーザーアカウント、サーバー保存のお気に入り
- Push 通知、BackgroundTasks による定期取得
- X への投稿取得、OCR、LLM、名寄せ、データ編集
- チケット購入のアプリ内埋め込み
- 画像の完全なオフライン保存
- Web と iOS の UI コード共有
- SwiftData、SQLite、常時稼働 API サーバー

Push 通知や個人設定が必要になった時点で、ADR-0005 が定める API 化条件と合わせて別の設計判断を行う。

## 4. コンポーネント構成

```text
SwiftUI Views
    ↓ 画面状態の読み書き
AppModel (@MainActor)
    ↓ bootstrap / refresh / select
ScheduleRepository (actor)
    ├── SeedScheduleSource       Bundle の schedule.json
    ├── DiskScheduleCache        Application Support の snapshot
    └── RemoteScheduleSource     URLSession で静的 JSON を取得
            ↓
       ScheduleDocument (Codable / Sendable)
            ↓
   FreshnessPolicy / NextSessionResolver / DateParser
```

### 4.1 AppModel

`AppModel` は SwiftUI に公開する画面状態だけを保持する `@MainActor` のモデルとする。

保持する状態の例:

- `document: ScheduleDocument?`
- `selectedSessionID: String?`
- `selectedTab`
- `calendarMonth`
- `imageIndex`
- `dataOrigin`: `remote` / `diskCache` / `seed`
- `freshness`: `fresh` / `stale` / `offline` / `unknown`
- `isRefreshing`
- `lastRefreshDiagnostics`

`AppModel` はファイル I/O や JSON デコードを直接行わず、`ScheduleRepository` の結果を UI 状態へ変換する。Swift 6 の並行性検査に合わせ、画面状態の更新は MainActor、データ取得とキャッシュ操作は actor 境界の内側で行う。

### 4.2 ScheduleRepository

Repository は次の二つの操作を公開する。

```text
bootstrap() async -> BootstrapResult
refresh(reason: RefreshReason) async -> RefreshResult
```

`bootstrap` は次の優先順で最初の表示データを決める。

1. 検証済みのディスクキャッシュ
2. アプリバンドル内の seed
3. どちらも無い場合のリカバリー画面

`refresh` は通信に成功した場合だけ新しい snapshot を採用する。通信エラー、HTTP エラー、JSON デコードエラー、未知の `schema_version`、ドメイン検証エラーのいずれでも、既存の有効なデータを破棄しない。

### 4.3 RemoteScheduleSource

- URL は `Info.plist` またはビルド設定から注入し、コード内に本番 URLを固定しない。
- `URLRequest` にタイムアウト、`Cache-Control: no-cache`、必要な `Accept: application/json` を設定する。
- `URLSession.data(for:)` の結果から HTTP ステータス、主要レスポンスヘッダー、取得時間、ペイロードサイズを `RequestDiagnostics` に記録する。
- 2xx 以外は更新失敗とする。受け取った JSON のデコードとドメイン検証が完了してからキャッシュへ書き込む。
- X、チケットサイト、画像ホストへスケジュール取得のためのリクエストを追加で送らない。

## 5. キャッシュと鮮度

### 5.1 保存形式

`Application Support/TaiteBoard/` に次のファイルを保存する。

```text
schedule.json       # 最後に検証を通過した公開データ
cache-metadata.json  # 取得元、取得時刻、HTTP情報、schema_version
```

保存は一時ファイルへの書き込み後に atomic replace し、アプリ終了や電波断の途中で壊れた JSON を正本として残さない。Application Support はアプリ内部のサポートファイル向けであり、ユーザーに見せる Documents には置かない。

`generated_at` と `cached_at` は別の値として扱う。

- `generated_at`: データパイプラインがその予定表を生成した時刻。ユーザーに知らせるデータの鮮度。
- `cached_at`: この端末が最後に正常取得した時刻。通信状態の説明に使う。

### 5.2 更新フロー

1. 起動時はキャッシュまたは seed を表示する。
2. UI 表示後にリモート更新を開始する。
3. アプリが `scenePhase == active` に戻ったとき、前回更新から一定時間が経過していれば更新する。
4. Pull to Refresh とデータ状態画面の再試行で明示的な更新も可能にする。
5. 成功時は JSON をデコード・検証してからキャッシュと UI を一括更新する。
6. 失敗時は表示中のデータを維持し、失敗理由と最終更新情報を表示する。

更新間隔の具体値は実装時に決めるが、同じ画面表示中の無制限な再取得は行わない。静的 JSON は小さいため、最初の実装では複雑な同期キューやデータベースを追加しない。

### 5.3 鮮度表示

既存 ADR-0008 に合わせ、`generated_at` が 24 時間未満なら通常表示、24 時間以上なら控えめな stale 表示を出す。通信に失敗してキャッシュまたは seed を表示している場合は、鮮度とは別に `offline` または `seed` を明示する。

表示例:

- オンライン: `最終生成 10/30 09:00`
- キャッシュ表示: `オフライン保存版・最終取得 10/30 09:02`
- seed 表示: `初期同梱データ・通信後に更新します`
- stale: `2日前の情報です。公式告知も確認してください`

## 6. データ契約とモデル

### 6.1 Wire model と Domain model の分離

`schema/schedule.schema.json` に対応する `ScheduleDocumentDTO` は通信・保存用の Codable モデルとする。画面が直接 DTO を解釈せず、次のドメイン層を通す。

- `ScheduleDocument`: 検証済みの公開データ
- `SessionTime`: `--:--` など未確定値を失わない時間表現
- `FreshnessState`: 鮮度とデータ取得元を統合した表示状態
- `NextSessionResolver`: `current_next_id` と現在時刻から直近公演を決める

日付・時刻を JSON デコード時に無条件で `Date` へ変換しない。現行データには告知待ちを表す `--:--` があるため、元の文字列を保持したうえで、表示や比較が可能な場合だけ `DateComponents` に変換する。比較には schedule の `timezone` を使い、端末のタイムゾーンを暗黙に正本としない。

### 6.2 スキーマ整合性

現在の `data/generated/schedule.json` は `promotions` に JSON Schema で明記されていない表示フィールドを含んでいる。iOS でフィールドが静かに失われる状態を許容しない。

実装前の P0 作業:

1. 実データで利用している Promotion フィールドを Schema に追加する。
2. `apps/web/src/types/schedule.ts` と Swift の wire model を更新する。
3. `additionalProperties` を厳格化できるか、既存データ全体を検証して決める。
4. `schema_version: 1` の fixture を Web、Go、iOS のテストから共通利用する。
5. 破壊的な変更は `schema_version` を上げ、旧クライアントが安全に扱える移行期間を設ける。

モデルの手書き複製を長期運用しない。JSON Schema を入力に wire model を生成する手段を評価し、生成物と生成コマンドを CI で検証する。生成ツールの選定を保留したまま iOS UI だけを先に広げない。

## 7. iOS の画面構成

Web の情報設計は維持するが、レイアウトと操作は SwiftUI のネイティブコンポーネントで組み直す。

```text
RootView
├── NextView                 # 直近ライブ / 選択中公演の詳細
├── ScheduleListView         # 全日程
└── CalendarView             # 月間カレンダー

SessionDetailView
├── SessionHeader
├── TimeCard
├── VenueAndTicketLinks
├── ImageGallery
└── PromotionCard

DataStatusView               # 鮮度、取得元、通信診断、JSON確認
```

- ルートは `TabView` または同等のアクセシブルな選択コンテナで構成する。
- 長い内容は各画面の `ScrollView` / `LazyVStack` で独立してスクロールさせる。
- 公演詳細は `NavigationStack` で遷移し、一覧やカレンダーから選択できるようにする。
- 会場、チケット、出典は `openURL` で Maps、Safari、チケットアプリ等へ渡す。アプリ内 WebView は使わない。
- 画像は `AsyncImage` 相当の非同期表示と明示的な失敗・プレースホルダーを持たせる。初期版では画像の完全なオフライン保証をしない。
- Dynamic Type、VoiceOver の読み上げ順、ボタンのアクセシビリティラベル、十分なコントラストを受け入れ条件に含める。

Apple の `AsyncImage` は非同期ロードと読み込み中・失敗時の表示を提供するが、画像の永続的オフライン利用をこの設計の前提にはしない。画像キャッシュが必要になった場合は、schedule JSON キャッシュとは別の容量・削除ポリシーを設計する。

- [AsyncImage | Apple Developer Documentation](https://developer.apple.com/documentation/swiftui/asyncimage)

## 8. データ透明性とプライバシー

### 8.1 DataStatusView

通常画面では情報密度を保ちつつ、ヘッダーまたは設定メニューからデータ状態画面を開けるようにする。表示項目は次のとおりとする。

- feed URL
- データの取得元: remote / disk cache / seed
- `generated_at`、`cached_at`
- `schema_version`
- 最終 HTTP ステータス
- RTT、ペイロードサイズ
- `Content-Type`、`Content-Length`、`ETag`、`Last-Modified` が存在する場合の値
- JSON 検証結果とエラー理由
- 整形した raw JSON の表示・コピー・共有

これらは端末内にだけ表示し、外部へ送信しない。デバッグログを有効にする場合も `Logger` の出力先は端末内の開発者向けログに限定し、個人識別子や閲覧履歴は記録しない。

### 8.2 権限と外部通信

- アカウント、広告 ID、分析 SDK、Cookie、UUID は追加しない。
- 必須通信は schedule feed と、ユーザーが明示的に開いた画像・外部リンクに限定する。
- 画像や外部サイトの失敗を、schedule JSON の表示失敗と同一視しない。
- App Transport Security の標準設定を維持し、HTTP のチケット URLは表示ではなく外部アプリへ委譲する。

## 9. リポジトリ配置案

```text
apps/ios/
├── TaiteBoard.xcodeproj/
├── TaiteBoard/
│   ├── App/
│   │   ├── TaiteBoardApp.swift
│   │   ├── AppModel.swift
│   │   └── AppEnvironment.swift
│   ├── Domain/
│   │   ├── ScheduleDocument.swift
│   │   ├── SessionTime.swift
│   │   ├── Freshness.swift
│   │   └── NextSessionResolver.swift
│   ├── Data/
│   │   ├── ScheduleRepository.swift
│   │   ├── RemoteScheduleSource.swift
│   │   ├── DiskScheduleCache.swift
│   │   ├── SeedScheduleSource.swift
│   │   └── RequestDiagnostics.swift
│   ├── Features/
│   │   ├── Next/
│   │   ├── ScheduleList/
│   │   ├── Calendar/
│   │   ├── SessionDetail/
│   │   └── DataStatus/
│   ├── SharedUI/
│   └── Resources/
│       └── schedule.json        # ビルド時に data/generated から同期
└── TaiteBoardTests/
    ├── Fixtures/
    ├── ScheduleRepositoryTests.swift
    ├── FreshnessTests.swift
    └── NextSessionResolverTests.swift
```

Swift Package Manager の外部依存は v1 では追加しない。Xcode プロジェクトは iOS アプリのビルド、テスト、署名設定を持ち、`Resources/schedule.json` は手編集せず、リポジトリの生成データから同期するスクリプトまたは Xcode の Run Script Build Phase で更新する。

## 10. 段階的な実装順序

### Phase 0: 契約固定

- Promotion の Schema と TypeScript 型の不一致を解消する。
- `schema_version: 1` の正常、空予定、告知待ち、同日複数公演、古い生成時刻、未知フィールドの fixture を作る。
- `current_next_id` が無い・存在しない・過去公演を指す場合の選択規則を固定する。
- wire model の生成方法または CI drift check を決める。

完了条件: Go の検証と Web の型検査に加え、fixture の契約が iOS 実装前に確定している。

### Phase 1: iOS のデータ基盤

- Xcode プロジェクト、SwiftUI の入口、seed 読み込みを作る。
- `ScheduleDocument` の decode と schema version gate を作る。
- `DiskScheduleCache` と atomic replace を作る。
- fake source を使った Repository テストを作る。

完了条件: ネットワークを使わず、初回起動と再起動の両方で予定表を表示できる。

### Phase 2: リモート更新と透明性

- `URLSession` の remote source を作る。
- 起動後、foreground 復帰、Pull to Refresh の更新を接続する。
- stale、offline、seed、schema error を表示する。
- DataStatusView とローカル診断情報を作る。

完了条件: 更新成功、タイムアウト、HTTP エラー、不正 JSON、未知スキーマの各ケースで、最後の正常データが保護される。

### Phase 3: 主要画面

- Next、一覧、カレンダー、詳細を実装する。
- 会場・チケット・出典リンクを外部アプリへ渡す。
- 画像のプレースホルダー、失敗表示、拡大表示を実装する。

完了条件: Web の主要 fixture と同じ公演内容が表示され、0.5 秒で重要な時刻・会場・特典会を判読できるレイアウトになっている。

### Phase 4: 品質確認

- Dynamic Type と VoiceOver の確認。
- 端末サイズ別の UI テスト。
- 機内モード相当の offline 起動テスト。
- `xcodebuild test`、Go 検証、Web 型検査、seed 同期の CI 化。

完了条件: リリース候補を実機で確認し、未確認の外部画像・チケットリンクを明示できる。

## 11. 検証項目

| 観点 | 確認内容 |
| --- | --- |
| 契約 | Schema v1 の fixture を decode し、未知 version は拒否する |
| 起動 | ネットワークなしの初回起動で seed が表示される |
| キャッシュ | remote 成功後に通信を切って最後の正常データが表示される |
| 保護 | 不正な remote JSON で既存キャッシュが上書きされない |
| 鮮度 | `generated_at` と `cached_at` が混同されず、24時間境界が正しい |
| 時刻 | `Asia/Tokyo` と告知待ち `--:--` を保持する |
| 選択 | 同日複数公演、過去公演、`current_next_id` 欠落を処理する |
| 透明性 | HTTP 状態、RTT、サイズ、検証結果、raw JSON を端末内で確認できる |
| プライバシー | 外部テレメトリ、広告 ID、アカウント、閲覧履歴送信がない |
| UI | Dynamic Type、VoiceOver、外部リンク、画像失敗状態を確認する |

ローカルの設計検証環境は Xcode 27.1、Swift 6.4、iOS SDK 27.1 である。現在の実行環境では CoreSimulatorService が利用できないため、シミュレータ UI テストは未実施であり、iOS プロジェクト作成後に実機または CI の simulator で確認する。

## 12. 未決定事項

- iOS の最低対応バージョン
- Bundle ID、署名チーム、App Store 配布の有無
- 本番 feed URL と staging feed URL
- Promotion の Schema 変更を `schema_version: 1` 内で行うか、v2 にするか
- wire model の生成ツール
- 画像を disk cache する時期と容量上限
- Push 通知を将来採用するか

上記は UI 実装の細部より先に決める。最低対応バージョンと feed URL が未確定でも、seed・cache・fixture を使ったデータ層の実装は開始できる。
