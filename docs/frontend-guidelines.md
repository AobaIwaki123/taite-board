# フロントエンド設計・TypeScript コーディング規約 (frontend-guidelines.md)

本ドキュメントは、Ribbon Clock におけるフロントエンド（`apps/web/`）の設計思想、TypeScript コーディング標準、およびコンポーネント実装のベストプラクティスを定めます。

Google TypeScript Style Guide および React / Preact 公式ベストプラクティスに準拠し、保守性と視読性の高いコードベースを維持します。

---

## 1. コア原則

### 1.1 宣言的 UI の徹底（命令型 DOM 操作の排除）
- `document.getElementById` による要素探索や、`element.innerHTML = \`<div>...</div>\`` のような文字列テンプレート結合による命令型 DOM 構築を禁止します。
- UI は状態（State）から決定論的に導出される純粋な宣言的コンポーネント（TSX）として記述します。
- 状態変更時は DOM を手動で書き換えるのではなく、フレームワーク（Preact）の仮想 DOM 差分適用に委ねます。

### 1.2 グローバルスコープ汚染の根絶
- HTML 属性（`onclick="window.someFunction()"`）に頼ったグローバル関数注入を禁止します。
- イベントハンドラはすべてコンポーネントの `props` やスコープ内のクロージャとして型安全に受け渡しを行います。

### 1.3 厳格な型定義とドメインモデル同期
- `any` 型の使用を禁止します。
- データ構造は `schema/schedule.schema.json`（Draft-07）から導出された `src/types/schedule.ts` を唯一の正本とし、UI コンポーネント側で独自の重複インターフェースを乱立させません。

### 1.4 バンドルフットプリントの最小化
- モバイルや低速通信環境での高速描画（LCP 最小化）を達成するため、ランタイムライブラリは最小構成（Preact: ~3kB）を維持します。
- アイコンライブラリ（Lucide）は一括インポートを禁止し、使用するシンボルのみを個別インポートしてツリーシェイキングを最大化します。

### 1.5 屋外視認性とタイポグラフィ基準（Outdoor Readability Floor）
- 日中の野外フェスや移動中など、直射日光下のモバイル環境でも瞬時にタイテを確認できるよう、極小文字（12px未満）を完全に排除します。
- ルート HTML に `font-size: 17.5px` を適用し、アプリ内の最小文字サイズは `text-xs`（実測 13.1px）を下回らない設計基準を義務付けます。
- 会場名、出演時間、特典会時間などの最重要情報は `text-2xl`（約26px）以上の太字（Bold）で即座に視認できるようにします。

---

## 2. ディレクトリ構成規約 (`apps/web/`)

```text
apps/web/
├── src/
│   ├── types/               # ドメイン型定義 (schedule.ts)
│   ├── state/               # アプリケーション状態・データフェッチ・キャッシュ
│   ├── components/          # 宣言的 TSX コンポーネント
│   │   ├── Header.tsx       # トップバー (ロゴ・鮮度インジケーター)
│   │   ├── Navigation.tsx   # 下部タブナビゲーション
│   │   ├── SessionCard.tsx  # 各公演サマリーカード
│   │   ├── CockpitView.tsx  # 直近・選択中ライブ詳細画面
│   │   ├── ListView.tsx     # 全日程一覧画面
│   │   ├── CalendarView.tsx # 月間カレンダー画面
│   │   ├── LightboxModal.tsx# 画像拡大モーダル
│   │   └── PromotionBanner.tsx # 生誕クラファン告知枠
│   ├── utils/               # 日付計算・アイコンローダー等の純粋関数
│   ├── styles/              # Tailwind CSS v4 エントリーポイント (@import "tailwindcss", @theme)
│   ├── App.tsx              # ルートコンポーネント (タブ・画面切り替え管理)
│   └── main.tsx             # アプリケーション初期化・マウント
├── public/                  # 静的アセット (画像、ローカルモックJSON)
├── index.html               # SPA マウント用最小 HTML
├── package.json
├── tsconfig.json            # Strict Mode 有効化
└── vite.config.ts           # Preact & @tailwindcss/vite 設定
```

---

## 3. コンポーネント設計規約

### 3.1 Props の型付け
すべてのコンポーネントは、受け取る `props` のインターフェースを明示的に定義します。

```tsx
// 良い例
interface SessionCardProps {
  session: Session;
  isNext?: boolean;
  onSelect: (id: string) => void;
}

export function SessionCard({ session, isNext = false, onSelect }: SessionCardProps) {
  return (
    <div 
      class="soft-card rounded-2xl p-3 flex items-center gap-3.5 cursor-pointer hover:border-rose-400/40 transition-all"
      onClick={() => onSelect(session.id)}
    >
      {/* 宣言的な構造 */}
    </div>
  );
}
```

### 3.2 副作用とライフサイクルのカプセル化
- データの取得、キャッシュ書き込み、ローカルストレージ操作は `state/` 配下のフックまたはストアモジュールにカプセル化し、表示コンポーネント内に生 fetch や DOM 依存の非同期処理を混在させません。
