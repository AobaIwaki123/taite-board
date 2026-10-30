# Android ネイティブクライアント設計案

## 位置づけ

- ステータス: Proposed
- 対象: `apps/android/` に追加する Ribbon Clock の Android クライアント
- 前提: `schedule.json` を配信する既存のデータパイプラインと Web / iOS クライアントは存続させる
- 実装対象: Androidプロジェクトの作成前に、データ境界、状態管理、キャッシュ、画面、品質条件を固定する

この設計は、WebのDOMやiOSのSwiftUIコードを再利用せず、Web・iOSと同じ公開データ契約を利用するAndroidアプリを定義する。X投稿の取得、画像解析、AIパース、名寄せ、手動キュレーションはAndroidへ移さない。

## 1. 設計の結論

1. UIは Kotlin と Jetpack Compose を使った独立したネイティブクライアントとする。
2. Androidクライアントは `schedule.json` の読み取り専用クライアントとする。
3. 起動直後は、検証済みのディスクキャッシュまたはアプリ内seedを表示し、通信完了を待たない。
4. キャッシュは小さなJSON snapshotをアプリ専用ストレージへ保存し、v1ではRoomやSQLiteを導入しない。
5. 画面状態は ViewModel と StateFlow で管理し、Repository、Domain、UIの責務を分離する。
6. `schema_version`、`generated_at`、`timezone`、必須フィールドを検証し、異常なリモートデータで最後の正常なキャッシュを上書きしない。
7. Data Status画面で、取得元、生成時刻、取得時刻、HTTP結果、検証結果、raw JSONを端末内だけで確認できるようにする。
8. Androidのadaptive iconは `icon-maskable.svg` を基準にし、Web・iOSと異なるブランド図形を作らない。

Androidの最小SDK、Kotlin、Compose、Gradle、AGPの具体的なバージョンは、プロジェクト作成時に実行環境を確認して決める。設計段階で古いバージョンを固定しない。

## 2. 既存クライアントとの境界

| 領域 | 共有するもの | Android固有のもの |
| --- | --- | --- |
| データ契約 | `schema/schedule.schema.json`、`schema_version`、fixture | Kotlinのwire model、domain model |
| 公開データ | `https://ribonclock.iorin.net/schedule.json` | `RemoteScheduleSource` |
| 鮮度 | `generated_at` の意味、stale判定の基準 | Androidの状態表示、通知文 |
| キャッシュ | 最後の正常データを保護する方針 | `filesDir`、atomic replace、cache metadata |
| 名寄せ | 完成済みの `canonical_id` を読む | 名寄せ処理は実装しない |
| UI | 情報設計、fixture、受け入れ条件 | Compose、Material、AndroidのInsetsとIntent |
| ブランド | カレンダーとリボン、maskable安全領域 | adaptive icon、Android splash/icon設定 |

iOS版との共通点はデータ契約、seed、cache、鮮度、外部リンク、透明性であり、画面コードや状態管理コードは共有しない。

## 3. スコープ

### 3.1 Android v1の対象

- 直近公演の表示
- 全日程一覧
- 月間カレンダーと日付ごとの公演一覧
- 公演詳細、出演時間、特典会、会場、チケット、出典URL
- フライヤー・タイムテーブル画像の切り替えと拡大表示
- オフライン時の最後に有効だったデータ表示
- seed、cache、remoteのデータ状態表示
- Google Maps、ブラウザ、チケットアプリなどへの外部Intent起動
- TalkBack、フォントサイズ変更、コントラスト、タッチ領域への対応

### 3.2 初期対象外

- ログイン、ユーザーアカウント、サーバー同期のお気に入り
- Push通知、定期バックグラウンド取得
- X投稿取得、OCR、LLM、Cloud Run呼び出し、名寄せ、キュレーション
- チケット購入のアプリ内WebView
- 画像の完全なオフライン保存
- Web、iOS、Android間のUIコード共有
- Room、SQLite、常時稼働のAndroid側APIサーバー

Push通知や個人設定が必要になった場合は、公開JSONの静的配信を維持できるかを確認してから別設計に分ける。

## 4. Androidアーキテクチャ

```text
Compose UI
    ↓ state collection
Screen ViewModels
    ↓ user action / lifecycle event
ScheduleRepository
    ├── SeedScheduleSource       assets/schedule.json
    ├── DiskScheduleCache        filesDir/TaiteBoard/
    └── RemoteScheduleSource     HTTPS schedule.json
            ↓
       ScheduleDocumentDTO
            ↓ decode / validate
       ScheduleDocument
            ↓
   FreshnessPolicy / NextSessionResolver / SessionTime
```

### 4.1 UI層

- Composeの画面は状態を受け取り、ユーザー操作をViewModelへ渡す。
- UIコンポーネント内でHTTP、ファイルI/O、JSON decodeを行わない。
- `collectAsStateWithLifecycle` 相当のライフサイクル対応を使い、停止中の画面で不要な購読を続けない。
- `LazyColumn`、`LazyVerticalGrid`、`Navigation`などの標準コンポーネントを基本とする。
- 画面回転やプロセス再生成で失われてよい選択状態と、Repositoryから復元すべきデータ状態を分ける。

### 4.2 ViewModel

ViewModelは画面状態とイベント処理だけを担当し、Repositoryの結果をUI向けのsealed stateへ変換する。

状態の例:

- `LoadingFromCache`
- `Showing(dataOrigin = remote | diskCache | seed, freshness = fresh | stale | offline | unknown)`
- `Refreshing(previousData = ...)`
- `Failure(previousData = ..., reason = ...)`
- `NoUsableData(reason = ...)`

通信失敗時に、表示中の正常データを空状態へ戻さない。エラーは画面の状態と独立したdiagnosticsとして保持し、再試行操作を提供する。

### 4.3 Repository

Repositoryは次の操作を公開する。

```text
bootstrap() suspend -> BootstrapResult
refresh(reason: RefreshReason) suspend -> RefreshResult
```

`bootstrap` は次の優先順で最初の表示データを決める。

1. 検証済みのディスクキャッシュ
2. アプリ内 `assets/schedule.json` のseed
3. どちらもない場合のリカバリー画面

`refresh` は通信成功、JSON decode、schema検証、ドメイン検証のすべてが完了した場合だけキャッシュを置き換える。

## 5. データ取得とキャッシュ

### 5.1 RemoteScheduleSource

- feed URLは `BuildConfig` または環境別リソースから注入し、本番URLを画面コードへ直接書かない。
- HTTPSのみを許可し、クリアテキスト通信は有効化しない。
- `Accept: application/json`、接続・読み込みタイムアウト、HTTPステータス検査を設定する。
- 2xx以外、JSON decode失敗、未知のschema version、必須項目不備は更新失敗とする。
- X、チケットサイト、画像ホストをschedule取得のために直接巡回しない。
- HTTPステータス、主要ヘッダー、取得時間、RTT、ペイロードサイズをdiagnosticsへ渡す。

### 5.2 DiskScheduleCache

アプリ専用領域に次のファイルを保存する。

```text
filesDir/TaiteBoard/schedule.json
filesDir/TaiteBoard/cache-metadata.json
```

`cache-metadata.json` には、少なくとも次を保存する。

- `generated_at`
- `cached_at`
- `schema_version`
- data origin
- HTTP status
- ETag / Last-Modified が存在する場合の値
- payload size

書き込みはtemporary fileへ行ってからatomic renameする。壊れたJSON、途中まで書かれたmetadata、schema不一致のデータを最後の正常データとして扱わない。

### 5.3 更新タイミング

1. cold startではcacheまたはseedを即時表示する。
2. UI表示後にremote refreshを開始する。
3. アプリがforegroundへ復帰した際、前回更新から一定時間が経過していれば更新する。
4. Pull to Refresh相当の明示的な更新を提供する。
5. 成功時は検証完了後にデータ、metadata、UI状態を更新する。
6. 失敗時は現在の表示を維持し、理由と再試行操作を表示する。

v1では定期的なバックグラウンド取得を実装しない。必要になった場合はWorkManager導入とバッテリー・通信量への影響を別途評価する。

### 5.4 鮮度表示

`generated_at` と `cached_at` を混同しない。

- `generated_at`: サーバー側で予定表が生成された時刻
- `cached_at`: この端末が最後に正常取得した時刻

既存のADR-0008に合わせ、`generated_at` が24時間以上古い場合はstale表示を出す。通信失敗でcacheを表示している場合は、鮮度とは別にoffline状態を表示する。seedの場合は、まだ端末がremoteデータを取得していないことを明示する。

## 6. データモデルと契約検証

### 6.1 DTOとDomain model

通信・保存用の `ScheduleDocumentDTO` と、UIが参照する検証済み `ScheduleDocument` を分ける。

- `ScheduleDocument`: schema検証済みの公開データ
- `SessionTime`: `--:--` を保持できる時間表現
- `FreshnessState`: fresh、stale、offline、seed、unknown
- `NextSessionResolver`: `current_next_id` と現在時刻から直近公演を選ぶ
- `RequestDiagnostics`: HTTP、取得時間、payload、検証結果

日付・時刻をdecode時に常に端末のタイムゾーンへ変換しない。scheduleの `timezone` を基準にし、未確定時刻 `--:--` を失わない。

### 6.2 Schema version gate

次の場合は新しいデータを破棄し、既存cacheを維持する。

- `schema_version` が未対応
- `generated_at`、`timezone`、`target_group`が欠落
- session IDが重複
- 日付または時間の形式が不正
- 必須のvenue、live、ticket情報が欠落

Web、iOS、Androidが同じfixtureを使えるようにする。正常系だけでなく、次のfixtureを共通化する。

- 通常の予定
- 告知待ち `--:--`
- 同日複数公演
- 古い `generated_at`
- `current_next_id` 欠落・過去公演・未知ID
- 空のsessions
- 未知のschema version
- HTTP取得失敗後のcache表示

Promotionの追加フィールドなど、既存schemaと実データの不一致はAndroid側で黙って捨てず、Web・iOSと同じ契約修正を先に行う。

## 7. 画面構成

```text
MainActivity
└── RootScreen
    ├── NextScreen              # 直近ライブ
    ├── ScheduleListScreen      # 全日程
    ├── CalendarScreen          # 月間カレンダー
    └── DataStatusScreen        # 鮮度・取得診断・raw JSON

SessionDetailScreen
├── SessionHeader
├── TimeCard
├── VenueAndTicketLinks
├── ImageGallery
└── PromotionCard
```

- Androidの戻る操作はNavigationの標準動作へ委譲する。
- 外部リンクは `ACTION_VIEW` を使い、Maps、ブラウザ、チケットアプリへ渡す。対応アプリがない場合はブラウザへフォールバックする。
- 画像は非同期表示し、loading、empty、error、retryを持たせる。
- 画像の完全オフライン保存はv1の前提にしない。画像取得失敗でschedule本体を非表示にしない。
- 日付、時刻、会場、特典会を一覧で判読できる情報密度を優先し、装飾よりも現場での確認性を受け入れ条件にする。

## 8. Androidブランドアセット

Androidのadaptive iconは、Webの `apps/web/public/icons/icon-maskable.svg` をデザイン基準にする。

- background: `#3a1320`
- foreground: カレンダーとリボンのベクター図形
- 重要な図形: Androidのadaptive icon safe zone内へ配置
- legacy icon: `icon-512.png` またはAndroid向けに生成したPNGを使用
- Web、iOS、Androidで独自のリボン形状や色を作らない

AndroidではOSがアイコンを円形・角丸・スクワークル等へマスクするため、通常のWebアイコンをそのままadaptive iconのforegroundへ入れない。maskable用の安全領域を基準にforegroundとbackgroundを分離する。

ブランドアセットを更新した場合は、Webの通常アイコン、Webのmaskableアイコン、iOS AppIcon、iOSアプリ内ヘッダー画像、Androidのadaptive iconを同じ変更単位で確認する。画像の生成・同期を自動化する場合は、入力SVG、出力先、生成サイズ、検証結果を固定する。

## 9. データ透明性とプライバシー

DataStatusScreenで次の情報を端末内に表示する。

- feed URL
- data origin: remote / disk cache / seed
- `generated_at`、`cached_at`
- `schema_version`
- HTTP status、RTT、payload size
- `Content-Type`、`Content-Length`、ETag、Last-Modified
- JSON検証結果とエラー理由
- 整形したraw JSONの表示・コピー・共有

診断情報、閲覧履歴、識別子を外部へ送信しない。広告ID、分析SDK、Cookie、アカウント、独自UUIDを追加しない。ログは開発者向けの端末ログに限定し、投稿本文や画像を通常ログへ出さない。

## 10. リポジトリ配置案

```text
apps/android/
├── settings.gradle.kts
├── build.gradle.kts
├── app/
│   ├── build.gradle.kts
│   └── src/
│       ├── main/
│       │   ├── AndroidManifest.xml
│       │   ├── java/net/aooba/taiteboard/
│       │   │   ├── MainActivity.kt
│       │   │   ├── app/
│       │   │   ├── data/
│       │   │   ├── domain/
│       │   │   ├── feature/
│       │   │   └── ui/
│       │   └── assets/schedule.json
│       ├── test/
│       │   └── ...
│       └── androidTest/
│           └── ...
└── README.md
```

`assets/schedule.json` は手編集せず、`data/generated/schedule.json` を同期する。Androidアプリだけが独自のscheduleデータを持たないようにする。

## 11. 段階的な実装順序

### Phase 0: 共通契約の固定

- Schemaと実データの不一致を解消する。
- Web、iOS、Androidで利用するfixtureを確定する。
- `current_next_id` のフォールバック規則と鮮度の意味を固定する。
- Androidの最低SDK、application ID、feed URLの注入方法を決める。

完了条件: AndroidのUI実装前に、正常系・告知待ち・古いデータ・不正データの期待結果が定義されている。

### Phase 1: Androidデータ基盤

- Composeの起動、seed読み込み、DTO decodeを作る。
- schema version gateとdomain validationを作る。
- `DiskScheduleCache`、metadata、atomic replaceを作る。
- fake sourceを使ったRepositoryとViewModelのテストを作る。

完了条件: ネットワークなしの初回起動と再起動で、seedまたは最後の正常cacheが表示される。

### Phase 2: リモート更新と診断

- HTTPS remote sourceを追加する。
- 起動後、foreground復帰、Pull to Refreshの更新を接続する。
- fresh、stale、offline、seed、schema errorを表示する。
- DataStatusScreenを追加する。

完了条件: HTTPエラー、タイムアウト、不正JSON、未知schemaで既存の正常cacheが保護される。

### Phase 3: 主要画面と外部リンク

- 直近、一覧、カレンダー、詳細を実装する。
- 会場、チケット、出典のIntent起動を実装する。
- 画像loading、失敗、再試行、拡大表示を実装する。

完了条件: Web・iOSと同じfixtureで同じ公演情報を表示できる。

### Phase 4: 品質確認と配布

- TalkBack、フォントスケール、表示サイズ、ダークモード、edge-to-edgeを確認する。
- 低速通信、機内モード、プロセス終了後の再起動を確認する。
- unit test、instrumented test、lint、assembleをCIへ追加する。
- 内部配布またはGoogle Playの運用方式を決める。

完了条件: Android実機で、オフライン起動、外部リンク、画像失敗、アイコンマスク、画面回転を確認できる。

## 12. 検証項目

| 観点 | 確認内容 |
| --- | --- |
| 契約 | Schema v1のfixtureをdecodeし、未知versionは拒否する |
| 起動 | ネットワークなしの初回起動でseedが表示される |
| キャッシュ | remote成功後に通信を切って最後の正常データが表示される |
| 保護 | 不正なremote JSONで既存cacheが上書きされない |
| 鮮度 | `generated_at`と`cached_at`が混同されず、24時間境界が正しい |
| 時刻 | `Asia/Tokyo` と告知待ち `--:--` を保持する |
| 選択 | 同日複数公演、過去公演、`current_next_id` 欠落を処理する |
| 透明性 | HTTP状態、RTT、サイズ、検証結果、raw JSONを端末内で確認できる |
| プライバシー | 外部テレメトリ、広告ID、アカウント、閲覧履歴送信がない |
| UI | TalkBack、フォントスケール、edge-to-edge、外部Intent、画像失敗状態を確認する |
| ブランド | adaptive iconのsafe zoneとWeb/iOSとの色・形状の一致を確認する |
| 配布 | debug / releaseのfeed URLと署名設定が混ざらない |

## 13. 未決定事項

- Androidの最低対応APIレベル
- application ID、表示名、署名キー、配布方式
- Kotlin、Compose、AGP、Gradleの実行環境同期
- 本番feed URLとstaging feed URL
- HTTPクライアントと画像ローダーの依存選定
- Android側で画像キャッシュを追加する時期と容量上限
- WorkManagerによるバックグラウンド更新を将来採用するか
- Android adaptive iconのforeground/backgroundの最終Asset Catalog構成
- PromotionのSchema変更を`schema_version: 1`内で行うか、v2へ上げるか

## 参考資料

- [Jetpack Compose](https://developer.android.com/develop/ui/compose)
- [ViewModel](https://developer.android.com/topic/libraries/architecture/viewmodel)
- [Lifecycle-aware components](https://developer.android.com/topic/libraries/architecture/lifecycle)
- [Save data in app-specific storage](https://developer.android.com/training/data-storage/app-specific)
- [Intents and intent filters](https://developer.android.com/guide/components/intents-filters)
- [WorkManager](https://developer.android.com/topic/libraries/architecture/workmanager)
- [iOS ネイティブクライアント設計案](ios-native-design.md)
- [ADR-0001: Web・ネイティブ共通の schedule.json データ契約](../adr/0001-schedule-json-data-contract.md)
- [ADR-0005: 静的 JSON 配信](../adr/0005-static-json-distribution.md)
- [ADR-0008: クライアント側のキャッシュとデータ鮮度](../adr/0008-client-offline-cache-and-freshness.md)
- [ADR-0011: SwiftUI による iOS ネイティブクライアントの分離](../adr/0011-ios-native-client-architecture.md)
