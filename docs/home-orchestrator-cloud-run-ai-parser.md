# 自宅オーケストレーターと Cloud Run AI パーサー設計案

## 1. ステータス

- 状態: Proposed
- 対象: X 投稿・告知画像からのイベント情報抽出、名寄せ、公開データ生成
- 対象外: Web UI、iOS / Android の画面設計、Cloud Run への本番デプロイ実装

## 2. 結論

自宅サーバーを低コストな判定・キャッシュ・キュレーションの実行基盤とし、Cloud Run は AI による画像・文章パースが必要な場合だけ呼び出す。

自宅サーバー側で次の処理を先に実行する。

1. X 投稿と画像の raw snapshot を保存する。
2. 投稿 ID、本文ハッシュ、画像ハッシュで重複を判定する。
3. 正規化、エイリアス辞書、source key による決定論的な名寄せを行う。
4. 正規表現などの安価な抽出で必要な情報が揃うか確認する。
5. 既存の candidate と parser / model / prompt のバージョンを照合する。
6. 新規情報または未解決フィールドがある場合だけ Cloud Run を呼び出す。

Cloud Run は解析結果を candidate として返すだけにし、`schedule.json` の直接更新、`canonical_id` の無根拠な確定、手動 override の上書きは行わない。

## 3. 背景と現行実装との差分

現行リポジトリには、次の基盤がある。

- `taite fetch` と `internal/source/x` による X 投稿取得
- `data/raw/runs/<run-id>/` への不変 snapshot 保存
- `taite validate` による公開 JSON の検証
- `data/curated/entities.json` による正準エンティティ・エイリアス辞書
- `data/curated/overrides.json` による手動補正
- `data/generated/schedule.json` と `apps/web/public/schedule.json` による静的配信

一方、現時点では `internal/extract/`、`internal/resolve/`、`internal/publish/` と、それらを呼び出す `taite extract`、`taite resolve`、`taite publish` は存在しない。`canonical_id` が生成データに入っていることは、実行時の名寄せロジックが完成していることを意味しない。

したがって本設計は、既存の `raw → candidate → curated → generated` というデータ境界を実装へ接続するための提案である。

## 4. 目的

- AIパースの呼び出し回数と画像送信量を最小化する。
- 自宅サーバーだけで再現可能な判定を完結させる。
- AIの出力を公開データへ直接混入させない。
- raw snapshot から同じ解析を再実行できるようにする。
- Cloud Run の認証、IP制限、リトライ、タイムアウトを明示的に管理する。
- Web、iOS、Android が同じ公開データ契約を読む状態を維持する。

## 5. 非目標

- Cloud Run から X を直接巡回し、取得結果を公開すること。
- LLMに `canonical_id` を推測させ、そのまま確定すること。
- Cloud Run の解析結果だけを根拠に中止・延期・時刻変更を自動公開すること。
- 自宅サーバーを外部へ公開して、クライアントから直接操作させること。

## 6. コンポーネント責務

| コンポーネント | 実行場所 | 責務 |
| --- | --- | --- |
| `taite fetch` | 自宅サーバー | X投稿取得、media URL正規化、raw snapshot保存 |
| Home Orchestrator | 自宅サーバー | 重複判定、キャッシュ、安価な抽出、AI呼び出し判定、ジョブ管理 |
| Deterministic Resolver | 自宅サーバー | 正規化、alias、source key、確定済み辞書による名寄せ |
| Candidate Store | 自宅サーバー | AI・ルール抽出結果、根拠、ハッシュ、バージョン、状態の保存 |
| Cloud Run AI Parser | Cloud Run | 本文・画像からのイベント候補抽出、根拠付き構造化結果の返却 |
| Human Curation | 自宅サーバーまたはGit | `overrides.json` の確認・更新 |
| Publisher | 自宅サーバー | candidate、resolver、overrideの統合、schema検証、公開JSON生成 |
| Web / iOS / Android | 公開環境・端末 | `schedule.json` の読み取りとキャッシュ |

Cloud Run の処理はステートレスにし、長期的な正本データは自宅サーバー側に保存する。

## 7. 処理フロー

### 7.1 取得と raw 保存

1. `data/input/` の対象 URL または監視対象から X 投稿を取得する。
2. 投稿本文、投稿 ID、投稿時刻、取得時刻、media URL、取得結果を raw snapshot に保存する。
3. 外部 URL は正規化した値だけでなく、取得時点の元 URL も保持する。
4. raw snapshot は上書きせず、`run_id` ごとに保存する。

Cloud Run から X を直接取得しない。これにより、取得の失敗と解析の失敗を分離でき、同じ raw snapshot を使って将来のパーサーを再評価できる。

### 7.2 自宅サーバー側の事前判定

raw snapshot ごとに、Cloud Run を呼ぶ前に次を評価する。

| 判定 | 結果 |
| --- | --- |
| 同じ投稿 ID、本文ハッシュ、画像ハッシュの成功済み candidate がある | 既存結果を再利用し、Cloud Runを呼ばない |
| 本文の正規表現抽出だけで必須フィールドが揃う | ルール結果を candidate にしてCloud Runを呼ばない |
| グループや会場は既知だが、新しいタイテ画像がある | Cloud Runを呼ぶ |
| 既存画像に未抽出の時刻・特典会情報がある | Cloud Runを呼ぶ |
| parser / model / prompt / schema のバージョンが変わった | 必要な範囲を再解析する |
| 一時的な取得失敗だけが記録されている | バックオフ後に再試行する |
| 同じ入力で恒久的な解析エラーが記録されている | 自動再試行せず、人手確認キューへ送る |

「名寄せ済み」はAI不要の十分条件ではない。会場が解決済みでも、画像内の出演時刻が未抽出ならAIパースが必要になる。そのため、`needs_ai` はエンティティの解決状態だけでなく、必須フィールドと根拠の充足状態から判定する。

### 7.3 Cloud Run AI パース

Cloud Runには、解析対象の raw snapshot または必要な media だけを送る。AIが不要な場合はCloud RunへのHTTPリクエスト自体を発生させない。

低頻度の単発解析は Cloud Run Service のスケール・トゥ・ゼロを基本とし、複数投稿の一括再解析や時間のかかる処理は Cloud Run Jobs またはキュー経由の非同期実行を候補とする。

Cloud Runは次の処理だけを担当する。

- 本文から日付、会場、出演、チケット、特典会の候補を抽出する。
- 告知画像からタイムテーブルの時刻・ラベル・注意書きの候補を抽出する。
- 各フィールドに根拠、確信度、抽出元を付与する。
- candidate schema に沿った構造化レスポンスを返す。

Cloud Runは次の処理を担当しない。

- `schedule.json` の公開
- 人手 override の削除や上書き
- 辞書にないエンティティの自動確定
- 入力に存在しない時刻・ルールの補完

### 7.4 candidate 以降

1. Cloud Runのレスポンスを `candidate` として保存する。
2. 自宅サーバーの deterministic resolver を再実行する。
3. `entities.json` の alias、source key、手動対応付けを適用する。
4. `unresolved`、`candidate`、`rejected` を人手確認キューへ送る。
5. `overrides.json` を適用する。
6. schema、重複、時刻の整合性、出典の有無を検証する。
7. 検証済みのものだけを `data/generated/schedule.json` として公開する。

## 8. キャッシュと冪等性

### 8.1 入力フィンガープリント

解析の単位は、少なくとも次の値から構成する。

```text
raw_fingerprint = SHA-256(
  source
  + source_id
  + normalized_text
  + ordered_media_sha256_list
)
```

Cloud Run呼び出しの冪等性キーは、入力だけでなく解析仕様も含める。

```text
parse_cache_key = SHA-256(
  raw_fingerprint
  + candidate_schema_version
  + parser_version
  + model_id
  + prompt_version
)
```

同じ `parse_cache_key` で成功済みの candidate があれば、Cloud Runは呼ばずに再利用する。パーサーやプロンプトを更新した場合はキーが変わるため、必要な範囲だけ再解析できる。

### 8.2 状態

candidate は少なくとも次の状態を持つ。

- `not_requested`: AI解析対象と未判定
- `skipped_deterministic`: ルール処理だけで完了
- `skipped_cache`: 既存candidateを再利用
- `queued`: Cloud Run解析待ち
- `running`: Cloud Run解析中
- `completed`: 構造化candidateを取得済み
- `needs_review`: 根拠不足または名寄せ未解決
- `failed_retryable`: 一時障害で再試行可能
- `failed_permanent`: 入力不備や恒久エラーで人手確認が必要

## 9. API 契約案

### 9.1 Home Orchestrator から Cloud Run へのリクエスト

画像を毎回インラインで再送するのではなく、AI解析が必要になったときだけ短時間有効な入力参照を発行する。小さな本文はリクエストに含め、画像とraw JSONはオブジェクト参照に分ける。

```json
{
  "request_id": "parse-20261030-000001",
  "idempotency_key": "sha256:...",
  "source": "x",
  "source_id": "2112891667640656194",
  "raw_fingerprint": "sha256:...",
  "candidate_schema_version": 1,
  "parser_version": "2026-10-30.1",
  "model_id": "provider/model",
  "prompt_version": "event-v1",
  "text": "明日のタイテです。",
  "media": [
    {
      "media_id": "...",
      "content_type": "image/jpeg",
      "sha256": "sha256:...",
      "input_url": "short-lived-signed-url"
    }
  ]
}
```

### 9.2 Cloud Run からのレスポンス

```json
{
  "request_id": "parse-20261030-000001",
  "status": "completed",
  "parser_version": "2026-10-30.1",
  "model_id": "provider/model",
  "prompt_version": "event-v1",
  "candidate": {
    "date": "2026-11-07",
    "venue_raw": "渋谷RING",
    "live_start_raw": "18:30",
    "live_end_raw": "19:00"
  },
  "evidence": [
    {
      "field": "live_start_raw",
      "source": "image",
      "media_id": "...",
      "quote": "18:30",
      "confidence": 0.98
    }
  ],
  "usage": {
    "input_images": 1,
    "input_tokens": 0,
    "output_tokens": 0
  }
}
```

`quote` や画像上の座標などの根拠がない候補は、`needs_review` として扱う。AIレスポンスのフィールドをそのまま公開スキーマへ変換せず、自宅サーバー側で型・値・根拠を検証する。

## 10. 認証・IP制限・ネットワーク

### 10.1 基本方針

- Cloud Runのエンドポイントは匿名公開しない。
- 自宅サーバーからのリクエストには、IP制限に加えてリクエスト認証を付ける。
- `request_id` と `idempotency_key` をログに残し、再送と重複処理を追跡できるようにする。
- Cloud Runから自宅サーバーへ直接接続させない。入力参照と結果返却の方向を限定する。

### 10.2 IP制限

IP制限を有効な制御にするには、自宅サーバーの外向きグローバルIPが固定されている必要がある。IPによる許可リストは、Cloud Runの前段に置くロードバランサー、Cloud Armor、またはアクセスゲートウェイで適用する。

自宅回線のIPが変動する場合は、IP制限だけに依存しない。OIDC、署名付きリクエスト、mTLS、VPNなどを併用し、IPは追加の防御層として扱う。

長期有効なサービスアカウント鍵を自宅サーバーへ配置する設計は採用しない。短期トークン、Workload Identity Federation、またはSecret Managerからのローテーション可能な共有秘密を候補とする。

### 10.3 入力参照

AI解析が必要な場合だけ、raw snapshotと画像をCloud Runから取得できる短時間有効な参照を発行する。参照は解析終了後に無効化し、Cloud Run側に長期保存しない。

## 11. 失敗・再試行・コスト制御

### 11.1 再試行

- 429、5xx、接続タイムアウトは指数バックオフ付きで再試行する。
- JSON schema違反や根拠不足は、同じ入力を無制限に再試行しない。
- `raw_fingerprint` と `parse_cache_key` で二重解析を防ぐ。
- 一定回数の失敗は `failed_permanent` として人手確認へ送る。
- raw snapshotを残すため、Cloud Run障害時も後日再実行できる。

### 11.2 コスト制御

- Cloud Run呼び出し前に必ず自宅サーバー側のキャッシュ判定を行う。
- 既知の投稿、既知の画像、同一parser versionの成功結果は再利用する。
- 本文だけで完結する投稿にVision AIを使わない。
- 画像の解像度・枚数を必要最小限にする。
- 1リクエストあたりの画像数、処理時間、再試行回数、日次AI予算に上限を設ける。
- モデルやプロンプトを変更したときは全件再解析ではなく、対象条件を限定する。

## 12. 観測性と監査

自宅サーバー側では、少なくとも次を記録する。

- `source_id`、`raw_fingerprint`、`parse_cache_key`
- AI判定結果: `skipped_cache`、`skipped_deterministic`、`requested` など
- Cloud Run呼び出し回数、成功率、レイテンシ、再試行回数
- parser / model / prompt / schema のバージョン
- 名寄せ結果と `matched_by`
- 人手overrideの適用結果

Cloud Run側では、本文や画像そのものを通常ログへ出さず、`request_id`、処理時間、入力数、モデル使用量、結果状態を記録する。rawとcandidateの保存先は自宅側の監査可能なストレージを正本とする。

## 13. 導入順序

### Phase 1: 決定論的な基盤

- `internal/resolve/` を実装する。
- NFKC、空白、大小文字、明示 alias、source key の正規化とテストを追加する。
- `resolved`、`candidate`、`unresolved`、`rejected` を表現できるcandidateモデルを定義する。
- `data/raw` のfixtureから再現可能なテストを作る。

### Phase 2: 自宅サーバー側の判定とキャッシュ

- `raw_fingerprint` と `parse_cache_key` を導入する。
- `needs_ai` 判定を実装する。
- Cloud Runなしで、ルール抽出・名寄せ・cache hitの早期returnを検証する。

### Phase 3: Cloud Run parser

- candidate schemaを固定する。
- 認証済みの最小APIを作る。
- 画像の短期参照、タイムアウト、冪等性、再試行を実装する。
- AI応答の根拠とバージョンを保存する。

### Phase 4: 公開生成と運用

- `taite extract`、`taite resolve`、`taite publish` の責務をCLIへ接続する。
- 手動確認キューと `overrides.json` の更新手順を定義する。
- 日次のAI予算、失敗通知、未解決件数、古いcandidateを監視する。

## 14. 完了条件

- 同一raw snapshotの再実行で同じ名寄せ結果になる。
- 解析済み入力に対してCloud Runを呼ばない。
- 新規・変更画像だけがCloud Runへ送られる。
- Cloud Runが停止しても既存の公開JSONとraw snapshotが失われない。
- AIの候補が人手確認とschema検証を通らない限り公開されない。
- 解析結果から、元投稿、画像、モデル、prompt、parser versionを追跡できる。
- 自宅サーバーのIP制限が変動しても、認証層が単独で安全性を維持できる。

## 15. 未決定事項

- Cloud Run ServiceとCloud Run Jobsの採用境界
- raw snapshotと画像の一時保管先をローカル、S3互換ストレージ、GCSのどれにするか
- AIモデルとVision解析のプロバイダー
- 自宅サーバーからCloud Runへ接続する認証方式
- 人手確認キューをGit上のJSONで始めるか、専用の小さな管理APIを作るか
- Cloud Runを呼び出す日次予算と、予算到達時の停止動作

## 参考資料

- [Cloud Run overview](https://cloud.google.com/run/docs/overview)
- [Cloud Run authentication overview](https://cloud.google.com/run/docs/authenticating/overview)
- [Cloud Run ingress settings](https://cloud.google.com/run/docs/securing/ingress)
- [Cloud Run jobs](https://cloud.google.com/run/docs/create-jobs)
- [Cloud Armor security policy concepts](https://cloud.google.com/armor/docs/security-policy-concepts)
- [ADR-0002: raw snapshot storage](../adr/0002-raw-snapshot-storage.md)
- [ADR-0003: canonical entities and alias dictionary](../adr/0003-canonical-entities-and-alias-dictionary.md)
- [ADR-0004: extraction, resolution, and manual curation boundary](../adr/0004-curation-boundary-and-overrides.md)
