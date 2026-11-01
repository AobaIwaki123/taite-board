# ADR-0017: グローバル・タイポグラフィトークン定義と構造的ガバナンス

## ステータス

Accepted

## 決定日

2026-11-01

## 対象コンポーネント

- `apps/web/src/styles/main.css`
- `AGENTS.md`
- `note/07_mobile-typography-and-outdoor-readability.md`

---

## コンテキストと課題 (Context & Problem Statement)

Ribbon Clock は、屋外直射日光下での可読性を担保するため、`note/07_mobile-typography-and-outdoor-readability.md` において **実測 12px 未満（`0.75rem` 未満）のフォント指定の完全禁止（Floor Standard）** を規定している。

当初、この規約を遵守させるために正規表現を用いたソースコードの全文字静的走査スクリプト（AST/正規表現パーサー）およびビルド済み CSS 検査を構築した。しかし、以下の構造的課題が浮き彫りとなった。

1. **過大実装・オーバーエンジニアリング**:
   - 文字サイズの制約を守るためだけに、コメント除去、テストコードの文字列難読化、CSS パース処理など数百行の独自スクリプトを抱えることになり、保守コストが増大した。
2. **ツールの副作用と結合度の上昇**:
   - テストコード内に書いた検知用サンプル文字列（`text-[10px]`）を Tailwind CSS の JIT スキャナが誤抽出して本番 CSS にバンドルしてしまうなど、外部ツールとの不要な摩擦を生んだ。
3. **力技の監視 vs 構造的解決**:
   - 事後的に全コードを重厚な Linter で警察・監視（Policing）するアプローチは対症療法に過ぎず、**「そもそも利用可能な文字サイズをグローバルに一元定義し、直接指定を禁止する」** という構造的解決の方が圧倒的に身軽で破綻しない。

---

## 決定要因・評価軸 (Decision Drivers)

1. **構造的シンプルさ (Structural Simplicity)**:
   - 制約遵守のために過大なコード量や計算量を要する場合は、実装手段を強行せず構造そのものを単純化する。
2. **単一の情報源 (Single Source of Truth)**:
   - 利用可能な文字サイズを `main.css` 内に標準 CSS 変数・標準ユーティリティクラスとして集約する。
3. **外部依存の極小化 (Minimal Dependencies)**:
   - Tailwind などの特定ツールの任意記法（`text-[...px]`）に依存せず、Web 標準の CSS 変数（Vanilla CSS）をベースとする。
4. **AIベースの規約ガバナンス**:
   - `AGENTS.md` に「直接指定の禁止とグローバルトークンの使用」を明文化し、AI エージェントが自律的にルールを遵守する。

---

## 決定内容 (Decision Outcome)

重厚な静的走査スクリプト群（`typography.ts`, `typography.test.ts`, `verify-typography.sh`）を撤廃し、以下の構造的アプローチへ移行する。

### 1. グローバル・タイポグラフィスケールの定義 (`apps/web/src/styles/main.css`)

Web 標準の CSS 変数およびユーティリティクラスとして、以下の5階層を一元定義する。

```css
:root {
  --text-floor: 0.75rem;     /* 13.1px: 規約下限値。注記・バージョン・補足用 */
  --text-meta: 0.875rem;     /* 15.3px: 特典会ブース・チケット価格 */
  --text-body: 1rem;         /* 17.5px: 本文・会場名・日程 */
  --text-subheading: 1.25rem;/* 21.9px: イベント名 */
  --text-heading: 1.875rem;  /* 32.8px: 最重要時刻（出番・特典会） */
}

.text-floor { font-size: var(--text-floor); }
.text-meta { font-size: var(--text-meta); }
.text-body { font-size: var(--text-body); }
.text-subheading { font-size: var(--text-subheading); }
.text-heading { font-size: var(--text-heading); }
```

### 2. 開発規約（`AGENTS.md`）への禁止事項明文化

- コンポーネントおよびスタイル定義において、`text-[10px]` などの任意 px クラスや `style={{ fontSize: ... }}` の直接指定を禁止。
- 文字サイズは、上記のグローバル定義クラス（`.text-floor`, `.text-meta`, `.text-body` 等）または標準 Tailwind クラス（`text-xs`, `text-sm`, `text-base` 等）のみを使用する。

---

## メリットとトレードオフ (Consequences)

### メリット
- **コード量の劇的削減**: 検査用スクリプト約 350 行を削除し、ビルド・テスト時間を短縮。
- **外部ツール摩擦の解消**: Tailwind JIT によるテスト文字列の誤抽出問題を根本解消。
- **設計の透明性**: デザインシステムとしてのフォントスケールが `main.css` 1箇所に集約され、開発者および AI が迷わず参照可能。

### トレードオフ・留意事項
- 機械的な全行静的エラーではなく AI 規約によるガバナンスとなるが、ルールが「直接指定の禁止」と極めてシンプルであるため、遵守率は極めて高い。

---

## 参考文献 (Authoritative External Sources)

- [W3C: CSS Custom Properties for Cascading Variables Module Level 1](https://www.w3.org/TR/css-variables-1/)
- [W3C: Web Content Accessibility Guidelines (WCAG) 2.2 - Visual Presentation (1.4.8)](https://www.w3.org/TR/WCAG22/#visual-presentation)
- [Apple Developer: Human Interface Guidelines - Typography](https://developer.apple.com/design/human-interface-guidelines/typography)
