# アクセス集計のPrometheus型設計案

## ステータス

Proposed。設計整理のみであり、実装・マニフェスト変更はこの文書では行わない。

## 目的

Ribbon Clock のアクセス状況を、個人を識別せずに1時間単位で確認し、必要に応じて Discord へ通知する。現在の「Nginx の標準出力を CronJob が直接読む」方式は、Pod の入れ替え後に過去ログを取得できないため、永続化の責務を集計側へ分離する。

欠損は許容する。重視するのは、次の責務と保存先を明確にすることである。

- Web Pod はアクセスを受け付け、匿名化したメトリクスを内部公開する。
- 集計・時系列保存は専用Podが担当し、PVCは専用Podだけが読み書きする。
- 通知処理は保存済みの集計値を読むだけにする。
- 外部公開経路へ内部メトリクスやraw logを接続しない。

## 1. 現状の問題

現行構成では、Nginx の匿名JSONログをWeb Podの標準出力へ出し、集計スクリプトが Kubernetes API の `pods/log` から読み取る。

この方式には次の制約がある。

- Podが再作成されると、以前のPodのログを同じセレクタから取得できない。
- `--last-minutes 360` のように期間を広げても、保存されていないログは復元できない。
- CronJobの実行頻度と、ログの保持期間が同じ仕組みに結合している。
- 集計PodがKubernetesのPodログを読むため、`pods` / `pods/log` RBACが必要になる。

したがって、CronJobの頻度を調整する前に、アクセスを集計側へ届ける契約と、集計値を保存する場所を分ける。

## 2. 提案する構成

```text
Nginx
  -> access-exporter sidecar
  -> internal ClusterIP /metrics
  -> Prometheus-compatible metrics store
  -> PVC上の時系列データ
  -> report CronJob
  -> Discord Webhook
```

### 2.1 Web Pod

Web PodにはNginxと `access-exporter` sidecarを配置する。

Nginxは現在と同じく、次の情報だけを生成する。

```json
{"ts":"2026-10-30T14:23:05+09:00","route":"document","status":200}
```

ログの受け渡し方法は次のいずれかとする。

1. Nginxが `emptyDir` 上のファイルへ書き、sidecarがtailする。
2. Nginxがlocalhostのsyslogまたは専用ソケットへ送る。

初期案では、既存のNginx設定との変更量が少ない `emptyDir` + sidecarを候補とする。`emptyDir` はWeb Podの生存期間だけ有効であり、Pod再作成時の欠損は許容する。

`access-exporter` はraw logを外部へ返すAPIではなく、固定メトリクスだけを内部ポートで公開する。例えば次のようなCounterを使用する。

```text
# HELP ribonclock_access_requests_total Origin request count.
# TYPE ribonclock_access_requests_total counter
ribonclock_access_requests_total 184
```

初期版では、IPアドレス、URL、User-Agent、Cookie、動的なラベルをメトリクスへ含めない。`route` や `status` の内訳が必要になった場合も、固定値の低カーディナリティラベルに限定し、Discord通知には出さない。

### 2.2 内部メトリクスService

`access-exporter` のメトリクス用ポートだけを、ClusterIPの内部Serviceで公開する。

- 外部Ingress、Cloudflare Tunnel、公開ドメインには接続しない。
- `NetworkPolicy` で、metrics storeからのscrapeだけを許可する。
- `/metrics` はraw log本文を返さない。
- Web Podの公開用HTTPポートとメトリクス用ポートを分離する。

これにより、ユーザーのリクエストを受けるWeb Pod内に内部APIを置きつつ、公開アプリケーションと集計処理の責務を分離できる。

### 2.3 Metrics store

集計PodはWeb PodのPVCをマウントしない。内部Serviceを一定間隔でscrapeし、時系列データを専用PVCへ保存する。

候補は次の2つである。

| 候補 | 特徴 |
| --- | --- |
| Prometheus | Counter、scrape、`increase()`、リテンション、PVC保存を既存の概念で扱える。導入・運用コンポーネントは増える。 |
| 小さなPrometheus互換集計Pod | 必要なCounterと時間バケットだけをSQLite/JSONへ保存でき、軽量。ただしscrape、リセット、保持、APIを自前実装する。 |

設計の正本はPrometheusのデータモデルに合わせ、実装時に既存のPrometheus基盤があるかを確認して選択する。Prometheusを導入する場合も、Grafanaや公開ダッシュボードは本設計の必須要件ではない。

### 2.4 通知CronJob

通知CronJobはWeb Podの `pods/log` を直接読まない。Metrics storeの内部APIへ問い合わせて、指定期間の時間バケットを取得する。

初期候補は次のとおりとする。

- 実行頻度: 5分ごとではなく、必要な通知頻度を別途決める。検証時は5分ごとにできる。
- 表示期間: 直近6時間。
- 表示単位: JSTの1時間。
- 表示内容: 時間帯別アクセス数と合計だけ。
- 取得方法: `increase(ribonclock_access_requests_total[1h])` 相当の集計。

5分ごとに直近6時間を通知する場合、同じ時間帯を含む通知が重複する。これは累積カウンターへ加算する処理ではなく、毎回同じ期間を再計算するスナップショットとして扱う。

## 3. 保存と保持

### Web Pod側

- raw logの一時バッファは `emptyDir` またはlocalhostのソケットを使用する。
- Web PodのPVCは作成しない。
- Pod再起動時に未送信のログが失われることを許容する。

### Metrics store側

- PVCはMetrics storeだけが読み書きする。
- 初期の候補は単一レプリカ + `ReadWriteOnce` とする。
- 時系列データの保持期間は、まず7日程度を候補とし、容量を確認して決める。
- raw logをMetrics storeのPVCへ保存するかは別途判断し、不要なら保存しない。
- 長期保持するのは、時間バケットと件数などの匿名集計値だけにする。

Prometheusを採用する場合は、PVCサイズ、retention、scrape intervalを一体で決める。小さな集計Podを採用する場合は、SQLiteのバックアップと破損時の再構築手順を決める。

## 4. 障害時の扱い

| 事象 | 扱い |
| --- | --- |
| Web Pod再起動 | exporterの未送信分は欠損し得る。Pod UIDが変わるため、集計側は前Podと新Podを同一Counterとして無理に連結しない。 |
| exporter停止 | Metrics storeではscrape欠損として扱う。通知側はデータ欠損を0件と混同しない。 |
| Metrics store停止 | PVC上の既存データを維持し、復旧後にscrapeを再開する。通知はエラーとして終了し、0件通知を送らない。 |
| 通知CronJob失敗 | 集計値はMetrics storeに残るため、再実行で同じ期間を再通知できる。 |
| Cloudflareキャッシュヒット | originへ到達しないため、引き続き集計対象外とする。 |

特に「データなし」と「アクセス数0件」は分ける。Metrics storeがqueryを返せない場合は、0件のDiscord通知を送らず、Jobを失敗させる。

## 5. セキュリティとプライバシー

- metrics endpointはClusterIPとNetworkPolicyで内部限定にする。
- Metrics storeへ送る値はCounterだけにし、URL、IP、User-Agent、Cookie、リクエスト本文を送らない。
- 動的なIDをPrometheusラベルにしない。ラベルは固定値・少数に限定する。
- Discord Webhook URLは通知CronJobのSecretからのみ注入する。
- Metrics storeのPVCは公開サイトのServiceやIngressから参照できないようにする。
- 現在のアクセス集計CronJobは停止状態を維持し、永続化方式を決めた後に再開を判断する。

## 6. 既存案との比較

| 案 | 評価 |
| --- | --- |
| Web Podの標準出力をCronJobが `pods/log` から読む | 実装は簡単だが、Pod交換後の履歴が失われ、今回のlookback要件を満たしにくい。 |
| Web Podがraw log APIを公開し、集計Podがpollする | PVCを集計Podだけに置ける。cursor、raw logの認証、再送、APIレスポンス制限を設計する必要がある。 |
| Web Podのexporterがmetricsだけを公開し、Metrics storeがscrapeする | raw logを内部APIで公開せず、保存責務も分離できる。本設計の推奨案。 |
| Web PodのログをPVCへ直接保存する | 欠損は減るが、Web Podとストレージが結合し、RWXやログローテーションの運用が必要になる。 |

## 7. 導入手順案

1. `ribonclock_access_requests_total` のメトリクス契約と、保持期間・scrape間隔を確定する。
2. Web Podへ `access-exporter` sidecarと一時ログバッファを追加する。
3. metrics専用ClusterIP、NetworkPolicy、scrape設定を追加する。
4. Metrics storeを単一レプリカ + PVCで起動し、Pod再起動時のCounterリセットを検証する。
5. ReporterをMetrics storeのquery API利用へ切り替える。
6. dry-run、Discord通知、欠損時の0件誤通知防止を確認する。
7. 確認後にのみ、現在停止中の `access-report` CronJobを再開する。
8. 新方式が安定したら、`pods/log` を読む旧RBACと旧取得経路を削除する。

## 8. 未決定事項

- 実Prometheusを導入するか、SQLiteベースの小型Metrics storeを作るか。
- `emptyDir` ファイルtailとlocalhost syslogのどちらを採用するか。
- scrape間隔を60秒またはそれ以下にするか。
- Metrics storeの保持期間とPVC容量。
- raw logをMetrics storeへ保存するか、Counterだけを保存するか。
- Web Pod再起動直後に発生した欠損を通知へ明示するか。
- 将来のLAN限定管理ダッシュボードがMetrics storeのquery APIを共有するか。

## 参考資料

- [Prometheus data model](https://prometheus.io/docs/concepts/data_model/)
- [Prometheus metric types](https://prometheus.io/docs/concepts/metric_types/)
- [Prometheus querying basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Prometheus storage](https://prometheus.io/docs/prometheus/latest/storage/)
