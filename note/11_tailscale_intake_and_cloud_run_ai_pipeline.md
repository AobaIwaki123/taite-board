# Tailscale モバイル Intake と Cloud Run AI パイプライン設計知見

## 1. 課題と背景

アイドル公式 X（旧 Twitter）やメンバーアカウントによる対バン・単独公演のタイムテーブル告知は、主に外出中やライブ前後のスマートフォン上で認知されます。
しかし、従来の手作業フローでは以下の摩擦が存在していました：

1. **PC 入力の認知負荷**:
   - スマホで告知ポストを見かけても、帰宅後に PC を起動してターミナルで `taite fetch` や JSON 手編集を行う必要があり、入力が遅延または忘却される。
2. **AI コストの最適化**:
   - 全ポストのテキストおよび画像を無差別にマルチモーダル LLM に投入すると、不要な従量課金が発生する。
   - 無料枠（Google Cloud $10 クレジット）を効率よく活用するため、テキストだけで完結するポストは自宅 k8s 内の機械処理で解決し、真に画像解析が必要な場合のみ Cloud Run へ委譲する責務境界が必要。

## 2. アーキテクチャ

自宅 Kubernetes クラスタで常時稼働している Tailscale エンドポイント（`ribonclock-dashboard`）を入口とし、以下の 4 段階パイプラインを構築しました。

```
[iPhone / Mobile Browser]
       │
       ▼ (Tailscale WireGuard / MagicDNS)
[k8s: ribonclock-dashboard (Port 8082)]
       │
       ├─ Step 1: Zero-Cost Raw Fetch (internal/source/x via Syndication API)
       ├─ Step 2: Deterministic Normalization & Entity Match (internal/resolve)
       │          - NFKC / 全角記号正規化
       │          - entities.json による会場・グループの確定
       │          - 本文正規表現による日時・出番・特典会抽出
       │
       ▼ needs_ai 判定 (出番未確定 かつ タイテ画像あり)
[Cloud Run: taite-parser (cmd/parser, gemini-2.5-flash)]
       │
       ▼ 構造化 Candidate レスポンス (確信度・抽出根拠付与)
[Tailscale Dashboard プレビュー & 即時反映]
```

## 3. 実装詳細

### 3.1 k8s Zero-Cost Fetch & Deterministic Resolver (`internal/resolve`)
- 外部 API キーや有料スクレイピングサービスに依存せず、公式 Syndication / oEmbed エンドポイントから投稿本文および画像 URL を取得。
- 全角数字（`０-９`）、全角コロン（`：`）、波ダッシュ（`〜`、`～`）を ASCII 正規化。
- `data/curated/entities.json` をバイナリ内へ埋め込み（`//go:embed`）、会場名（`渋谷RING`、`新宿ZircoTokyo` 等）やグループ名（`リボンクロック`、`リボクロ`）を決定論的に名寄せ。
- 本文内に `LIVE 18:30〜19:00` などの出番時刻が明記されている場合は `needs_ai = false` と判定し、AI 課金リクエストの発生を遮断。

### 3.2 Cloud Run AI Parser (`cmd/parser`, `internal/parser`)
- Google Cloud $10 クレジット枠内で運用可能な `gemini-2.5-flash` を採用。
- 画像のダウンロードおよび base64 展開、JSON モード指定（`response_mime_type: "application/json"`）による決定論的スキーマ出力を強制。
- 抽出結果には出番時刻、特典会時刻に加え、画像内のどの文字を根拠としたかのエビデンス（`quote`、`confidence`）を付与。

### 3.3 Tailscale 管理ダッシュボード UI (`internal/monitor/dashboard.html`)
- 外出先の iPhone ブラウザから `http://ribonclock-dashboard/` を開くだけで、最上部に URL 入力フォームが表示される。
- ポスト URL を貼り付けて「Fetch Post」を押すと、k8s 側で即座に本文・添付画像・抽出パラメータが表示される。
- タイテ画像が存在し出番時刻が空欄の場合、琥珀色の「Run Cloud Run AI Parser」ボタンが表示され、1 クリックで Cloud Run が実行されてリアルタイムに時刻が補完される。

## 4. 参考文献・公的ソース

- [Cloud Run Documentation (Google Cloud)](https://cloud.google.com/run/docs)
- [Gemini API Structured Outputs & Vision Reference (Google AI for Developers)](https://ai.google.dev/gemini-api/docs/structured-output)
- [Tailscale Kubernetes Operator & Service Exposure](https://tailscale.com/kb/1236/kubernetes-operator)
- [RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format](https://www.rfc-editor.org/rfc/rfc8259)
- [Unicode Standard Annex #15: Unicode Normalization Forms (NFKC)](https://www.unicode.org/reports/tr15/)
