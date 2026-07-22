# Change Log

## v1.4.1 - 2026-07-22

### Fixed

- worker: stop-timeout 対応により Stop 処理に時間がかかるようになり、メッセージループがブロックされる問題を修正
- worker: メッセージのデコードに失敗するとメッセージループが無言で終了する問題を修正
- generator: テスト終了後も次のテスト開始まで statsAggregator が残る問題を修正


## v1.4.0 - 2026-07-22

### Added

- ワーカーの直近のログをマスターに転送する機能 (launce.LogCapture, launce.WithLogCapture) を追加
- --stop-timeout による graceful stop に対応
- --reset-stats による spawn 完了後の統計リセットに対応
- ParsedOptions に --json-file, --profile, --otel を追加
- stats のエラー情報に first_seen, last_seen を追加

### Changed

- spawner.RestartNever を spawn 中にエラーになったユーザーを再 spawn しないようにしてスパイクを回避するように変更
- 元の spawner.RestartNever を spawner.RestartLocustCompatible として追加
- 対応する Locust のバージョンを 2.46.0 に更新

### Fixed

- generator: OnStart成功時にcancelが残ったままになる問題を修正
- generator: OnTestStart がエラーを返すと以降のテストでユーザーが生成されなくなる問題を修正
- wait: Between の引数に同じ値を渡すと panic になる問題を修正


## v1.3.0 - 2025-03-20

### Added

- ユーザー終了時の挙動を設定するオプション launce.WithRestartMode を追加


## v1.2.0 - 2025-01-04

### Added

- ユーザーシナリオで発生したエラーをキャッチするかどうかを指定するオプションを追加

### Changed

- Workerのオプション指定をFunctional Optionsに変更
- ParsedOptionsにLocustで追加されたオプションを追加

### Fixed

- Download ReportでStartTimeやRPSが0になる問題を修正


## v1.1.1 - 2024-11-23

### Fixed

- SequentialでInterruptTaskSetを返しても続けてタスクが実行される問題を修正


## v1.1.0 -2024-06-02

### Changed

- ユーザー数が多い場合の統計情報更新のパフォーマンス向上

### Fixed

- ユーザーの Spawn 処理が停止し、ユーザーが増えなくなる場合がある問題を修正


## v1.0.1 - 2024-04-01

### Fixed

- launce.Version の修正


## v1.0.0 - 2024-04-01
