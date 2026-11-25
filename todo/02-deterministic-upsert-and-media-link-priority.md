# TODO-02: 決定論的 Upsert 統合（新規/既存トグル廃止）、画像 ID 重複排除、代表リンク優先順位付け

- **ステータス**: 未着手
- **優先度**: 最優先（緊急課題）
- **並列着手**: 可能（Go バックエンド中心。API 互換を保つか Unified Endpoint を先に整備すれば他セッションと独立して進められます）

---

## 1. 背景と解決したい問題

現在の X ポスト登録〜名寄せパイプライン（[`internal/monitor/server.go`](../internal/monitor/server.go), [`internal/schedule/`](../internal/schedule/), [`internal/resolve/`](../internal/resolve/)）には、実運用上で以下の 3 つの問題が発生しています。

### 問題 A: 「新規登録 / 既存更新（isPatch）」の手動トグルが破綻している
- 現状は [`internal/resolve/resolver.go`](../internal/resolve/resolver.go) 内のテキストヒューリスティクスで `cand.IsPatch = true` を推測し、フロント（`admin.html`）上で「新規登録（`POST /api/intake/promote`）」と「既存更新（`POST /api/intake/patch`）」の 2 系統を人間に選ばせています。
- しかし実運用上、アイドルのライブは同日に多くても 1〜2 本しかなく、同日内で別公演が並ぶ場合でも開催時刻に十分な間隔があるか `1部` / `2部` の表記が伴います。
- にもかかわらず現行の `ApplyAddSession`（[`internal/schedule/add.go`](../internal/schedule/add.go)）は以下の弱点があるため、自動名寄せに失敗しやすい構造になっています:
  1. `ApplyAddSession` 先頭（71行目）で `Date`, `Title`, `VenueName` を必須としており、[`server.go`](../internal/monitor/server.go)（352-358行目）が欠損値を `"ライブ出演 (YYYY-MM-DD)"` と `"未定"` で埋めて渡すため、追記ツイートを登録すると既存公演とタイトルも会場も一致せず、**同日に不要な重複ダミーセッションが新規作成**されてしまいます。
  2. `add.go` 内のマッチ条件（181-185行目）は `titleMatched || (venueMatched && isPlaceholderOrSameTime)` のみであり、`reconcile.go` にある `1部` vs `2部`（`isDistinctSessionLabel`）やコンクリート時刻 `> 15分` 差（`shouldMergeSessions`）のガードが共有されていません。

### 問題 B: 画像の重複排除が働かず、2枚目以降の異なる画像も追加されない
- X のメディア URL には同じ画像 ID でも以下のような形式揺れが発生します:
  - `https://pbs.twimg.com/media/HVi3dC9bsAAinY6?format=jpg&name=large`
  - `https://pbs.twimg.com/media/HVi3dC9bsAAinY6.jpg?name=large`
- [`internal/schedule/reconcile.go`](../internal/schedule/reconcile.go) の `dedupeImages`（671-699行目）は URL 文字列の完全一致（`seen[u]`）しか見ていないため、表記違いの同一画像が重複保存されます。
- 一方、[`internal/schedule/add.go`](../internal/schedule/add.go) の `mergeSessionFields`（421-435行目）は `if len(imgs) == 0` のときしか `images` を更新しないため、先にフライヤー画像が登録されたセッションへ後からタイムテーブル画像を統合しても 2 枚目が追加されません。

### 問題 C: 名寄せ統合後に「情報量の薄いポスト」が代表リンクのまま固定される
- `mergeSessionFields`（[`internal/schedule/add.go`](../internal/schedule/add.go) 438-442行目）および `mergeReconciledSessions`（[`internal/schedule/reconcile.go`](../internal/schedule/reconcile.go) 517-519行目）が、どちらも **`source_tweet_url` を「空のときだけ埋める（First-Write-Wins）」実装**になっています。
- そのため、写真だけ・予告だけの情報量が薄い先行ポストが先に保存されると、後から詳細なタイテ・特典・チケット情報の公式ポストを名寄せしても、アプリ上の「公式告知」リンクが薄いポストに固定されたままになります。

---

## 2. 対象コードベースと主要箇所

- [`internal/schedule/add.go`](../internal/schedule/add.go): `ApplyAddSession`, `mergeSessionFields`, `buildPhotoMapFromAdd`
- [`internal/schedule/update.go`](../internal/schedule/update.go): `ApplyUpdateSession`
- [`internal/schedule/reconcile.go`](../internal/schedule/reconcile.go): `ApplyReconcileSchedule`, `shouldMergeSessions`, `mergeReconciledSessions`, `dedupeImages`, `addUniqueSourceRef`
- [`internal/resolve/resolver.go`](../internal/resolve/resolver.go): `IsPatch` 周辺
- [`internal/monitor/server.go`](../internal/monitor/server.go): `POST /api/intake/promote` (335行目), `POST /api/intake/patch` (491行目), `POST /api/reconcile` (801行目)
- [`internal/reconcile/runner.go`](../internal/reconcile/runner.go) & `cmd/reconcile/`: Cloud Run Batch Reconcile 側

---

## 3. 設計・実装要件

### 要件 1: 「新規 / 既存更新」トグル不要の決定論的 Upsert 統合
1. **共通の名寄せ・衝突判定ロジックを `internal/schedule/` に集約する**
   - `add.go` と `reconcile.go` で個別に書かれているマッチ判定を一本化し、`reconcile.go` の安全ガード（異なる `session_name` 例: `1部`/`2部` は非統合、異なる非プレースホルダー `venue.canonical_id` は非統合、具体的な `live.start_time` が 15 分超離れていれば非統合）を Upsert 時にも必ず適用すること。
   - **同日ユニークフォールバック**:
     - `date` が一致する既存セッションが **1件のみ** 存在し、かつ上記の非統合ガード（別部・別具体会場・時刻 15 分超差）に一切抵触しない場合は、タイトルがプレースホルダーや補遺ポスト（「撮可」「特典」「タイテ」等）であっても **決定論的に既存セッションへ統合する**。
     - 同一日に非統合条件となる複数セッション（例: `1部` と `2部`、昼公演と夜公演）がすでに存在し、かつ `target_session_id` 指定もタイトル/時刻の一致もない場合のみ、無理に誤統合せずエラー（または統合候補一覧を含む `409 Conflict` 等の明示的レスポンス）を返し、**人間による手動名寄せ・対象指定を例外時の逃げ道として残す**こと。
   - `ApplyAddSession`（または新設する `ApplyUpsertSession`）において、マージ先が存在する場合は `Title` / `VenueName` が空またはプレースホルダーであっても補遺フィールド（`admission_bonus`, `photo`, `special_notes`, `benefit`, `ticket`, `images`, `source_refs`）のみを安全に反映できるようにすること。
   - 明示的に `TargetSessionID` を指定して統合できるオプションも用意し、手動名寄せフォールバックからも同一コードパスを使えるようにすること。
2. **`server.go` の統合エンドポイント整備**
   - `POST /api/intake/promote`（または新設 `POST /api/intake/upsert` と `/api/intake/promote` の統合）が上記 Upsert を呼び出し、**新規作成したか既存へ統合したか（`action: "created" | "merged"`, `session_id`）** を返すようにすること。
   - 既存フロント（現行 `admin.html` や `TODO-04` 着手前の過渡期）が壊れないよう、`POST /api/intake/patch` も当面互換維持する（内部で同じ Upsert / 明示 `TargetSessionID` 更新へ委譲する）こと。

### 要件 2: 画像メディア ID 正規化による重複排除と複数画像蓄積
1. **正規化キー抽出ヘルパーの実装**
   - 例として X 画像 URL `https://pbs.twimg.com/media/<MEDIA_ID>?format=jpg&name=large` と `https://pbs.twimg.com/media/<MEDIA_ID>.jpg?name=large` やサイズ違い（`name=small`, `name=orig` 等）を同一メディア `twimg:<MEDIA_ID>` として正規化する関数を実装すること（クエリ・拡張子・プロトコル差を吸収）。
   - `http://` vs `https://`、末尾スラッシュ等の一般的な URL 正規化も行うこと。
2. **マージ時の画像蓄積**
   - `mergeSessionFields`（`add.go`）と `mergeReconciledSessions`（`reconcile.go`）の双方で、`len(imgs) > 0` でも新規画像を末尾追加したうえで正規化済み `dedupeImages` を通し、**同じ画像（URL 揺れ）は 1 枚に集約し、フライヤー＋タイムテーブルのような別画像は両方残す**こと。
   - 同一メディア ID が複数見つかった場合は、解像度・形式パラメータ（例: `name=large` / `name=orig` > パラメータ無し）が良好な URL を残すこと。

### 要件 3: 代表リンク（`source_tweet_url`）の情報量スコアによる優先採用
1. **情報量スコアリングの導入**
   - 「先に登録された URL で固定する」First-Write-Wins を廃止し、新規ポストや `reconcile` 時のマージ元・マージ先のそれぞれについて、情報網羅度に基づくスコアを算出して比較する関数を用意すること。
   - スコア要素の例（客観的なメタデータに基づくこと）:
     - 出番開始/終了時刻が具体的（`--:--` でない）
     - 会場が具体的（`未定` でない）
     - タイトルがプレースホルダー（`ライブ出演 (...)` 等）でない
     - 特典会時刻・チケット URL・入場特典・撮影可否ルールを含んでいる
     - 補助元 URL のテキスト情報量や種別（`source_refs` 内のメタデータ、あるいは投稿本文文字数・`is_pinned` の有無など）
   - ※ スコア判定の根拠データ（例: 当該セッションまたは `source_refs` 各要素の充足情報・テキスト長やスコア値、あるいは公式本告知優先フラグ）は `source_refs` やセッション構造に無理なく持たせ、既存の JSON Schema（[`schema/schedule.schema.json`](../schema/schedule.schema.json)）との整合性も確認・維持すること。
2. **手動固定の尊重（任意・余地確保）**
   - スコアに関わらず運用者が明示的に URL を差し替えたいケース（例: `UpdateOfficialURL: true`）は優先できるようにしつつ、通常の自動登録・自動名寄せではより情報量の多いポストが自動的に `source_tweet_url` へ昇格するようにすること。

---

## 4. 検証とデプロイ留意事項

- [ ] `go test ./internal/...` を充実させること:
  - X 画像 URL 揺れ（`?format=jpg&name=large` vs `.jpg?name=large`）が重複排除され、別 ID 画像は 2 枚以上保持されるテスト
  - 先行の薄いポスト（画像・概要のみ）に対し、後続の詳細告知を統合・reconcile した際に `source_tweet_url` が情報量の多いポストへ切り替わるテスト
  - 同日 1 公演に対して補遺テキストのみを `IsPatch` 指定なしで投入した場合に、ダミー公演を作らず既存セッションへ決定論的に統合されるテスト
  - 同日 2 公演（例: `1部` / `2部` や別時刻公演）が存在する場合に、誤統合されず安全に分離または候補提示エラーになるテスト
- [ ] `./scripts/verify.sh` が全通過すること
- [ ] **運用注意**: `internal/schedule/` や `internal/reconcile/` を変更し `main` へマージした場合、GitHub Actions でイメージビルドされるのに加え、Cloud Run Job 側の `taite-reconcile` が最新イメージを参照するように更新が必要になります（PR 本文または作業後チェックに明記すること）。
