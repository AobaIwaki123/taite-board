# TODO-07: 家計簿のドリンク代任意入力対応とプライバシー注意書きの明記

- **ステータス**: 完了（任意入力・注意書き反映済み）
- **目的**: 固定選択肢にないドリンク代（例: ¥800、2D ¥1,200、¥650 等）を入力可能にし、あわせて利用者が記録内容の公開範囲（管理者側にも共有・閲覧されないこと）を誤解なく理解できるようにする。

## 現状と課題

1. **ドリンク代が 4 択の固定型になっている**:
   - `apps/web/src/utils/expense.ts` で `DRINK_FEE_OPTIONS = [0, 500, 600, 700] as const`、`SessionExpense.drinkFee` が `DrinkFeeOption` 型に固定されている。
   - `apps/web/src/components/ExpenseMemoCard.tsx` の UI も 4 つのボタンのみで、チケット代欄のような自由数値入力フォームが存在しない。
2. **プライバシー・非共有範囲の注意書きが不足している**:
   - `ExpenseMemoCard.tsx` 末尾および `apps/web/src/components/SyncAccountModal.tsx` には「未ログイン時はブラウザ内保存」「ログイン時はクラウド同期」とのみ記載されており、「個人の出費・メモ・参戦記録が運営／管理者向け画面・公開集計に送信・表示されない」ことが明記されていない。
   - 実装上の実態も整理しておく:
     - 未ログイン時はブラウザの `localStorage` のみに保存され、サーバーへ一切送信されない。
     - クラウド同期ログイン時は端末間共有のため `gs://taite-board-state/users/<sha256(uid)>/sync.json` に保存されるが、管理 UI（`/admin`・`/admin/dashboard`）や Prometheus 集計には一切表示・利用されない設計になっている。

## 検討・実装項目

1. **ドリンク代の任意金額入力対応**:
   - `apps/web/src/utils/expense.ts` の `SessionExpense.drinkFee` 型を `number` に変更し、`DRINK_FEE_OPTIONS` は「クイック選択プリセット（`0, 500, 600, 700`）」として維持する。
   - `ExpenseMemoCard.tsx` のドリンク代セクションに、チケット代と同様の数値入力（`input type="number"`, `min="0"`, 任意の円単位入力可）を追加する。
   - 既存の保存データ・クラウド同期データ（`mergeSyncData`）および `calculateExpenseTotal` / `calculateMonthExpenseSummary` がプリセット外の数値（例: `800`, `1200`）を正しく合算することをテストで確認する。
2. **家計簿プライバシー注意書きの明記**:
   - `ExpenseMemoCard.tsx` の案内枠および `SyncAccountModal.tsx` の同期説明に、以下の実態に即した注意書きを追加する（過大な誇張や技術実態と矛盾する表現は避ける）。
     - 未ログイン時はこの端末のブラウザ内のみに保存され、外部サーバーや管理者には一切送信されないこと。
     - ログインしてクラウド同期を有効にした場合でも、他端末同期用の専用領域にのみ保持され、管理画面・管理者やアクセス統計に個人の家計簿・参戦内容が表示・共有されることはないこと。
   - 将来「ストレージ管理者からも読めない完全 E2E 暗号化（パスフレーズ鍵導出等）」が必要になる場合は別タスクとして切り出す。

## 検証条件

- [x] ドリンク代にプリセット外（例: ¥800、¥1,200、¥650）を入力・保存・リロード復元・月次集計できる。
- [x] `apps/web` 単体テスト（`expense.test.ts` 等）および `./scripts/verify.sh` が通過する。
- [x] `ExpenseMemoCard` および `SyncAccountModal` 上で非共有・プライバシーに関する注意書きが視認できる。
