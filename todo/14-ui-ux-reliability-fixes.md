# TODO-14: UI/UX の実不具合・データ不整合修正（高信頼項目）

- **ステータス**: 未着手（セルフレビュー起票）
- **優先度**: 中（ユーザー体験の正確性・操作安全性）
- **目的**: コード検証で確実に特定された UI/UX 上の不整合（古い静的データ表示・バージョン表記ズレ・一括反映の並列競合・管理画面認証体験の分断）を修正する。

## 監査で確認された現状と課題

1. **ポータル画面の次回ライブ表示が過去イベントで固定されている (`apps/web/src/components/PortalView.tsx`)**:
   - `PortalView.tsx` の `NEXT_EVENT_DATA`（57〜68 行目）に `ribonclock: { date: '11/3 (祝)', title: 'アイドルマリアージュ', venue: 'ベルサール虎ノ門' }`、`dollykiss: { date: '11/3 (祝)', title: 'アイドルマリアージュ / 横丁', venue: '都内複数会場' }` が静的ハードコードされている。
   - そのため、実際のスケジュール更新や公演終了後もポータルカード上の「次のライブ」表示が過去のイベントのまま更新されない。
2. **ポータル画面のバージョン表記がハードコードのままズレている (`apps/web/src/components/PortalView.tsx`)**:
   - `CockpitView.tsx` はビルド時定数 `__APP_VERSION__` を表示しているが、`PortalView.tsx` 349 行目は `<span>v1.4.3</span>` のまま固定されており、実バージョン（`v1.26.0` 系）と乖離している。
3. **管理画面「まとめて反映」が並列実行で競合・失敗しやすい (`apps/web/src/components/AdminView.tsx`)**:
   - `AdminView.tsx` 897 行目の `readyEntries.forEach((item) => void commitEntry(item))` がすべての未反映カードを同時並列で `/api/admin/intake/promote` へ送信する。
   - 1 件でも 409 重複エラーが出た場合やサーバー側書込が競合した場合に結果が不安定になり、ユーザーが進捗を追いにくい。順次処理（直列ループ）にして進捗とエラーを明確に扱う必要がある。
4. **管理画面の認証導線と監視画面の体験分断 (`apps/web/src/App.tsx`, `apps/web/src/components/AdminView.tsx`)**:
   - `App.tsx` では `/graph`・`/arch` のみが `<AdminAuthGate>` で保護されており、`/admin` だけ未ログイン時にも管理フォームが描画された後に 401/403 バナーが出る作りになっている。
   - また `AdminView.tsx` ヘッダーの「監視」リンクが Preact SPA とは別デザイン・別セッション画面の `/admin/dashboard` HTML を開くため、管理体験が一貫していない。

## 検討・実装項目

1. **`PortalView.tsx` の次回公演動的取得とバージョン定数統一**:
   - マウント時に `REGISTERED_GROUPS` 各グループの `dataUrl` を並列フェッチ（失敗時や未確定時のみフォールバック文言）し、`apps/web/src/utils/date.ts` の `resolveNextSession` を用いて次回（または当日進行中）のライブを自動表示する。該当する予定がない場合は「次回予定調整中」を表示する。
   - フッターの `v1.4.3` を `__APP_VERSION__` へ置き換える。
2. **`AdminView.tsx` の一括反映直列化と `<AdminAuthGate>` 統合**:
   - 「まとめて反映」実行時は `for (const item of readyEntries) { await commitEntry(item); }` で直列実行し、処理中状態と完了後のセッション再読込を 1 回にまとめる。
   - `App.tsx` で `<AdminView />` も `<AdminAuthGate title="Ribbon Clock 管理画面">` で保護し、未ログイン・権限不足時の導線を `/graph`・`/arch` と統一する（あわせて `AdminView.tsx` 内の冗長な旧認証メッセージ分岐を整理）。

## 検証条件

- [ ] `PortalView` で各グループの `schedule.json` に基づいた最新の次回ライブ（または予定なし表示）および `__APP_VERSION__` が表示され、過去の固定イベント文字列が残っていないこと。
- [ ] `AdminView` の「まとめて反映」が直列で実行され、複数件連続登録しても全件が欠落せず反映されること。
- [ ] `/admin`, `/graph`, `/arch` がすべて `<AdminAuthGate>` 経由で統一的なログイン・権限エラー表示を行うこと。
- [ ] `./scripts/verify.sh`（型チェック・テスト・ビルド）が通過すること。
