# TODO-04: 管理画面（/admin）のモバイル最適化・情報削減とデザインシステム統合

- **ステータス**: 未着手（UI/UX の細部確認を伴うため、他タスク完了後またはユーザーとの対話を交えながら進める推奨）
- **優先度**: 最優先（ただし UI/UX 判断の比重が高いトラック）
- **依存関係**: [`TODO-02`](02-deterministic-upsert-and-media-link-priority.md) の「新規/既存更新トグル不要の単一 Upsert API」と密接に連動します。

---

## 1. 背景と現行管理画面の課題

現行の管理画面 [`internal/monitor/admin.html`](../internal/monitor/admin.html)（1,679行）および [`internal/monitor/dashboard.html`](../internal/monitor/dashboard.html) は、公開 Web アプリ（[`apps/web/`](../apps/web/) 配下の Preact + TypeScript + Tailwind CSS）とは独立した素の HTML/CSS/JS で実装されており、スマートフォンからの日常登録操作で以下の深刻な UX 劣化が起きています。

### 確認済みの問題箇所（`admin.html`）
1. **レスポンシブグリッドの破綻（415-417行目 / 1468行目）**
   - CSS 上は `@media (max-width: 640px)` で `.f-grid` を 2 列（`repeat(2, minmax(0, 1fr))`）に落とす設計になっていますが、特記事項などの要素にインラインで `style="grid-column: span 4;"` が直書きされており、スマホ幅でもグリッドが 4 列分に押し広げられて画面全体が横に崩れます。
2. **iOS Safari フォーカスズームと操作窮屈さ（46行目）**
   - `button, input, select, textarea { font-size: 13px; }` のため、スマホの Safari で入力欄をタップするたびに画面が自動ズームして操作しづらくなります（`16px` 以上が必要）。
3. **過剰な項目展開・情報過多・二重スクロール**
   - `.topbar` に 6 要素以上が並びモバイルで折り返しが乱れます。
   - 候補カード 1 件あたり 12 項目以上のフォーム入力欄が最初から常時展開されており、スクロール量が膨大です。
   - サイドバーのフィード（履歴）領域が `max-height: 540px` で内側スクロールを持っており、ページ全体スクロールと競合して二重スクロールになります。
   - 公開側（`apps/web/`）のダークストーン基調デザインシステム（[`apps/web/src/styles/main.css`](../apps/web/src/styles/main.css)）と見た目が乖離し、Storage 状態や通知種別などの不要なバッジ・ラベルが多数並んでいます。

---

## 2. 確定している UI/UX 設計方針（厳守事項）

これまでのユーザーとの議論で明確に合意された制約は以下のとおりです。実装時は「足す」のではなく「徹底して削る」前提で組み立ててください。

1. **公開アプリ（`apps/web/`）とデザインシステムを完全統一する**
   - 管理画面も `apps/web/` の Preact + TSX + Tailwind CSS 基盤へ統合してビルドし、カラー・余白・角丸・ボタン・タイポグラフィを一致させること（`font-size: 16px` 以上の入力欄を徹底し iOS 自動ズームを防止）。
   - `admin.html` のような Go 埋め込み生 HTML での二重保守を解消する構成を目指すこと。
2. **余計なラベル・バッジは極力排除する**
   - 「Storage モード」「Bucket 名」「Discord 通知」「パイプライン詳細ステータス」といった作業に直接不要な補助バッジ・装飾ラベルを表示しないこと。
   - 登録作業に必要なのは「X URL 入力（または取り込み済み候補の本文確認）」「グループ選択」「主要入力項目」「確定ボタン」が中心です。
3. **項目の横並び配置を原則禁止し、縦 1 列配置を基本とする**
   - 入力項目を横に並べるとモバイル画面幅でほぼ確実に描画崩れ・入力しづらさを招くため、**フォーム項目は原則すべて縦 1 列に並べる**こと。
   - 例外として横並びを許容するのは、「**撮影可能曲の曲番号（例: 1〜2 桁の数値）＋ 撮影区分**」のように、文字幅・桁数が確実に短いことが保証されるペアのみとすること。タイトル・会場・時間・特典・URL 等は絶対に横並びにしない。
4. **活動履歴パネルは UX を悪化させるなら置かない**
   - 「履歴は基本的にあまりいらない。あってもよいが UX（一覧性・操作感・スクロールの軽快さ）を悪化させるなら不要」という立ち位置を採る。
   - 登録画面のサイドバーや下部に履歴タイムラインや二重スクロール領域を抱え込まず、メインの登録導線を最優先にすること（必要なら最小限の送信完了フィードバックのみに留める）。
5. **新規登録 / 既存更新トグルを UI から撤去する（TODO-02 前提）**
   - 判定は [`TODO-02`](02-deterministic-upsert-and-media-link-priority.md) の決定論的 Upsert API に委譲し、通常フローからトグル操作をなくすこと。
   - 同一日に複数公演があり自動統合先が一意に定まらない（API から競合・複数候補が返された）場合のみ、シンプルな候補選択プロンプトを出す設計にすること。
6. **段階的開示（Progressive Disclosure）の検討**
   - 日常の補遺登録（撮可曲、入場特典のみ追加など）や AI パース結果確認では、頻繁に触る項目と任意・詳細項目を視覚的に整理し、最初から 12 項目が壁のように並ばない工夫を検討すること（最終レイアウトはユーザーに確認を取りながら調整）。

---

## 3. 関連ファイルと認証の注意点

- バックエンド配信・認証保護:
  - [`deploy/nginx.conf`](../deploy/nginx.conf) 113-150行目: `/admin`, `/admin/`, `/admin/dashboard`, `/api/admin/*` は `:8083`（`internal/syncapi/handler.go`）へプロキシされる。
  - [`internal/syncapi/handler.go`](../internal/syncapi/handler.go): 30 日 HMAC Cookie（`taite_session`）による `requireAdmin` / `requireAdminPage` RBAC が実装されている。UI 移行時も 401 時のリフレッシュ・ログイン導線と RBAC 保護を絶対に緩めないこと。
- 関連 UI ファイル:
  - [`internal/monitor/admin.html`](../internal/monitor/admin.html)
  - [`internal/monitor/dashboard.html`](../internal/monitor/dashboard.html)（観測用ダッシュボードの統合・整理要否も必要に応じ確認）
  - [`apps/web/src/styles/main.css`](../apps/web/src/styles/main.css)
  - [`apps/web/src/components/Header.tsx`](../apps/web/src/components/Header.tsx), [`apps/web/src/components/Navigation.tsx`](../apps/web/src/components/Navigation.tsx)

---

## 4. 完了までの進め方メモ

1. まず [`TODO-02`](02-deterministic-upsert-and-media-link-priority.md) のバックエンド統合 API 仕様を確定させる。
2. 375px〜390px 幅（iPhone 標準サイズ）を基準にしたモバイル画面構成（配置する要素・削る要素・折りたたみ単位）をユーザーと確認する。
3. Preact コンポーネントとして実装し、[`internal/monitor/server.go`](../internal/monitor/server.go) 側の `/admin` 配信構成と接続したうえで `./scripts/verify.sh` および `go test ./internal/...` で検証する。
