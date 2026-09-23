# DATABASE.md — 数据库地图与保护规则

## 0. 硬规则（先读这一条）

> ⛔ **禁止 Coding Agent 未经维护者确认直接修改数据库 Schema、迁移逻辑或用户数据结构。**
> 任何触碰 `AutoMigrate` 列表、`internal/migrations/*`、`pkg/metric/migrations.go`、
> `internal/metricstore/store_migration.go` 的改动，必须先输出：
> 为什么改 / 影响哪些数据 / 是否可能丢数据 / 是否可回滚 / 如何备份，并等待确认。

## 1. 两个数据库（相互独立）

| 库 | 类型 | 默认位置 | 配置方式 | 代码 |
| --- | --- | --- | --- | --- |
| **主库** | **仅 SQLite** | `./data/komari.db`（WAL） | `-d/--database` 参数；`cmd/flags/config.go` 只认 sqlite | `database/dbcore/*` |
| **指标库** | SQLite / MySQL / PostgreSQL | `./data/metrics.db` | `metric_db_driver` + `metric_db_dsn`（可运行时切换并迁移） | `internal/metricstore/config.go`、`pkg/metric/*` |

DSN 细节：主库 DSN 由 `dbcore.buildSQLiteDSN()` 组装（`_busy_timeout`、`_txlock=immediate`）；
指标库 DSN 推断逻辑在 `internal/metricstore/config.go`（`ResolveDriverFromConfig` / `InferDriverFromDSN`）。

## 2. Schema 定义位置

- 表结构（GORM 模型）：`database/models/*.go`（`Client`、`User`、`Task`、`Theme`、`PingTask`、
  `Clipboard`、`LoadNotification`、`OfflineNotification`、`OidcProvider`、`MessageSenderProvider`…）
- 建表/字段演进：`database/dbcore/dbcore.go` → `doInitialize()` 的 `AutoMigrate(...)` 列表
  （GORM 只加不删：**不删列、不删表**）
- 配置项：`internal/config/settings.go` 定义，数据存 `configs` 表（当前为 `key/value` 键值结构；
  `ConfigItem.TableName()` 明确返回 `configs`）

## 3. 迁移机制（三层）

| 层 | 位置 | 说明 |
| --- | --- | --- |
| ① GORM AutoMigrate | `database/dbcore/dbcore.go` | 启动时补列/建表 |
| ② 一次性历史迁移 | `internal/migrations/*` | 处理 0.x/1.0.x/1.1.x → 现结构（时间戳 UTC 化、`configs` 从旧单行模型重建为同名 `key/value` 表、旧 ping 任务展开等）；`migrations.go:Run()` 是总入口 |
| ③ 指标库迁移 | `pkg/metric/migrations.go`、`internal/metricstore/store_migration.go` | 指标表结构、rollup/digest 结构升级；另有跨库搬迁（`internal/metricstore/migration_store.go` + `web/migration`） |

## 4. 升级 / 备份 / 回滚（现有安全网，勿破坏）

- `dbcore.backupOnVersionUpgrade()`：启动时若 `system_version` 与当前构建版本不一致，
  先 `PRAGMA wal_checkpoint(TRUNCATE)`，再把整个 `./data`（排除备份自身）打包为
  `data/backup/upgrade-<UTC时间戳>.zip`，然后写入新版本标记。
- 全新安装（无 DB 文件且无版本标记）只写标记、不备份。
- 手工备份：管理端下载备份（`/api/admin/download/backup`）、上传恢复（安装向导 / 归档上传）。
- 指标库恢复向导：`web/recovery`（连接失败时进入，DSN 输出已脱敏 `internal/metricstore/redact.go`）。

## 5. 高风险表 / 数据（改动需加倍谨慎）

| 对象 | 风险 |
| --- | --- |
| `clients`（节点） | 含 `token`（Agent 凭据）；字段变化影响鉴权与上报 |
| `users` / `sessions` | 账号、密码哈希、2FA 密钥、会话有效性 |
| `configs`（键值表） | 站点全部配置（含 API Key、AutoDiscovery Key、通知密钥） |
| 指标库 `series/labels/resolutions/rollups/raw_points` | 体量大、参与聚合与摘要回收；错误迁移会污染历史 |
| 旧记录表（`records`/`ping_records`/`gpu_records`） | 仅作为旧库导入 DTO 保留（见 `internal/metricstore/legacy_records.go`） |

## 6. 已知约定

- 时间统一 UTC（`internal/migrations/timestamp.go` 负责旧数据转换）。
- `1.5.0-fix1` 起不再创建 `TrafficReportNotification` 表（**不删**已有表）。
- 指标库"维护"动作（VACUUM / 清理）在 `internal/metricstore/maintenance.go` 与
  `pkg/metric/maintenance.go`，属重 IO 操作，改动需评估大库表现。
