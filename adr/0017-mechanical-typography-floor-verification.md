# ADR-0017: 最小文字サイズ（Floor Standard）の機械的検証基盤

## ステータス

Accepted

## 決定日

2026-11-01

## 対象コンポーネント

- `apps/web/src/utils/typography.ts`
- `apps/web/src/utils/typography.test.ts`
- `apps/web/package.json`
- `scripts/verify-typography.sh`
- `scripts/verify.sh`
- `note/07_mobile-typography-and-outdoor-readability.md`

---

## コンテキストと課題 (Context & Problem Statement)

Ribbon Clock は、強烈な直射日光下や移動中での視認性・瞬読性を確保するため、`note/07_mobile-typography-and-outdoor-readability.md` において以下の設計規約を定めている。

1. ルート要素フォントサイズの拡大（`font-size: 17.5px`）。
2. アプリケーション全体における **実測 12px 未満（Tailwind `text-xs` / 0.75rem 未満）のフォント使用の完全禁止（Floor Standard）**。

しかしながら、これまでは「人間の目視レビュー」や「事後的な気づき」に依存しており、開発時や機能追加時に `text-[10px]` などの微小フォントが紛れ込むインシデント（直近のバージョン表示配置時など）が発生していた。

人為的ミスや主観的レビューに依存せず、CI やローカル検証において **12px 未満のフォント指定を機械的かつ決定論的（Deterministic）に検知・遮断する仕組み** が不可欠である。

---

## 決定要因・評価軸 (Decision Drivers)

1. **決定論的かつゼロ偽陰性（Zero False Negatives）**:
   - 12px 未満のクラス名（`text-[10px]`, `text-[11px]`, `text-[0.6rem]` 等）や CSS/インラインスタイルを確実に検出し、見逃さない。
2. **高速フィードバック（Sub-second Execution）**:
   - `npm test` や `./scripts/verify.sh` の実行時間を悪化させず、ミリ秒単位でローカル検証できること。
3. **多層防御（Defense in Depth）**:
   - 単一のレイヤー（ソースコードの正規表現検索のみ）に頼るのではなく、**ソースコード静的解析（Tier 1）** と **ビルド済み CSS バンドル解析（Tier 2）** の2系統で二重検知を行う。
4. **外部依存の極小化（Zero Heavy Dependencies）**:
   - 重厚なブラウザ自動化（Puppeteer/Playwright）を常時テストに持ち込まず、Node.js ネイティブ機能およびシェルスクリプトで完結させる。

---

## 決定内容 (Decision Outcome)

以下の2層防御アーキテクチャによる機械的検証パイプラインを確立する。

```
[開発時 / PR前]
   │
   ├── Tier 1: ソースコード検査 (apps/web/src/utils/typography.test.ts)
   │     ├─ 対象: apps/web/src/**/*.{tsx,ts,css,html}, apps/web/public/**/*.{html}
   │     ├─ 検証内容:
   │     │    ├─ Tailwind 任意サイズ class (text-[<size>]) の解析 (px < 12, rem < 0.75, em < 0.75 を禁止)
   │     │    ├─ CSS font-size 宣言の解析 (< 12px, < 0.75rem を禁止)
   │     │    └─ インライン style fontSize 属性の解析 (< 12px を禁止)
   │     └─ 実行契機: npm test (node:test)
   │
   └── Tier 2: ビルド済み CSS バンドル検査 (scripts/verify-typography.sh)
         ├─ 対象: apps/web/dist/assets/*.css
         ├─ 検証内容: 生成された CSS 全体の font-size プロパティを抽出し、12px / 0.75rem 未満のルールが存在しないことを検証
         └─ 実行契機: ./scripts/verify.sh (ステップ 6/6) & release.sh
```

### 1. 検知対象の構文規則と閾値

| パターン | 検出例 | 許容基準 | 違反判定 |
|---|---|---|---|
| Tailwind 任意文字サイズ | `text-[10px]`, `text-[11px]` | `>= 12px` (`text-xs` 以上) | `< 12px` または `< 0.75rem` |
| Tailwind rem 任意文字サイズ | `text-[0.65rem]`, `text-[0.7rem]` | `>= 0.75rem` | `< 0.75rem` |
| CSS font-size | `font-size: 10px;` | `>= 12px` | `< 12px` または `< 0.75rem` |
| JSX インラインスタイル | `fontSize: '11px'`, `fontSize: 10` | `>= 12px` | `< 12px` |

### 2. エラーメッセージの自己解決性 (Actionable Diagnostics)

違反が検出された場合、単に失敗させるだけでなく、開発者が即座に修正できるよう以下の情報を標準エラー出力に出力する。

- 違反箇所のファイルパスおよび行番号
- 検出された不正な指定（例: `text-[10px]`）
- 規約下限値（`text-xs` / `12px`）および参照すべきドキュメント（`note/07_mobile-typography-and-outdoor-readability.md`）へのリンク

---

## メリットとトレードオフ (Consequences)

### メリット
- **人為的ミスの完全排除**: コミット前および CI 段階で機械的にブロックされるため、微小フォントが本番環境へ流出しない。
- **高速性**: 正規表現およびストリーム解析により、全ファイルの検査が約 15ms 以内に完了する。
- **透明性と一貫性**: チームや AI エージェントの交代によらず、常に同一のタイポグラフィ基準が強制される。

### トレードオフ・留意事項
- 特殊な動的文字列結合（例: ``text-[${size}px]``）は静的解析を回避する可能性があるが、Tailwind のクラス名抽出原則（JITパージ対策）として完全な静的クラス名記述が義務付けられているため、実用上のリスクは無視できる。

---

## 参考文献 (Authoritative External Sources)

- [W3C: Web Content Accessibility Guidelines (WCAG) 2.2 - Visual Presentation (1.4.8)](https://www.w3.org/TR/WCAG22/#visual-presentation)
- [Apple Developer: Human Interface Guidelines - Typography](https://developer.apple.com/design/human-interface-guidelines/typography)
- [Google Material Design 3: Typography System](https://m3.material.io/styles/typography/overview)
