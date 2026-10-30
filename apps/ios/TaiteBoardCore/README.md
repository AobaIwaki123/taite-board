# TaiteBoardCore

`TaiteBoardCore` は、iOS クライアントから利用する `schedule.json` の読み取り専用データ基盤です。

このパッケージは UI や iOS アプリの署名設定を持たず、次の責務だけを提供します。

- JSON Schema v1 に対応する Codable モデル
- スキーマバージョン、日時、タイムゾーン、セッション ID の検証
- `current_next_id` を利用した直近公演の解決とフォールバック
- `generated_at` に基づく鮮度判定
- seed / remote source と Application Support 向けディスクキャッシュの抽象化
- 不正な更新で最後の正常なキャッシュを上書きしない Repository

## ローカル検証

```bash
swift test
```

アプリ target は後続タスクで追加し、アプリバンドルの seed と `Info.plist` の feed URL をこのパッケージへ注入します。
