# Change Log

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
