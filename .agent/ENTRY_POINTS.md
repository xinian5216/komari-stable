# ENTRY_POINTS.md — 主要入口一览

> 找入口先看这里，不要全仓库搜索。

## 进程与 CLI

| 入口 | 位置 | 说明 |
| --- | --- | --- |
| 程序主入口 | `main.go` | 打印版本 → `cmd.Execute()` |
| CLI 根命令 | `cmd/root.go` | 全局 flag：`-d/--database`、`-t/--db-type`；`-l/--listen` 在 `cmd/server.go` 的 server 子命令 |
| 服务端启动 | `cmd/server.go` → `RunServer()` | 启动生命周期编排（见 ARCHITECTURE.md） |
| 改管理员密码 | `cmd/chpasswd.go` | 离线工具 |
| 关闭 2FA | `cmd/disable2FA.go` | 离线工具 |
| 允许密码登录 | `cmd/permitPasswordLogin.go` | 离线工具 |

## 服务端生命周期

| 阶段 | 位置 |
| --- | --- |
| 应用对象 / 阶段编排 | `internal/server/app.go` |
| 启动引导（目录、主库、settings） | `internal/server/bootstrap.go` |
| 主库迁移判定与执行 | `internal/server/app.go`（`DatabaseMigrationRequired` / `RunDatabaseMigration`） |
| 指标库连接与恢复 | `internal/server/metric_store.go`、`web/recovery/recovery.go` |
| 后台任务注册 | `internal/server/runtime.go` |
| 路由构建 | `internal/server/app.go`（`BuildRouter`）→ `web/router/router.go` |

## HTTP / WebSocket

| 入口 | 位置 |
| --- | --- |
| 路由注册总入口 | `web/router/router.go`（`Register`：public / agent / admin 三组） |
| RPC2 直连端点 | `POST|GET /api/rpc2` → `web/rpc/jsonrpc/transport.go` |
| 方法注册表 | `web/rpc/jsonrpc/register.go` |
| 前端实时数据 WS | `GET /api/clients` → `web/api/ws.go` / `WebSocket.go` |
| Agent v2 RPC | `GET|POST /api/clients/v2/rpc` → `web/api/client/report_v2.go` |
| 已移除的远控入口 | 终端、远程文件、任务相关旧路径在 `web/router/router.go` 统一返回 `410 Gone` |
| 自动发现注册 | `POST /api/clients/register` → `web/api/client/autoDiscovery.go` |
| 安装向导 | `web/install/install.go`（`/install`、`/api/install/*`） |
| 指标库恢复向导 | `web/recovery/recovery.go`（`/database-recovery`） |
| 指标库迁移向导 | `web/migration/*` |

## 数据与存储初始化

| 入口 | 位置 |
| --- | --- |
| 主库初始化 | `database/dbcore.Initialize()` → `doInitialize()`（AutoMigrate 列表在此） |
| 一次性历史迁移 | `internal/migrations.Run()` |
| 指标库初始化 | `internal/metricstore/store.go`、`pkg/metric/store.go` |
| 指标库 schema 迁移 | `pkg/metric/migrations.go`、`internal/metricstore/store_migration.go` |

## 插件 / 调度 / 其它

| 入口 | 位置 |
| --- | --- |
| 插件管理器 | `internal/plugin/plugin.go`（Manager 生命周期） |
| 插件宿主路由/钩子 | `internal/plugin/server.go`、`hook.go`、`inject.go` |
| 定时调度器 | `internal/scheduler/scheduler.go` |
| 重启（UI 触发） | `internal/lifecycle/restart.go` |
| 安装脚本（裸机） | `install-komari.sh`（参数化：`REPO_OWNER`/`REPO_NAME`/`RELEASE_BASE`） |
| Docker 启动 | `Dockerfile`（`CMD ["/app/komari","server"]`） |

## CI 入口

| 入口 | 位置 |
| --- | --- |
| 稳定分支 CI | `.github/workflows/stable-ci.yml` |
| 发布 | `.github/workflows/stable-release.yml` |
| 密钥扫描 | `.github/workflows/secret-scan.yml` |
| 其它现有 workflow | `.github/workflows/`（以目录中的文件和各自触发条件为准） |
