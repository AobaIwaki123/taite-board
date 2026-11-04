# 0020. フェス合同タイムテーブル機能のアーキテクチャ・データスキーマ・動的セッションバインディング

- **ステータス**: 承認 (Accepted)
- **日付**: 2026-11-04
- **関連ADR**:
  - [ADR-0001: 統一スケジュールデータ規約](./0001-schedule-json-data-contract.md)
  - [ADR-0017: グローバルタイポグラフィトークンと構造的ガバナンス](./0017-global-typography-tokens-and-structural-governance.md)
  - [ADR-0019: ポータルウォレットカードスタックアーキテクチャ](./0019-portal-wallet-card-stack-architecture.md)

---

## 1. コンテキストと課題 (Context & Problem)

アイドル現場において、大型フェスや対バン（例: アイドルマリアージュ、@JAM、TIF等）では、1日に数十組のグループが複数ステージ・並行物販ブースで同時に稼働します。

従来の `taite-board` は特定グループ視点の単一スケジュール表示（CockpitView）に特化していましたが、ファンが現場で複数グループを回遊（ホッピング）するにあたり、以下の課題が生じていました：

1. **現場での同時性の把握困難**:
   - 「今、ステージAで誰がライブ中か」「今、ブースCで誰が特典会中か」を瞬時に把握する横断的な視点が欠落していた。
2. **トップ画面（ポータル）の責務混乱**:
   - ポータル画面に特定のフェス（例: 11/3 マリアージュ）の導線を決め打ちで配置すると、「グループ選択の玄関口」というポータル本来の役割が破壊され、特定イベントへの密結合が生じる。
3. **過大実装（Heavy Policing）の誘惑**:
   - 全フェスのタイテを自動収集・解析するために、重厚なマルチステージバッチクローラーや画像解析OCR基盤を自前構築すると、メンテナンスコストや外部HTML/画像フォーマット変更による脆弱性が増大する。

---

## 2. 決定事項 (Decisions)

### (1) セッション動的バインディング (Dynamic Session Binding)
- **原則**: ポータル画面にはフェスタイテへの決め打ちリンクを置かず、**「各グループ詳細で現在閲覧している公演（Session）の文脈」からのみ動的に導線を展開**する。
- **データ契約**:
  - `Session` スキーマにオプショナル項目 **`cross_track_event_id?: string`**（例: `"2026-11-03/mariage"`）を追加。
  - CockpitView では、開いている公演に `cross_track_event_id` が存在する場合にのみ、「当日のタイムテーブル ❯」バナーを表示する。
  - ワンマン公演や、合同タイテが存在しない公演ではバナーを表示しない。
  - 遷移URLには `from=${groupId}` を付与し、フェスタイテ画面から1タップで元のグループ詳細へ戻れる閉包（Closed Loop）を担保する。

### (2) AIベースの軽量データ生成運用 (Cognitive Extraction over Heavy Crawlers)
- 自前で重厚なOCR・画像パーサーをスクラッチ開発することを厳禁とする（過大実装・過剰防衛の抑止原則）。
- 公式X等で告知されたタイムテーブル画像やテキスト情報を、AI（LLM）の高度な認知能力を活用して直接正規化JSON（`CrossTrackEvent`）へ変換・生成する。
- 人間がレビューして `data/events/{date}/{slug}.json`（または静的データ定義）に配置する半自動・オンデマンド運用を採用する。

### (3) 機械式案内板UI（Signboard Detailed）の標準化
- コンポーネントライブラリを並べただけの平坦なカードを脱却し、大型フェス会場のエントランス案内板としてのリアリティを追求する：
  1. **左柱の独立性**: `LIVE 01`（ステージ・出番順）および `特典会 並行A`（物販ブース位置）を左側 52px の独立柱に刻印。
  2. **反転フラップの水平ヘアライン溝**: カード中央を横切る 1px の極細スリットにより、機械式案内板のプレート感を表現。
  3. **金属ボルトピン留め**: 左柱の上下に極小のボルトピン（リベット）を刻印。
  4. **5分刻みのタイム目盛り（Ticks）**: 底辺に `|···|···|` の計器目盛りを刻み、枠内の現在位置を精密に可視化。
  5. **勝手なレインボー着色・自己主張バッジの全廃**: 推測による偽のカラーパレットや `MY IDOL` 等の陳腐なラベルを全廃し、深いモノトーン（Jet Black × Stone × White）に統一。

---

## 3. データスキーマ仕様 (Data Schema Contract)

### (1) `Session` 拡張スキーマ (`schema/schedule.schema.json`)
```json
{
  "cross_track_event_id": {
    "type": "string",
    "description": "Identifier for the multi-group festival cross-track event timeline (e.g. 2026-11-03/mariage)"
  }
}
```

### (2) `CrossTrackEvent` スキーマ (TypeScript 定義)
```typescript
export interface CrossTrackSession {
  id?: string;
  groupId?: string;          // 登録グループID（推しハイライト等に利用、例: "ribonclock"）
  groupName: string;         // 正式グループ名（略称ではなく正式名称）
  type: 'live' | 'benefit';  // 'live' (ステージ) または 'benefit' (特典会)
  label: string;             // "ライブ出番", "特典会" など
  startTime: string;         // "HH:MM" (例: "11:55")
  endTime: string;           // "HH:MM" (例: "12:15")
  location?: string;         // "並行A", "並行C", "メインステージ" など
  detail?: string;
}

export interface CrossTrackEvent {
  id: string;                // "YYYY-MM-DD/{event-slug}"
  date: string;              // "YYYY-MM-DD"
  dayOfWeek: string;         // "火祝", "土" など
  title: string;             // フェス正式名称 (例: "アイドルマリアージュ Vol.79")
  venueName: string;         // 会場名 (例: "ベルサール虎ノ門")
  venueUrl?: string;         // Google Map 等のURL
  ticketUrl?: string;        // チケット販売ページURL
  sessions: CrossTrackSession[];
}
```

---

## 4. AIベースのデータ抽出プロトコル (AI Extraction Prompt)

フェス告知のタイムテーブル画像やテキストから、以下のシステムプロンプトを用いて一発で正規化JSONを生成する：

```markdown
以下のアイドルフェスのタイムテーブル情報（画像またはテキスト）から、
全出演グループの出番（live）および特典会（benefit）を抽出し、
CrossTrackEvent 型の JSON データを生成してください。

【抽出ルール】
1. グループ名は略称を使わず、必ず正式名称（groupName）を抽出すること。
2. 特典会の場所（並行A、並行B、終演後物販など）は location フィールドに正確に格納すること。
3. 開始・終了時刻は 24時間表記の "HH:MM" 形式で正規化すること。
4. 自社管理グループ（リボンクロック: ribonclock、Dolly Kiss: dollykiss 等）に該当する場合は groupId を付与すること。
```

---

## 5. 結果とメリット (Consequences & Benefits)

- **文脈に沿った自然なUX**: ユーザーは自分が見ている公演から迷わずワンタップで該当フェスのタイムテーブルへ到達し、ワンタップで元の公演へ戻れる。
- **アーキテクチャの純粋性維持**: ポータルはグループ一覧としての純度を保ち、不要なイベント癒着・コードのハードコードを完全に撲滅。
- **運用摩擦の極小化**: クローラーの障害対応やパーサーの保守に工数を奪われず、AIの認知能力で新規フェスタイテを数分で即時データ化可能。
