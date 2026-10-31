---
name: taite-board-json-ingest
description: >-
  Converts URLs and local image paths supplied in a terminal request into a
  traceable taite-board candidate JSON intake file. Use when a user provides
  X or web links, flyer images, or timetable images and asks to extract
  schedule data while preserving evidence, unresolved fields, and review state.
---

# taite-board JSON ingest

URL やフライヤー画像を taite-board の公開データへ反映する前に、取得元とハッシュを残した候補 JSON に変換するための skill です。端末ベースで完結させ、ブラウザ操作や候補データの自動公開は行いません。

## 使う場面

- ユーザーが X・Web の URL を渡して公演情報を JSON 化したい場合
- ユーザーがフライヤーやタイムテーブル画像を渡して JSON 化したい場合
- 既存の候補 JSON を再確認し、出典と未確定項目を整理したい場合

## 手順

1. まず入力を `stage_sources.py` で intake manifest にする。画像は可能ならリポジトリ内の `data/raw/manual/<run-id>/images/` にステージし、URL は変更せず保存する。

   ```bash
   python3 .agents/skills/taite-board-json-ingest/scripts/stage_sources.py \
     --repo-root . \
     --stage-dir "data/raw/manual/${run_id}/images" \
     --output "data/candidate/manual/${run_id}.json" \
     --url "https://x.com/example/status/123" \
     --image "/path/to/flyer.png"
   ```

   URL と画像は複数指定できる。入力ファイルを使う場合は `--input` を使い、1 行 1 入力、必要なら `url:` または `image:` を先頭につける。

2. X の投稿本文が必要な URL は、既存の CLI で raw snapshot を取得する。ブラウザで表示した内容を根拠にせず、取得失敗時は URL のみを出典として未確認にする。

   ```bash
   go run ./cmd/taite fetch --out data/raw "https://x.com/example/status/123"
   ```

3. ローカル画像は `view_image` で目視し、本文・画像に明記された事実だけを候補へ転記する。画像 URL を端末から一時取得する場合も、`/private/tmp` などへ保存してからハッシュを取り、取得元 URL を残す。
   - **開演時刻と出演枠の混同禁止**: 対バンの「開場/開演時刻（OPEN/START）」はイベント全体の開始時刻であり、グループ個別の出演時刻ではありません。公式X告知本文またはタイムテーブル画像に出演枠が明記されていない限り、開演時刻を出番に流用してはならず、必ず `--:--`（タイテ告知待ち）として扱います。
   - **フライヤーとタイテ表の全画像回収**: X投稿に複数画像がある場合、フライヤーだけでなくタイムテーブル画像も漏れなく抽出し、`type: "timetable"` を割り当てます。
   - **公式X以外のURLのソース混入禁止**: `source_tweet_url` にチケット販売サイト（eplus, TicketDive, LivePocket等）を設定してはなりません。チケットURLは `ticket.url` に格納し、ソースには必ず公式X告知投稿URLを設定します。
   - **ローカル相対画像パスの禁止**: `/images/...` や `images/...` 等の架空・暫定ローカルパスを成果物に設定してはなりません。外部配信可能な完全修飾URL（`https://pbs.twimg.com/media/...`）を使用します。

4. 特典情報・チケット情報の機械的抽出と候補 JSON の作成
   - 入場特典の抽出およびチケットプラットフォームのサニタイズには、必ず `extract_admission_bonus.py` を使用します。手動での文字列切り出しによる「券10枚」の脱落やプラットフォーム名への完売情報混入を防止します。

   ```bash
   python3 .agents/skills/taite-board-json-ingest/scripts/extract_admission_bonus.py \
     --file "data/raw/runs/${run_id}/posts/${post_id}.json" \
     --platform "<platform-string>" \
     --json
   ```

   - `references/candidate-contract.md` に従って `sessions` を埋める。各セッションには必ず `source_refs` を付け、日付・会場・時刻・特典会など、根拠が見つからない値は推測しない。時刻は `--:--`、値は `null`、状態は `unconfirmed` または `needs_review` を使う。

5. 完成した候補を検証する。

   ```bash
   python3 .agents/skills/taite-board-json-ingest/scripts/validate_candidate.py \
     --file "data/candidate/manual/${run_id}.json" \
     --repo-root . \
     --require-session
   ```

6. 検証結果と未解決項目をユーザーへ報告する。候補 JSON を `data/generated/schedule.json` にコピーしたり、候補だけで本番公開したりしない。公開には [docs/manual-ingest-release.md](../../../docs/manual-ingest-release.md) のレビュー手順を使う。

## Script と Skill の責務分割 (Separation of Responsibilities)

- **Script の責務 (機械的・決定論的処理)**:
  - `extract_admission_bonus.py`:
    - 投稿本文から入場特典（お目当て3択など）を正規化抽出。選択肢③内部のスラッシュ（`券5枚⇒集合写真/券10枚⇒私物サイン`）による文字列分断・切り捨てを機械的に防止する。
    - チケット購入ボタンの描画崩れ防止のため、`ticket.platform` に混入した「完売」「当日券あり」等のステータス文言を自動除去し、純粋なプラットフォーム名（`イープラス` 等）と備考に分離する。
  - `validate_candidate.py`:
    - 特典の選択肢欠落（「券5枚」があるのに「10枚」が脱落）や `ticket.platform` の長文混入を機械的ルールとして自動検知・警告する。
- **Skill の責務 (エージェント判断・検証プロトコル)**:
  - 複数画像ポストにおける告知画像とタイテ表の種別判定、Apple Vision OCR の実行と突合。
  - 個別特殊イベント（ビンゴ大会、動員招待特典等）における例外的な特典パターンの判定と確認。
  - `extract_admission_bonus.py` を呼び出して抽出した結果を候補 JSON に反映し、推測による直書きを避ける。

## 完了条件

- 入力 URL、画像の相対パス、SHA-256、取得時刻が候補 JSON に残っている
- 候補の各事実が少なくとも 1 つの `source_refs` に結び付いている
- 未確認の時刻や名寄せを推測で補っていない（開演時刻を出番に流用していない）
- チケットURLが `source_tweet_url` に設定されておらず、公式X告知URLが設定されている
- 架空のローカル画像相対パスではなく、完全修飾HTTP(S) URLが設定されている
- `validate_candidate.py --require-session` が成功している
- 公開用 JSON の更新は人間レビューと通常のリリース検証が終わるまで行っていない

候補 JSON の項目と、公開用 `schema/schedule.schema.json` への対応は [references/candidate-contract.md](references/candidate-contract.md) を参照する。
