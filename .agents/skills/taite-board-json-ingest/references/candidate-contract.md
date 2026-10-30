# Candidate intake contract

`taite-board-json-ingest` の出力は、公開用 `schema/schedule.schema.json` そのものではありません。外部ソースの取得証跡と、人間レビュー前の候補を保持する中間契約です。

## ルート

| 項目 | 必須 | 意味 |
| --- | --- | --- |
| `contract` | 必須 | `taite-board.candidate-intake/v1` 固定 |
| `run_id` | 必須 | 取得・変換単位の安定した識別子 |
| `created_at` | 必須 | intake manifest の作成時刻。ISO 8601 UTC |
| `status` | 必須 | `needs_review` または `ready`。`published` は禁止 |
| `inputs` | 必須 | URL と画像の入力一覧 |
| `sessions` | 必須 | 候補セッション。初期 manifest では空配列でもよい |

## 入力の扱い

URL は入力された文字列を変更せず `inputs[].url` に保存します。X 投稿なら `source_type` を `x_post`、その他は `web_page` とします。

ローカル画像は次の属性を保存します。

- `path`: `data/raw/manual/<run-id>/images/` 配下へステージした相対パス
- `original_name`: 元ファイル名
- `sha256`: ステージ前後で一致する画像の SHA-256
- `media_type` と `size_bytes`

画像バイナリを PR に含めるかは、著作権・サイズ・リポジトリ運用に応じて判断します。含めない場合でも、レビュー中はローカル raw と hash を保持し、候補には URL・ファイル名・hash を残します。

## セッション候補

各 `sessions[]` は次の値を持ちます。

```json
{
  "id": "candidate-20261107-1",
  "status": "needs_review",
  "title": "告知から確認できた公演名",
  "date": "2026-11-07",
  "venue": { "name": "確認できた会場名", "canonical_id": null },
  "live": { "start_time": "--:--", "end_time": null, "stage": null },
  "benefit": { "start_time": "--:--", "end_time": null, "type": null },
  "ticket": { "platform": null, "url": null, "prices": [] },
  "images": [],
  "source_refs": [
    { "input_id": "input-01", "type": "x_post", "url": "https://x.com/example/status/123" }
  ],
  "evidence": {
    "date": "画像の上部に記載",
    "venue": "本文の会場記載"
  }
}
```

`source_refs` は空にしません。本文や画像で確認できない項目は、時刻なら `--:--`、その他の値なら `null` とし、`evidence` に「未確認」の理由を残します。LLM や OCR が候補を出しても、根拠のない補完値を `scheduled` として扱いません。

### アンチパターンと品質防止ルール

1. **開演時刻の出番流用禁止**:
   - イベント全体の「OPEN / START」時刻（例: 開演10:20）を、出演グループの出番（`live.start_time`）として勝手に割り当ててはなりません。タイムテーブル画像または出演枠告知がない場合は必ず `--:--`（タイテ告知待ち）とします。
2. **チケットURLのソース誤設定禁止**:
   - イープラス、TicketDive、LivePocket等の販売ページURLを `source_tweet_url` に設定してはなりません。ソースは必ず公式X投稿URL（`https://x.com/...`）とします。
3. **ローカル相対画像パスの混入禁止**:
   - `/images/...` や `images/...` のようなモック残骸や架空の相対パスを設定してはなりません。外部アクセス可能な完全修飾URL（`https://pbs.twimg.com/...`）を使用します。
4. **タイテ画像の抽出漏れ禁止**:
   - 公式投稿にフライヤーとタイムテーブルの両方が添付されている場合、フライヤー1枚だけで満足せず、タイムテーブル画像も漏れなく `type: "timetable"` として登録します。

## 公開用 JSON への対応

レビュー後に候補を公開データへ反映する場合は、次の項目へ対応づけます。

| Candidate | 公開用 `schema/schedule.schema.json` |
| --- | --- |
| `title`, `date`, `venue` | `sessions[].title`, `date`, `venue` |
| `live` | `sessions[].live` |
| `benefit` | `sessions[].benefit` |
| `ticket` | `sessions[].ticket` |
| `images` | `sessions[].images` と `image_url` |
| `source_refs` | `sessions[].source_refs` |
| `evidence` | 公開用には出さず、候補・レビュー記録に残す |

公開前に `data/curated/entities.json` の canonical ID と照合し、例外は `data/curated/overrides.json` に宣言的に記録します。候補 JSON を直接 `data/generated/schedule.json` として配信してはいけません。
