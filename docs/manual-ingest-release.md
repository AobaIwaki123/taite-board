# 端末入力から公開までの初期リリース手順

## 目的と適用範囲

この手順は、X・Web の URL、フライヤー画像、タイムテーブル画像を端末から取り込み、根拠を残した候補 JSON を人間レビュー後に Ribbon Clock へ反映する初期運用を定義します。

現在の `taite` CLI に実装されているのは `fetch`、`validate`、`build` です。`extract`、`resolve`、`publish` は ADR とディレクトリ設計上の予定であり、まだ候補から公開 JSON を自動生成しません。そのため、初期運用では候補作成と公開反映を明確に分けます。Cloud Run の AI パーサーを導入する場合も、AI の出力は候補に留め、このリリースゲートを通します。

## 1. 作業場所を分ける

既存の作業ツリーと混ぜず、`origin/main` から作業用 worktree を作ります。

```bash
git fetch origin main
git worktree add \
  .worktrees/feat/manual-schedule-update \
  -b feat/manual-schedule-update \
  origin/main
cd .worktrees/feat/manual-schedule-update
```

skill の定義は `.agents/skills/taite-board-json-ingest/` にあります。skill の検証用スクリプトは、候補を作成するだけで公開成果物を書き換えません。

## 2. URL と画像を intake manifest にする

一意の `run_id` を決め、画像は raw の手動取得領域へコピーします。元画像は上書きせず、manifest には入力 URL と画像の SHA-256 を残します。

```bash
run_id="manual-20261030-example"

python3 .agents/skills/taite-board-json-ingest/scripts/stage_sources.py \
  --repo-root . \
  --stage-dir "data/raw/manual/${run_id}/images" \
  --output "data/candidate/manual/${run_id}.json" \
  --url "https://x.com/Ribbon_Clock/status/1234567890" \
  --image "/absolute/path/to/flyer.png" \
  --image "/absolute/path/to/timetable.png"
```

複数 URL・画像をファイルで渡す場合は、次のように 1 行 1 入力で指定します。`url:` と `image:` を付けると自動判定を固定できます。

```text
url:https://x.com/Ribbon_Clock/status/1234567890
image:/absolute/path/to/flyer.png
```

```bash
python3 .agents/skills/taite-board-json-ingest/scripts/stage_sources.py \
  --repo-root . \
  --stage-dir "data/raw/manual/${run_id}/images" \
  --input /path/to/inputs.txt \
  --output "data/candidate/manual/${run_id}.json"
```

## 3. 外部本文と画像を確認する

X 投稿本文が必要なら、URL をブラウザで開かず CLI の raw snapshot として取得します。

```bash
go run ./cmd/taite fetch \
  --out data/raw \
  "https://x.com/Ribbon_Clock/status/1234567890"
```

`fetch` は取得時刻から別の run ID を生成するため、完了後に表示された run ID を候補の `source_refs` と PR 本文へ記録します。

ローカル画像は `view_image` で確認し、次の事実をそのまま候補へ転記します。

- 公演名、開催日、会場名
- 出演時間、ステージ名
- 特典会の開始・終了時刻、形式、場所
- チケット URL、料金、販売プラットフォーム
- フライヤーとタイムテーブルの対応

確認できない時刻は `--:--`、確認できない文字列は `null` とします。日付や会場を曜日・地名から推定したり、別投稿から時間を補ったりしません。名寄せできない会場は `canonical_id: null` のまま `needs_review` にします。

## 4. 候補 JSON を完成させて検証する

`data/candidate/manual/<run-id>.json` の `sessions` を編集し、各セッションに入力 ID 付きの `source_refs` と、必要な `evidence` を付けます。`evidence` は公開用 JSON には出さず、レビュー用の根拠として保持します。

```bash
python3 .agents/skills/taite-board-json-ingest/scripts/validate_candidate.py \
  --file "data/candidate/manual/${run_id}.json" \
  --repo-root . \
  --require-session
```

この検証は、入力 ID の参照切れ、ローカル画像の hash 不一致、日付・時刻形式の誤り、出典のないセッションを検出します。raw 画像を PR に含めない場合、クリーンな checkout では次のように存在チェックを警告へ下げます。ただし、レビュー時の手元では通常の検証を通してください。

```bash
python3 .agents/skills/taite-board-json-ingest/scripts/validate_candidate.py \
  --file "data/candidate/manual/${run_id}.json" \
  --repo-root . \
  --allow-missing-local \
  --require-session
```

## 5. 名寄せと公開 JSON への手動反映

候補を公開対象にする前に、`data/curated/entities.json` でグループ・会場・プラットフォームの canonical ID を確認します。新しい表記揺れは辞書へ追加し、個別公演の例外や時間修正は `data/curated/overrides.json` に記録します。

現時点では publish コマンドがないため、レビュー済みの候補から `data/generated/schedule.json` へ必要なフィールドを人間が反映します。次のルールを守ります。

1. `data/raw/` は取得した事実として編集しない。
2. 候補だけで不明値を埋めず、公開用 `live.start_time` と `benefit.start_time` は `--:--` を許容する。
3. `data/generated/schedule.json` の変更は候補・辞書・上書きと同じ PR に含め、どの入力から生成したかを PR 本文に記載する。
4. ローカル開発用の `apps/web/public/schedule.json` は、公開正本と一致させる必要がある場合だけコピーし、`cmp` で確認する。Docker は `data/generated/schedule.json` を直接取り込む。

```bash
cp data/generated/schedule.json apps/web/public/schedule.json
cmp -s data/generated/schedule.json apps/web/public/schedule.json
```

## 6. リリース前ゲート

候補・辞書・生成 JSON の差分を確認したあと、次を実行します。

```bash
go test ./...
go run ./cmd/taite validate data/generated/schedule.json
npm --prefix apps/web run typecheck
npm --prefix apps/web run test
npm --prefix apps/web run build
node scripts/validate_schedule.js
```

`node scripts/validate_schedule.js` は画像・チケット URL へ HEAD リクエストを送るため、ネットワーク到達性を確認できる環境で実行します。失敗時は URL の失効、X 側の一時障害、画像 URL の `name=large` 付与漏れを切り分け、無視した理由を PR に残します。

任意で本番コンテナまで確認します。

```bash
docker build -t taite-board-local .
docker run --rm -d -p 8081:8080 --name taite-test taite-board-local
curl -fsS http://localhost:8081/healthz
curl -fsS http://localhost:8081/schedule.json | python3 -m json.tool >/dev/null
docker stop taite-test
```

## 7. PR とデプロイ確認

検証結果を PR 本文に記載し、`main` へ直接 push しません。

```bash
git status --short
git diff --check
git add data/candidate data/curated data/generated apps/web/public/schedule.json
git commit -m "data: update schedule from reviewed intake"
git push -u origin feat/manual-schedule-update
gh pr create --base main --head feat/manual-schedule-update \
  --title "data: publish reviewed schedule intake" \
  --body-file /path/to/pr-body.md
```

PR では少なくとも次を確認します。

- 候補の各セッションに URL または画像入力への `source_refs` がある
- 名寄せできない項目が `null` と `needs_review` のまま可視化されている
- `data/generated/schedule.json` が `go run ./cmd/taite validate ...` を通っている
- typecheck、テスト、Web build、URL 検証の結果が記録されている
- `apps/web/public/schedule.json` を変更した場合、生成正本との差分がない

PR を merge すると、GitHub Actions が Docker image を GHCR へ push し、`k8s/manifests/deployment.yaml` の short SHA を更新します。その自動コミット後に ArgoCD が同期します。

## 8. 本番反映後の確認とロールバック

Actions の deploy 成功、GitOps の image tag 更新、ArgoCD の同期を順に確認します。外部からは次を確認します。

```bash
curl -fsS https://ribonclock.iorin.net/healthz
curl -fsS 'https://ribonclock.iorin.net/schedule.json' | python3 -m json.tool >/dev/null
curl -fsSI https://ribonclock.iorin.net/og-image.png
```

データ誤りや画像破損が見つかった場合は、まず公開 PR を revert して通常の CI/CD を通します。GitOps の自動コミットを手で force push せず、必要なら ArgoCD の同期状況と対象 Pod のログを確認します。

## 将来の自動化境界

名寄せ辞書で決定論的に解決できた入力は自宅サーバー側で早期リターンし、未解決の候補だけを Cloud Run の AI パーサーへ委譲する設計とします。Cloud Run の結果も `data/candidate/` に戻し、`validate_candidate.py`、人間レビュー、通常の PR ゲートを通過するまでは公開しません。これにより、現行の terminal intake と将来の AI 委譲が同じ公開境界を共有できます。
