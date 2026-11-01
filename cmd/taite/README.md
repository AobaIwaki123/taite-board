# taite - Idol Timetable Data Pipeline CLI

`taite` は、アイドルの公式X（旧Twitter）投稿や告知からライブ情報を取得・検証・ビルドするための統合コマンドラインツールです。

---

## インストール

Go の標準的なインストールプロトコルに準拠しています。

```bash
# GOBIN ($GOPATH/bin) へのインストール
make install

# または直接実行
go install ./cmd/taite

# インストールスクリプト経由
./scripts/install.sh
```

インストール後、環境変数 `PATH` に `$GOBIN` が含まれていれば、任意のディレクトリから `taite` コマンドとして実行できます。

### ローカルバイナリのビルド (リポジトリ直下の bin/taite)
```bash
make build
# または
go build -o bin/taite ./cmd/taite
```

---

## コマンド一覧

### 1. `taite fetch`
外部URL（Xの投稿等）から投稿本文および添付画像URLを取得し、不変の生スナップショット（Raw Snapshot）として `data/raw/runs/<run-id>/` に保存します（ADR-0002準拠）。

```bash
# 単一または複数のURLを指定して取得
taite fetch "https://x.com/Ribbon_Clock/status/2112891667640656194"

# URLリストファイルを指定して一括取得 (行ごとにURLを記載)
taite fetch -input data/input/input_urls.txt

# オプション指定
taite fetch -input data/input/input_urls.txt -out data/raw -timeout 15 -workers 5
```

#### 主なオプション:
- `-input <file>`: URL一覧テキストファイルのパス（`#` から始まるコメント行および空行は無視）。
- `-out <dir>`: 生スナップショットの保存先ディレクトリ（デフォルト: `data/raw`）。
- `-timeout <sec>`: HTTPリクエストのタイムアウト秒数（デフォルト: `10`）。
- `-workers <count>`: 並列取得ワーカー数（デフォルト: `5`）。

#### 出力構造:
```text
data/raw/runs/YYYYMMDDTHHMMSS+0900/
├── manifest.json       # 実行メタデータ (成功数、失敗URL、エラー理由)
└── posts/
    ├── <status-id-1>.json
    └── <status-id-2>.json
```

---

### 2. `taite validate`
生成されたスケジュールJSONが [schema/schedule.schema.json](../../schema/schedule.schema.json) およびドメインルール（ADR-0001、ADR-0014準拠）に適合しているかを厳格に検証します。

```bash
# デフォルトパス (data/generated/schedule.json) を検証
taite validate

# 任意のファイルを指定して検証
taite validate --file path/to/schedule.json
```

#### 主な検証項目:
- 必須プロパティ（`schema_version`, `generated_at`, `timezone`, `target_group.name`, `sessions`）の存在。
- セッションIDの一意性（重複検出）。
- 日付フォーマット（`YYYY-MM-DD`）。
- 時刻フォーマット（`HH:MM` または `--:--`）。
- 会場オブジェクト（`venue.name`）およびチケットオブジェクトの存在。
- **時系列順序検査 (ADR-0014)**: `sessions[]` が `date`（昇順）→ `live.start_time`（昇順）→ `id`（昇順）に整列されているかを検証。
- **同日複数公演インターバル検査 (ADR-0014)**: 同日に2公演以上ある場合、直前セッションの特典会終了時刻と次セッションの出番開始時刻の重複をエラー検出（30分未満の場合は警告）。

---

### 3. `taite sync`
`data/generated/schedule.json` を `apps/web/public/schedule.json` および `apps/web/public/mock_schedule.json` へ自動複製し、SHA-256 チェックサムの一致を保証します。

```bash
taite sync
# または
make sync
```

---

### 4. `taite sort`
`data/generated/schedule.json` 内の `sessions[]` を日付（昇順）→ 出番開始時刻（昇順）→ ID（昇順）で機械的にソートし、`generated_at` を現在時刻に更新して Web 公開ディレクトリへ自動同期します（ADR-0014準拠）。

```bash
taite sort
# または
taite sort -file path/to/schedule.json
```

---

### 5. `taite add-session`
新規公演セッションを引数から安全に追加します。同一日の既存セッション数からセッションID（`YYYYMMDD-1` 等）および曜日（`day_of_week`）を自動算定し、時系列ソートと Web 公開ディレクトリへの自動同期を実行します（ADR-0014準拠）。

```bash
taite add-session \
  -date "2026-11-15" \
  -title "IDOL LIVE CIRCUIT 2026" \
  -venue "渋谷近未来会館" \
  -live-start "14:20" \
  -live-end "14:40" \
  -benefit-type "parallel" \
  -benefit-start "14:50" \
  -benefit-end "15:50" \
  -booth "Bブース" \
  -ticket-platform "TIGET" \
  -ticket-url "https://tiget.net/events/example"
```

#### 主なオプション:
- `-date <YYYY-MM-DD>`: 公演日付（必須）。
- `-title <string>`: ライブ・イベント名称（必須）。
- `-venue <string>`: 会場名（必須）。
- `-live-start <HH:MM>`: ライブ出演開始時刻（必須）。
- `-live-end <HH:MM>`: ライブ出演終了時刻（省略可）。
- `-benefit-type <type>`: 特典会種別（`parallel`: 並行, `after`: 終演後, `none`: なし, `before`: 事前。デフォルト: `after`）。
- `-benefit-start <HH:MM>`: 特典会開始時刻。
- `-benefit-end <HH:MM>`: 特典会終了時刻。
- `-booth <string>`: 特典会ブース番号・場所名。
- `-ticket-platform <string>`: チケット販売プラットフォーム（デフォルト: `TIGET`）。
- `-ticket-url <string>`: チケット販売・予約ページURL。
- `-file <string>`: スケジュールJSONファイルのパス（デフォルト: `data/generated/schedule.json`）。
- `-no-sync`: Web 公開ファイルへの自動同期をスキップする場合に指定。

---

### 6. `taite update-session`
公演セッションの撮影可能ルール、特記事項、公式告知ポスト URL を更新し、`generated_at` の日時更新と Web 配下への自動同期を実行します。

```bash
taite update-session -session 20261101-1 \
  -rule "5曲目『75秒の沈黙』撮影可能" \
  -tweet-url "https://x.com/Ribbon_Clock/status/2116455139959840824" \
  -notes "私服ライブ出演"
```

#### 主なオプション:
- `-session <id>`: 更新対象のセッションID（必須）。
- `-rule <text>`: 正規化済みの撮影可否ルール文言。
- `-raw <text>`: 告知元の生テキストスニペット。
- `-tweet-url <url>`: 出典告知ポストのURL。
- `-official`: 指定した `-tweet-url` を公演の公式告知リンク（`source_tweet_url`）にも反映（デフォルト: `true`）。
- `-notes <text>`: 特記事項に追記・結合する文言。
- `-no-sync`: Web 公開ファイルへの自動同期をスキップする場合に指定。

---

### 7. `taite build`
フロントエンド（`apps/web/`）の TypeScript バンドルビルド（`scripts/build.sh` 経由で Vite ビルド）を実行します。

```bash
taite build
# または
make web-build
```

---

## 内部パッケージ構成 (`internal/`)

外部I/Oや抽出・検証ロジックは、CLIの `main.go` ではなく `internal/` 配下の独立したパッケージに隔離されています（ADR-0006準拠）。

- **`internal/source/x/`**:
  - X公式のoEmbed APIおよびSyndication APIとの通信アダプター。
  - レート制限、タイムアウト、テキスト整形、画像URL抽出を担当。
- **`internal/raw/`**:
  - `Post` および `RunManifest` 構造体の定義。
- **`internal/fetch/`**:
  - 並列フェッチオーケストレータおよびローカルファイルへの不変保存処理。
- **`internal/validate/`**:
  - JSON Schema準拠のドメイン検証、時系列順序検査、同日公演インターバル検査ロジック。
- **`internal/schedule/`**:
  - スケジュール JSON の SHA-256 同期保証（`sync.go`）、時系列決定論的ソート（`sort.go`）、安全なセッション追加（`add.go`）、セッション情報の決定論的更新（`update.go`）。
  - スケジュール JSON の SHA-256 同期保証およびセッションの決定論的更新ロジック。
