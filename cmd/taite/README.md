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
生成されたスケジュールJSONが [schema/schedule.schema.json](../../schema/schedule.schema.json) およびドメインルール（ADR-0001準拠）に適合しているかを厳格に検証します。

```bash
# デフォルトパス (data/generated/schedule.json) を検証
taite validate

# 任意のファイルを指定して検証
taite validate path/to/schedule.json
```

#### 主な検証項目:
- 必須プロパティ（`schema_version`, `generated_at`, `timezone`, `target_group.name`, `sessions`）の存在。
- セッションIDの一意性（重複検出）。
- 日付フォーマット（`YYYY-MM-DD`）。
- 時刻フォーマット（`HH:MM` または `--:--`）。
- 会場オブジェクト（`venue.name`）およびチケットオブジェクトの存在。

---

### 3. `taite build`
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
  - JSON Schema準拠のドメイン検証ロジック。
