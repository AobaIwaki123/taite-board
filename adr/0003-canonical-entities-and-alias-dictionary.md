# ADR-0003: canonical_id とエンティティ名寄せ辞書の設計

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- `data/curated/entities.json`
- `internal/resolve/`

## 背景と課題
アイドルのライブ告知では、同一の対象が媒体や告知ツイートごとに異なる文字列で表記される（表記揺れ）。
- グループ名: 「リボンクロック」「リボクロ」「Ribbon Clock」「Ribbon Clock🎀」
- 会場名: 「渋谷RING」「SHIBUYA RING」「渋谷リング」
- チケットサービス: 「TicketDive」「ticketdive」「チケットダイブ」「TIGET」「チゲット」
これを文字列のまま素朴に扱っていると、会場マップクエリの生成ミス、同日2回し公演の重複判定ミス、チケットプラットフォームごとの適切なUIバッジ表示の破綻につながる。

## 決定事項
1. **安定した `canonical_id` の体系化**:
   - エンティティごとに `entity_type:kebab-case-slug` の識別子体系を定義する。
     - グループ: `group:ribbon-clock`
     - メンバー: `member:ribbon-clock:ayu-kagura`
     - 会場: `venue:shibuya-ring`, `venue:shinjuku-zirco-tokyo`
     - チケットプラットフォーム: `platform:ticketdive`, `platform:livepocket`, `platform:tiget`, `platform:eplus`
2. **名寄せ辞書（`data/curated/entities.json`）の管理**:
   - 以下の構造を持つ正準辞書を整備する。
   ```json
   {
     "entities": {
       "groups": [
         {
           "canonical_id": "group:ribbon-clock",
           "display_name": "リボンクロック",
           "aliases": ["リボクロ", "Ribbon Clock", "Ribbon Clock🎀"],
           "source_keys": { "x_user_id": "21118095222" }
         }
       ],
       "venues": [
         {
           "canonical_id": "venue:shibuya-ring",
           "display_name": "渋谷RING",
           "area": "渋谷",
           "map_query": "渋谷RING",
           "aliases": ["SHIBUYA RING", "渋谷リング", "RING"]
         }
       ]
     }
   }
   ```
3. **決定論的解決プロトコル（6段階）**:
   1. 安全な文字列正規化（Unicode NFKC、前後空白・連続空白除去、ASCII小文字化）。
   2. `canonical_id` 完全一致。
   3. エイリアス辞書（`aliases`）完全一致。
   4. ソースキー（XユーザーID、公式URL等）による照合。
   5. 高確信度の前方一致・部分一致（明らかなビル名等を除いた照合）。
   6. 未解決（Unresolved）として人間確認キューへ送致。

## 採用理由と設計根拠
- **決定論性と再現性**: 機械学習やLLMによる推測を名寄せの中核から外し、辞書ベースの完全一致・エイリアスマッチを第一とすることで、100%予測可能な結果を保証する。
- **データ契約の安定化**: UIやクライアントは `canonical_id` に紐づく公式名称・地図クエリ・チケットアイコン色を参照できるため、生ツイートの表記揺れによるレイアウト崩れを防げる。

## 検討した代替案と却下理由
- **LLMによる動的推測と自動置換**:
  - 却下理由: 似た名称の別会場（例: 「渋谷Club Asia」と「渋谷O-EAST」）の誤認混同や、ハルシネーションによる誤った会場リンク生成のリスクが高く、ライブ現場での致命的な案内ミスにつながる。
- **UI側での動的文字列マッチング**:
  - 却下理由: クライアントごとに実装が重複し、Webとネイティブで表記や地図リンクが不一致になる。

## 影響範囲とリスク
- **新規会場・新規イベントの登録運用**: 初めて出演する会場やイベントは未解決になるため、名寄せ辞書（`entities.json`）へエイリアスを追加する運用フローが必要となる。
