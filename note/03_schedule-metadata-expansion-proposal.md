# 03: 公開スケジュールのメタデータ拡充案

## 1. 背景と現状

`taite-board` は、外部ソースから取得した投稿を人間確認・補正し、Web と将来の iOS ネイティブアプリが共通で読む `schedule.json` を静的配信する方針を採用している。

現時点の公開データには、次の情報がある。

- ルート: `schema_version`, `generated_at`, `timezone`, `target_group`, `current_next_id`, `promotions`, `sessions`
- 公演: `id`, `title`, `date`, `venue`, `live`, `benefit`, `photo`, `ticket`, `image_url`, `source_tweet_url`, `images`
- 名寄せ辞書: `data/curated/entities.json`
- 手動補正: `data/curated/overrides.json`

一方、設計資料が想定している `extract`、`resolve`、`publish` の実装はまだ存在せず、現在の `data/generated/schedule.json` は手動で構成されたデータを基準にしている。したがって、最初に必要なのはフィールドを大量に増やすことではなく、Web と iOS の両方が同じ意味で扱える最小限のメタデータを決め、schema・TypeScript 型・生成データ・キュレーションデータのずれをなくすことである。

## 2. 目的と対象外

### 目的

- 公演の開催状態とタイムテーブルの確定状態を、時刻文字列だけに依存せず表現する。
- どの情報をいつ、どの出典から確認したかをクライアントが追跡できるようにする。
- Web と iOS の表示差を減らし、将来のローカルキャッシュ・鮮度表示に利用できるようにする。
- 既存の `schema_version: 1` を直ちに破壊せず、段階的に拡張できる形にする。

### 対象外

- ユーザー認証、ユーザーごとのお気に入り、通知サーバー。
- 動的 API、データベース、リアルタイム投稿機能。
- OCR や LLM による未確認情報の自動公開。
- 公開データのための専用キュレーション UI の実装。

## 3. 現在確認できる不整合

| 領域 | 現状 | 課題 |
| --- | --- | --- |
| 時刻未確定 | `--:--` などの表示値で表現 | 「告知待ち」「取得失敗」「本当に未定」の区別がつかない |
| 公演状態 | `overrides.json` に自由な `status` があるが schema で定義されていない | Web と iOS が状態を別々に解釈する可能性がある |
| 出典 | `source_tweet_url` と `source_refs` が併存し、公開データでは `source_refs` が未設定の公演もある | 出典の扱いがクライアントごとに分岐する |
| 会場 | 名寄せ辞書には `canonical_id` と `map_query` があるが、生成済み公演では名前と地図 URL が中心 | 会場の同一性と表示情報が分離されていない |
| 撮影ルール | schema と TypeScript は `photo.available`、既存の生成スクリプトは `photo.allowed` | 同じ意味のフィールド名が食い違っている |
| Promotion | 生成スクリプトには `type`、`badge`、`cta_label`、`priority`、`active` があるが、TypeScript 型・schema の定義が追いついていない | 宣伝枠の表示がデータ契約から漏れている |
| 生成経路 | `scripts/generate_real_schedule.js` にイベント情報がハードコードされている | `raw`、`candidate`、`curated`、`generated` の境界が実装上はまだ閉じていない |

## 4. 最初に追加する最小フィールド案

既存クライアントへの影響を抑えるため、まずは任意フィールドとして追加し、schema・TypeScript 型・サンプルデータを同時に更新する。

### 4.1 公演状態

`Session.status` を定義し、時刻の有無から状態を推測しない。

```json
{
  "status": "scheduled"
}
```

候補値は次の範囲に留める。

- `scheduled`: 公演予定が確認済み
- `timetable_pending`: 公演は告知済みだが、出番または特典会の詳細待ち
- `postponed`: 延期
- `cancelled`: 中止
- `unconfirmed`: 情報は存在するが、公開確認が完了していない

`completed` や `live` は現在時刻からクライアントが計算する表示状態であり、配信データの恒久的な `status` には混ぜない。

### 4.2 確認時刻

```json
{
  "last_confirmed_at": "2026-10-30T03:00:00+09:00"
}
```

`generated_at` はフィード全体の生成時刻、`last_confirmed_at` はその公演の情報を最後に確認した時刻として意味を分ける。時間の比較に使う値は ISO 8601 / RFC 3339 のタイムゾーン付き形式に統一する。

### 4.3 出典参照

`source_tweet_url` は後方互換のため残し、段階的に `source_refs` を正本とする。

```json
{
  "source_refs": [
    {
      "type": "x_post",
      "source_id": "2112891667640656194",
      "url": "https://x.com/Ribbon_Clock/status/2112891667640656194",
      "observed_at": "2026-10-30T02:55:00+09:00"
    }
  ]
}
```

最初の段階では `type`、`source_id`、`url` を必須とし、`observed_at` は任意とする。画像 URL は出典 URL と混同せず、`images` の各要素に保持する。

### 4.4 会場の正準情報

```json
{
  "venue": {
    "canonical_id": "venue:shibuya-ring",
    "name": "渋谷RING",
    "map_query": "渋谷RING",
    "map_url": "https://maps.google.com/?q=渋谷RING"
  }
}
```

`canonical_id` は内部同一性、`name` は表示、`map_query` は地図検索、`map_url` は明示されたリンクとして役割を固定する。クライアント側で会場名から地図 URL を推測しない。

### 4.5 Promotion の契約化

既存の生成データが持つ宣伝枠の情報を、schema と TypeScript 型に反映する。

```json
{
  "id": "promo-ayyu-birthday",
  "type": "hero",
  "badge": "20TH BIRTHDAY",
  "title": "神楽あゆ 生誕祭クラウドファンディング",
  "subtitle": "初めての生誕祭、そして20歳という大切な節目",
  "cta_label": "生誕詳細・支援",
  "cta_url": "https://x.com/Ribbon_Clock/status/2114643208710033610",
  "image_url": "https://example.com/promotion.jpg",
  "priority": 100,
  "active": true,
  "deadline": "2026-11-22"
}
```

`priority` と `active` は表示制御用のデータであり、クライアントが配列順だけを信頼しないために用意する。広告・宣伝を意味するフィールドと、ライブ公演そのものの確定情報は同じ `Session` に混ぜない。

## 5. 互換性を保つための段階的な実装順序

1. `photo.available` と `photo.allowed` のどちらを正とするか決め、全ファイルを一括で同じ名前へそろえる。
2. `Promotion` の schema と TypeScript 型を現行データに合わせる。
3. `Session.status`、`last_confirmed_at`、`source_refs`、`venue.canonical_id` を任意フィールドとして追加する。
4. `data/generated/schedule.json` の全公演に対して、出典・状態・確認時刻の欠落を検査する。
5. Web と iOS の両方で、未定時刻・延期・中止・出典表示を同じ fixture で確認する。
6. 候補抽出・名寄せ・公開生成の実装を追加する段階で、`data/curated/overrides.json` を公開データ生成へ接続する。

現行の `--:--` 表現をすぐに nullable な時刻へ変更すると、Web と将来の Swift モデルの両方に破壊的変更が生じる。そのため、まずは `status` を追加して意味を補い、時刻を `null` と ISO 8601 の日時へ移行する案は `schema_version: 2` の検討事項として分離する。

## 6. この案から先行実装した範囲

この worktree では、現行の Web リファクタや将来の iOS 実装と衝突しにくい範囲として、次を先行反映した。

- Promotion の `type`、`badge`、`cta_label`、`priority`、`active` を schema と TypeScript 型に追加。
- `source_refs.observed_at`、`Session.status`、`Session.last_confirmed_at` を将来利用できる任意フィールドとして追加。
- 現行11公演の `venue.canonical_id` と `map_query` を `data/curated/entities.json` から補完。
- 既存の `source_tweet_url` から、X投稿またはチケットページの `source_refs` を補完。
- 生成スクリプト内の撮影可否フィールドを `photo.available` に統一。

`status` と `last_confirmed_at` は、根拠のない値を埋めないため、今回の既存データには一律追加していない。状態や確認時刻を確定できる生成経路ができた段階で追加する。

## 7. 未決定事項

- `status` を公演の編集状態として扱うか、公式告知の状態として扱うか。
- `last_confirmed_at` の確認者・確認方法をどこまで記録するか。
- `source_refs` を公開データの必須項目にするタイミング。
- `event_url` を `ticket.url` と分離するか。チケット URL がイベントの一次 URL を兼ねる場合の扱い。
- `schema_version: 1` の後方互換範囲と、日時フィールドを再設計する時期。

## 8. 参考文献

- [JSON Schema Draft-07 Core Specification](https://json-schema.org/draft-07/json-schema-core.html)
- [JSON Schema Draft-07 Validation Specification](https://json-schema.org/draft-07/json-schema-validation.html)
- [RFC 3339: Date and Time on the Internet](https://www.rfc-editor.org/rfc/rfc3339)
- [JSON Schema Specification Links](https://json-schema.org/specification-links.html)
