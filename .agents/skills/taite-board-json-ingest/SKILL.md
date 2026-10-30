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

4. `references/candidate-contract.md` に従って `sessions` を埋める。各セッションには必ず `source_refs` を付け、日付・会場・時刻・特典会など、根拠が見つからない値は推測しない。時刻は `--:--`、値は `null`、状態は `unconfirmed` または `needs_review` を使う。

5. 完成した候補を検証する。

   ```bash
   python3 .agents/skills/taite-board-json-ingest/scripts/validate_candidate.py \
     --file "data/candidate/manual/${run_id}.json" \
     --repo-root . \
     --require-session
   ```

6. 検証結果と未解決項目をユーザーへ報告する。候補 JSON を `data/generated/schedule.json` にコピーしたり、候補だけで本番公開したりしない。公開には [docs/manual-ingest-release.md](../../../docs/manual-ingest-release.md) のレビュー手順を使う。

## 完了条件

- 入力 URL、画像の相対パス、SHA-256、取得時刻が候補 JSON に残っている
- 候補の各事実が少なくとも 1 つの `source_refs` に結び付いている
- 未確認の時刻や名寄せを推測で補っていない
- `validate_candidate.py --require-session` が成功している
- 公開用 JSON の更新は人間レビューと通常のリリース検証が終わるまで行っていない

候補 JSON の項目と、公開用 `schema/schedule.schema.json` への対応は [references/candidate-contract.md](references/candidate-contract.md) を参照する。
