# taite-board アーキテクチャ設計（Draft）

## 1. この文書の位置づけ

taite-boardのMVPから、Web実装とネイティブアプリ実装が混在する段階へ拡張するための全体設計を記録する。

このPRでは設計書を1ファイルだけ追加する。実装や既存データの変更は行わない。方針が確定した後、論点ごとにADRへ切り出す。

### 対象範囲

- URL単体またはURLリストを入力にした情報取得
- Go CLIによる外部情報の取得とrawデータ保存
- 投稿・画像・イベント情報の正規化
- グループ、メンバー、会場、チケット情報などの名寄せ
- Webとネイティブアプリが共通利用する公開データ形式
- 取得失敗、古いデータ、出典を確認できる運用

### 初期段階で対象外とするもの

- ユーザーごとの認証や権限管理
- 予約、決済、チケット販売の代理
- 常時稼働するAPIサーバーやデータベース
- すべての投稿を人手なしで正確に構造化すること
- 複数サービスをまたぐリアルタイム同期

## 2. 基本方針

データ取得、データの意味付け、表示を分離する。特に、外部サービスから取得したrawデータと、クライアントが読む公開データを同じJSONとして扱わない。

~~~text
URL単体 / URLリスト / 標準入力
        ↓
Go CLI: fetch
        ↓
raw snapshot（取得した事実）
        ↓
候補抽出・正規化・名寄せ・手動確認
        ↓
schedule.json（クライアント共通の公開データ）
        ↓
Web / iOS / Androidなどのクライアント
~~~

この分離により、Xなどの外部サービスの仕様が変わっても、影響を取得アダプターに閉じ込められる。Webとネイティブアプリは外部サービスを直接参照せず、同じ公開データを読む。

## 3. 現在のMVPとの対応

現在は次の2つの流れが並行している。

~~~text
data/input_urls.txt
        ↓
cmd/taite-fetch
        ↓
data/raw_posts.json

scripts/generate_real_schedule.js
        ↓
data/mock_schedule.json
        ↓
web/index.html / docs/index.html
~~~

Go CLIの出力は取得結果として独立しており、現時点ではUIが読むスケジュールデータと直接接続されていない。また、スケジュール生成スクリプトには手動で確認・補正した情報が含まれている。

この状態はMVPの検証として許容する。ただし次の段階では、rawデータ、候補データ、確認済みデータ、クライアント向けデータの役割を明示する。

## 4. コンポーネントの責務

| 層 | 主な責務 | 入力 | 出力 |
| --- | --- | --- | --- |
| Input | URL単体、URLファイル、標準入力の受付 | CLI引数、ファイル、stdin | URL集合 |
| Fetch | 外部サービスとの通信、取得ログ、再試行、取得時刻の記録 | URL集合 | raw snapshot |
| Extract | 投稿本文・画像・メタデータから構造化候補を抽出 | raw snapshot | 候補レコード |
| Resolve | 名称・ID・URLを正規エンティティに対応付け | 候補レコード、名寄せ辞書 | 解決済み候補 |
| Curate | 自動処理の結果を確認し、修正・採用・保留する | 解決済み候補 | 確認済みデータ |
| Publish | クライアント共通形式への変換とスキーマ検証 | 確認済みデータ | schedule.json |
| Client | 表示、検索、キャッシュ、データ鮮度の表示 | schedule.json | Web・ネイティブUI |

Fetchは外部I/Oに集中させる。イベントの意味解釈や名寄せをFetchの中に埋め込まない。

## 5. rawデータと公開データ

### 5.1 raw snapshot

raw snapshotは、外部サービスから取得した事実を後から再解析できるように保存する。UI用の表示名や補正済みの時刻はrawデータに上書きしない。

~~~json
{
  "schema_version": 1,
  "run_id": "20261030T005641+0900",
  "source": "x",
  "fetched_at": "2026-10-30T00:56:41+09:00",
  "posts": [
    {
      "source_id": "211...",
      "url": "https://x.com/...",
      "author": "...",
      "text": "...",
      "images": [],
      "fetched_at": "2026-10-30T00:56:41+09:00"
    }
  ]
}
~~~

保存単位は、将来的には取得実行単位と投稿単位の両方を追跡できる形にする。投稿ごとのsource_idを安定したキーとして扱い、同じ投稿を再取得しても出典を失わないようにする。

### 5.2 schedule.json

schedule.jsonはWebとネイティブアプリが読む公開データの契約である。クライアントはraw snapshotの内部形式に依存しない。

~~~json
{
  "schema_version": 1,
  "generated_at": "2026-10-30T01:00:00+09:00",
  "timezone": "Asia/Tokyo",
  "target_group": {
    "canonical_id": "group:ribbon-clock",
    "name": "リボンクロック"
  },
  "sessions": [
    {
      "id": "20261031-1",
      "title": "...",
      "date": "2026-10-31",
      "live": {},
      "benefit": {},
      "photo": {},
      "ticket": {},
      "source_refs": [
        {
          "type": "x_post",
          "source_id": "211...",
          "url": "https://x.com/..."
        }
      ]
    }
  ]
}
~~~

最低限、schema_version、generated_at、timezone、安定したsession id、出典URLを持たせる。仕様変更時はschema_versionを更新し、古いクライアントが安全に扱えない変更を隠さない。

## 6. 名寄せ処理

### 6.1 名寄せの目的

同じ対象が、投稿やサービスごとに異なる表記になる問題を解消する。

- グループ名の略称、旧称、記号付き表記
- メンバー名の漢字、ひらがな、ローマ字表記
- 会場名の正式名称、略称、ビル名付き表記
- チケット販売サービスの表示名、URL、サービス固有ID
- 同一イベントの表記揺れ

表示用の文字列を置き換えるだけではなく、内部では安定したcanonical_idを持つ。

### 6.2 名寄せの段階

名寄せは次の順に適用する。自動判定できない情報を無理に一つへ統合しない。

1. 文字列の安全な正規化を行う。Unicode NFKC、前後空白、連続空白、明らかな区切り記号、ASCII文字の大文字小文字などを対象にする。意味を変え得る変換は行わない。
2. canonical_idと完全一致するキーを確認する。
3. 明示的な別名辞書を確認する。
4. XのユーザーID、公式URL、会場URL、チケットURLなど、文字列より強いsource keyを確認する。
5. 一意性があり、十分な根拠がある場合だけ候補を提示する。あいまいな類似度判定は自動確定しない。
6. 人が確認した対応付けを辞書へ追加し、次回以降の判定に利用する。

初期実装では、機械学習やLLMによる自動統合を必須にしない。決定的な正規化、別名辞書、source key、手動確認を優先する。

### 6.3 名寄せのデータモデル

~~~json
{
  "entities": {
    "groups": [
      {
        "canonical_id": "group:ribbon-clock",
        "display_name": "リボンクロック",
        "aliases": ["リボクロ", "Ribbon Clock"],
        "source_keys": {
          "x_user_id": "..."
        }
      }
    ],
    "venues": [],
    "ticket_platforms": []
  },
  "resolutions": [
    {
      "entity_type": "group",
      "raw_value": "リボンクロック",
      "canonical_id": "group:ribbon-clock",
      "status": "resolved",
      "matched_by": "alias",
      "confidence": 1.0,
      "source_ref": "211..."
    }
  ]
}
~~~

名寄せ結果には、元の値、canonical_id、判定状態、判定方法、確信度、出典を残す。状態は少なくともresolved、candidate、unresolved、rejectedを持たせる。

候補や未解決の値はraw_valueを保持したまま確認待ちにする。未解決のものを空文字や推測値で上書きしない。

## 7. Webとネイティブアプリの構成

Webとネイティブアプリは同じデータ契約を利用するが、UIコードは共有しない。各クライアントは、プラットフォームの操作感とライフサイクルに合わせて実装する。

### Web

- 静的ホスティングを基本とする。
- HTTPS上のschedule.jsonを取得する。
- 取得失敗時は、最後に取得したデータまたは同梱seedデータを利用できる設計にする。
- 表示時にgenerated_atを確認し、古いデータを無表示のまま使わない。
- Xなどの外部サービスをブラウザから直接取得しない。

### ネイティブアプリ

- iOS、Androidなどのネイティブ実装をapps配下の独立クライアントとして扱う。
- アプリはschedule.jsonを読むRepository層を持つ。
- Remote RepositoryとLocal Cacheを差し替え可能にする。
- 起動時またはフォアグラウンド復帰時に更新し、成功したデータをキャッシュする。
- 通信失敗時は最後に取得したデータを表示し、取得時刻と古さを示す。
- X取得、画像解析、名寄せ、チケットサイトへの書き込みはアプリの責務にしない。

ネイティブの最初の対象プラットフォームは別途決定する。複数プラットフォームのコード共有を先に目的にせず、共有するものはschemaとデータ契約に限定する。

## 8. ディレクトリ構成案

既存MVPから段階的に移行する前提の構成を示す。最初からすべてのディレクトリを実装する必要はない。

~~~text
taite-board/
├── AGENTS.md
├── discussion.md
├── docs/
│   ├── index.html
│   └── architecture.md
├── adr/
│
├── cmd/
│   └── taite/
│       └── main.go
├── internal/
│   ├── source/
│   │   └── x/
│   ├── fetch/
│   ├── extract/
│   ├── resolve/
│   ├── publish/
│   └── validate/
├── apps/
│   ├── web/
│   │   ├── index.html
│   │   ├── app.js
│   │   └── styles.css
│   └── ios/
│       └── TaiteBoard/
├── data/
│   ├── input/
│   │   └── urls.txt
│   ├── raw/
│   │   └── runs/<run-id>/posts/<source-id>.json
│   ├── curated/
│   │   ├── entities.json
│   │   └── schedule.json
│   └── generated/
│       └── schedule.json
├── schema/
│   └── schedule.schema.json
├── testdata/
└── scripts/
~~~

役割は次のように分ける。

- data/rawは取得結果。原則として上書きしない。
- data/curatedは人が確認した名寄せ辞書とスケジュールの正とする。
- data/generatedはクライアントへ配布する生成物とする。
- schemaはWeb、ネイティブ、Goの境界となる契約とする。
- apps/webとapps/iosはクライアントごとの表示責務を持つ。
- internalは外部取得や正規化の実装を閉じ込め、クライアントから参照しない。

現状のweb/index.htmlとdocs/index.htmlの重複は、最終的にはソースを一つにする。docsは配布物として生成するか、静的ホスティングの正規ルートとして一方に統一する。

## 9. CLIのインターフェース案

~~~text
taite fetch https://x.com/Ribbon_Clock/status/...
taite fetch --input data/input/urls.txt --output data/raw/runs/<run-id>
taite fetch --stdin --output data/raw/runs/<run-id>
taite normalize --raw data/raw/runs/<run-id> --output data/generated/schedule.json
taite validate data/generated/schedule.json
~~~

URL単体とURLリストは別アプリにせず、一つのfetchコマンドで扱う。入力元が違っても、内部では同じURL集合へ正規化する。

normalizeは、rawから抽出した候補、名寄せ辞書、手動確認済みデータを入力にする。候補を作る処理と、確認済みの公開データを作る処理を分け、未確認の候補がそのまま公開されないようにする。

## 10. 設定、ログ、外部依存への防御

- Goのログはlog/slogなどの構造化ログを基本とする。
- run_id、source、URL、source_id、処理時間、成功数、失敗数、名寄せ未解決数をログに含める。
- 一部URLの取得に失敗した場合も、URL単位のエラーをraw run manifestに残す。
- 全件失敗、部分成功、成功を終了コードで区別する。
- timeout、user agent、出力先、並列数などは設定値として扱い、デフォルト値と実行時設定をログに残す。
- Xなど外部サービスの取得処理はsource adapterの境界に閉じ込める。
- 外部サービスに依存するライブ監視は、通常のユニットテストと分け、失敗しても取得済みrawデータの再利用を妨げない。

## 11. 検証方針

- rawからの本文整形、画像URL抽出、日時抽出、名寄せの各処理は固定fixtureでテストする。
- schedule.jsonはJSON Schemaとドメインルールの両方で検証する。
- session idの重複、日付と時刻の不正、出典の欠落、未解決名寄せの公開を検出する。
- Webとネイティブは同一のschedule fixtureを読み、主要な表示モデルが一致することを確認する。
- 外部サービスへアクセスするテストは、通常のテストから分離した手動または定期実行のcanaryとする。

## 12. 段階的な実装順序

### Phase 1: 契約の固定

- raw snapshotとschedule.jsonの境界を定義する。
- schedule.schema.jsonを作る。
- canonical_id、source_ref、resolution statusの形式を決める。

### Phase 2: CLIとraw保存

- 単体URL、URLファイル、標準入力をfetchで受ける。
- run単位・投稿単位でrawを保存する。
- URL単位のエラーと実行メタデータを保存する。

### Phase 3: 抽出・名寄せ・手動確認

- rawから候補を作る。
- 名寄せ辞書を使ってcanonical_idへ対応付ける。
- candidateとunresolvedを確認できる。
- 確認済みデータからschedule.jsonを生成する。

### Phase 4: Webとネイティブ

- Webをschedule.jsonの単一データ契約へ移行する。
- 同じfixtureを利用してネイティブ版を実装する。
- キャッシュ、データ鮮度、取得失敗を各クライアントで扱う。

### Phase 5: 自動化

- 定期取得と公開データ生成を自動化する。
- 外部サービスのcanary監視を追加する。
- 更新通知やPushが必要になった時点で配信基盤を検討する。

DB、APIサーバー、キュー、マイクロサービスは、静的なschedule.jsonの配信で要件を満たせなくなった時点で追加する。

## 13. 確定したアーキテクチャ選定（ADR）

本設計書の各論点は、独立したADRとして確定・運用されている。詳細な決定事項、採用理由、捨てた選択肢、影響範囲は以下の個別ADRを参照。

- [ADR-0001: Web・ネイティブ共通のschedule.jsonデータ契約とスキーマ管理](../adr/0001-schedule-json-data-contract.md)
- [ADR-0002: 生データ（Raw Snapshot）の保存構造と不変性の保証](../adr/0002-raw-snapshot-storage.md)
- [ADR-0003: canonical_id とエンティティ名寄せ辞書の設計](../adr/0003-canonical-entities-and-alias-dictionary.md)
- [ADR-0004: 自動抽出・名寄せと手動キュレーション（Overrides）の境界](../adr/0004-curation-boundary-and-overrides.md)
- [ADR-0005: 静的JSON配信の継続方針とAPIサーバー化の条件](../adr/0005-static-json-distribution.md)
- [ADR-0006: 外部取得アダプター（Source Adapter）の責務分離](../adr/0006-source-adapter-boundary.md)
- [ADR-0007: ソースコード・ビルド生成物のディレクトリ配置再編](../adr/0007-source-and-artifact-layout.md)
- [ADR-0008: クライアント側のキャッシュとデータ鮮度（Freshness）の表示](../adr/0008-client-offline-cache-and-freshness.md)
- [ADR-0009: Kubernetes GitOps デプロイと Nginx コンテナ基盤](../adr/0009-kubernetes-gitops-and-nginx-container.md)
- [ADR-0010: フロントエンドのVite・モダンTypeScript刷新とマルチステージビルド化](../adr/0010-modern-typescript-frontend-and-multistage-docker.md)
- [ADR-0011: 複数プロモーション枠分離と締切カウントダウンのアーキテクチャ](../adr/0011-multi-slot-promotions-and-deadline-countdown.md)
- [ADR-0012: ツールチェーンの最新化とTailwind CSS v4移行](../adr/0012-toolchain-modernization-vite8-ts7-tailwind4.md)

## 14. 未決定事項

- 最初のネイティブ対象をiOS、Android、または両方のどれにするか。
- curated/schedule.jsonを人が直接編集するか、専用の確認UIを先に作るか。
- 公開ファイル名をschedule.jsonに固定するか、feed.jsonなどにするか。
- 名寄せ対象をグループ・会場・チケットサービスからどこまで広げるか。
- GitHub Pagesなどの静的ホスティングを正式な公開先にするか。

これらは実装開始前にすべて確定させる必要はない。ただし、公開データのschema_version、canonical_id、source_refは早期に固定する。
