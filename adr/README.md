# アーキテクチャ意思決定記録 (ADR) ガイドライン

本リポジトリでは、アーキテクチャ設計および技術選定の変更を客観的・体系的に追跡するため、Michael Nygard 形式および MADR (Markdown Architectural Decision Records) に基づく ADR (Architecture Decision Record) を `adr/` 配下に蓄積します。

---

## 1. ADR 記述の基本原則

1. **技術的事実とトレードオフの重視**:
   - 個人の主観、感情的表現（「イケてない」「ダサい」「辛い」等）、誇張、情緒的な比喩は記述しません。
   - 技術的要件、性能数値、依存関係、運用制約などの客観的事実のみを判断根拠とします。
2. **代替案の比較（Considered Options）の必須化**:
   - 採用案の利点だけを並べるのではなく、検討した他の選択肢（代替案）とそれぞれのトレードオフ（メリット・デメリット）を公平に明記します。
3. **負の影響・リスク（Consequences / Negative）の記録**:
   - あらゆるアーキテクチャ選定にはトレードオフが存在します。選定によって発生する制約、移行コスト、運用負荷を隠蔽せず明記します。
4. **不変性（Immutability）**:
   - 一度 `Accepted` となった ADR は原則として過去改ざんを行いません。決定を変更・更新する場合は、新規 ADR を起票して `Supersedes ADR-XXXX` としてリンクします。

---

## 2. ADR 標準テンプレート

新規 ADR（`adr/XXXX-<title>.md`）を作成する際は、以下のテンプレートを厳守します。

```markdown
# ADR-XXXX: [決定事項の簡潔な要約]

## ステータス
[Proposed | Accepted | Rejected | Deprecated | Superseded by ADR-YYYY]

## 決定日
YYYY-MM-DD

## 対象コンポーネント
- [影響を受けるディレクトリ・パッケージ名]

## コンテキストと課題 (Context & Problem Statement)
[どのような技術的要件、制約、性能・保守性上の課題が発生していたかを客観的事実ベースで記述]

## 決定要因・評価軸 (Decision Drivers)
- [例: 型安全性の担保]
- [例: オフライン環境での表示速度]
- [例: 依存ライブラリのフットプリント最小化]

## 検討した選択肢 (Considered Options)
### 選択肢 1: [案の名称]
- **概要**: 
- **メリット (Pros)**: 
- **デメリット (Cons)**: 

### 選択肢 2: [案の名称]
- **概要**: 
- **メリット (Pros)**: 
- **デメリット (Cons)**: 

## 決定事項 (Decision Outcome)
[選択肢 X を採用する。その選定理由を決定要因に基づいて記述]

## 影響とトレードオフ (Consequences)
### 正の影響 (Positive)
- 
### 負の影響とリスク (Negative & Mitigations)
- 
```

---

## 3. ADR の一覧と採番

- ファイル名は `0001-<kebab-case-title>.md` の4桁連番形式とします。
- 破壊的変更やアーキテクチャ再編が発生した際は、必ず PR 起票時に ADR を作成または更新提案に含めます。

| 番号 | タイトル | ステータス | 決定日 |
|---|---|---|---|
| [ADR-0001](0001-schedule-json-data-contract.md) | Web・ネイティブ共通のschedule.jsonデータ契約とスキーマ管理 | Accepted | 2026-10-30 |
| [ADR-0002](0002-raw-snapshot-storage.md) | 生データ（Raw Snapshot）の保存構造と不変性の保証 | Accepted | 2026-10-30 |
| [ADR-0003](0003-canonical-entities-and-alias-dictionary.md) | canonical_id とエンティティ名寄せ辞書の設計 | Accepted | 2026-10-30 |
| [ADR-0004](0004-curation-boundary-and-overrides.md) | 自動抽出・名寄せと手動キュレーション（Overrides）の境界 | Accepted | 2026-10-30 |
| [ADR-0005](0005-static-json-distribution.md) | 静的JSON配信の継続方針とAPIサーバー化の条件 | Accepted | 2026-10-30 |
| [ADR-0006](0006-source-adapter-boundary.md) | 外部取得アダプター（Source Adapter）の責務分離 | Accepted | 2026-10-30 |
| [ADR-0007](0007-source-and-artifact-layout.md) | ソースコード・ビルド生成物のディレクトリ配置再編 | Accepted | 2026-10-30 |
| [ADR-0008](0008-client-offline-cache-and-freshness.md) | クライアント側のキャッシュとデータ鮮度（Freshness）の表示 | Accepted | 2026-10-30 |
| [ADR-0009](0009-kubernetes-gitops-and-nginx-container.md) | Kubernetes GitOps デプロイと Nginx コンテナ基盤 | Accepted | 2026-10-30 |
| [ADR-0010](0010-modern-typescript-frontend-and-multistage-docker.md) | フロントエンドのVite・モダンTypeScript刷新とマルチステージビルド化 | Accepted | 2026-10-30 |
| [ADR-0011](0011-ios-native-client-architecture.md) | iOSネイティブクライアント（SwiftUI）の採用と共通データ契約接続 | Accepted | 2026-10-30 |
| [ADR-0012](0012-multi-slot-promotions-and-deadline-countdown.md) | 複数プロモーション枠分離と締切カウントダウンのアーキテクチャ | Accepted | 2026-10-30 |
| [ADR-0013](0013-toolchain-modernization-vite8-ts7-tailwind4.md) | ツールチェーンの最新化とTailwind CSS v4移行 | Accepted | 2026-10-30 |
