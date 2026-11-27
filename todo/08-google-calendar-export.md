# TODO-08: 公演詳細からの Google カレンダー登録機能

- **ステータス**: 未着手（起票済み）
- **目的**: 利用者が各公演カードからワンタップで自分の Google カレンダーへ予定（時刻・会場・詳細メモ）を登録できるようにする。

## 現状と課題

- 現在 `apps/web/src/components/CockpitView.tsx` や `apps/web/src/components/ListView.tsx` にカレンダー登録導線（Google Calendar テンプレート URL や `.ics` 出力）が存在しない。
- 各公演データ（`Session`）には `date`・`title`・`venue`（`name`, `address`）・`live`（`start_time`, `end_time`）・`benefit`（`start_time`, `end_time`, `booth`）・`ticket.url`・`special_notes`・`links` が揃っているため、追加のバックエンド処理なしでフロントエンドのみで登録 URL を生成できる。

## 検討・実装項目

1. **Google カレンダー URL 生成ロジックの切り出し (`apps/web/src/utils/calendar.ts`)**:
   - エンドポイント: `https://calendar.google.com/calendar/render?action=TEMPLATE&...`
   - **時間範囲の決定ルール**:
     - `live.start_time` と `live.end_time` が両方確定（`--:--` 等でない）している場合: 基本の予定範囲とする。特典会終了時刻（`benefit.end_time`）も同一公演内に登録したいか（例: `ライブ開始〜特典会終了` にするか `ライブ時間のみ` にし `details` に特典会時間を記載するか）を設計・比較する。深夜 24:00 以降表記（例: `24:30`）や終了が翌日跨ぎになる場合の扱いも考慮する。
     - 時刻未定（`--:--` 等）の場合: Google カレンダーの終日イベント形式（`YYYYMMDD/YYYYMMDD+1day`）へフォールバックする。
     - タイムゾーンパラメータ: `ctz=Asia/Tokyo` を明示指定し、端末のタイムゾーン設定に引きずられないようにする。
   - **タイトル・場所・詳細 (`text`, `location`, `details`)**:
     - `text`: `[グループ名 or 公演タイトル]`（文字数や視認性を整理）
     - `location`: `venue.name`（可能であれば `venue.address` 併用）
     - `details`: 出演時間、特典会時間・場所、入場特典、特記事項、チケット URL、公式リンク（Google カレンダー URL 長の上限に配慮して簡潔に整形）。
2. **UI 配置**:
   - `CockpitView.tsx` の主要アクションエリア（マップ／チケット導線周辺、またはヘッダー補助ボタン）に「Google カレンダーに追加」ボタンを配置する。
   - モバイルブラウザ・iOS Safari 等で別タブ / カレンダーアプリ遷移が自然に行えることを実機またはヘッドレスで確認する。

## 検証条件

- [ ] 時刻確定公演（通常時間帯および深夜帯）・時刻未定公演（終日フォールバック）それぞれについて正しい Google カレンダー URL が生成される単体テストを追加する。
- [ ] `details` 内に不要な `undefined` 文字列や余計な空行が入らない。
- [ ] `./scripts/verify.sh` が通過する。
