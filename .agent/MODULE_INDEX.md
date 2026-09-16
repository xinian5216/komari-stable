# MODULE_INDEX.md — 模块 → 核心文件 → 职责

> 调试时**只读相关模块的文件**，不要全仓库搜索。路径相对仓库根。

## 认证与授权

- 相关文件：`web/api/Auth.go`、`web/api/AuthSensitive.go`、`web/api/principal.go`、
  `web/api/public/login.go`、`web/api/public/oauth.go`、`web/oauth/*`、
  `database/accounts/accounts.go`、`database/accounts/sessions.go`、`database/accounts/2fa.go`
- 职责：会话 Cookie、API Key（Bearer）、角色（admin/client/guest）、OAuth、2FA、会话续期与清理。

## 节点与上报

- 相关文件：`web/api/client/register.go(AutoDiscovery)`、`report_v2.go`、`ingest.go`、`presence.go`、
  `uploadBasicInfo.go`、`web/connection/safe_conn.go`、`database/clients/*`、`internal/metricstore/report_*.go`
- 职责：自动发现注册、v2 上报（WS/POST）、在线状态、实时状态缓存。

## 指标存储

- 相关文件：`pkg/metric/*`（store/raw_points/rollup*/digest*/maintenance/restructure/dialect*）、
  `internal/metricstore/*`（config/store/migration_store/definitions/compaction/deletion）
- 职责：写入与查询、滚动聚合、摘要压缩与回收、保留期、库迁移（含跨库搬迁）、维护（VACUUM 等）。

## 配置

- 相关文件：`internal/config/*`（settings.go 为配置项定义）、`internal/managedconfig/*`、
  `database/models/*`、`cmd/flags/config.go`
- 职责：配置项默认值/读写（存 `config_items` 表）、启动参数（`-l` 监听、`-d` 数据库路径）、
  主题受管配置。

## 数据库核心

- 相关文件：`database/dbcore/dbcore.go`（连接、DSN、AutoMigrate 列表、升级备份）、
  `database/dbcore/maintenance.go`、`internal/sqlitetune/*`、`internal/migrations/*`
- 职责：主库初始化、schema 演进、升级前自动备份 `data/backup/upgrade-*.zip`、SQLite 调优、一次性迁移。

## Agent 协议

- 相关文件：`protocol/v2/jsonrpc.go`、`web/api/client/*`、`web/rpc/jsonrpc/transport.go`
- 职责：v2 JSON-RPC 方法常量与结构、WS/POST 传输、事件下发与 ack。

## 终端（Web SSH）

- 相关文件：`web/api/terminal/{terminal,request,establish,forward}.go`、`web/rpc/jsonrpc/admin.xtermjs.go`、
  `web/connection/safe_conn.go`
- 职责：会话创建/重附着、浏览器 ↔ 服务器 ↔ Agent 双向流、终端设置（可含 2FA 流程）。

## 文件管理与传输

- 相关文件：`web/filemanager/{client,transfer,stream_relay}.go`、`web/rpc/jsonrpc/admin.file.go`、
  `web/upload/{handler,chunk}.go`、`web/api/admin/archive_upload.go`
- 职责：远程文件浏览/编辑、分块上传、流式中继（大文件）、备份上传。

## 插件系统

- 相关文件：`internal/plugin/{manager/plugin,server,hook,inject,rpc,cron,install,manifest,state,version,logs,config}.go`、
  `pkg/jsruntime/*`、`web/rpc/jsonrpc/admin.plugin.go`、`web/api/admin/plugin_market.go`
- 职责：插件安装/升级/运行、JS 运行时能力（fs/net/crypto/stream/http…）、路由与钩子注入、市场下载。

## 主题系统

- 相关文件：`web/public/public.go`（嵌入主题 + 静态服务）、`database/models/theme.go`、
  `web/api/admin/theme*.go`、`internal/managedconfig/*`
- 职责：默认主题解压与页面注入（sitename/custom_head 等）、第三方主题管理、主题市场。

## 通知与消息

- 相关文件：`utils/notifier/{load,offline,expire,traffic}.go`、`utils/messageSender/*`（telegram/email/webhook/
  bark/serverchan*/javascript）、`database/notification/*`、`web/rpc/jsonrpc/admin.notification.go`
- 职责：负载/离线/到期通知、消息渠道发送、通知模板。

## 定时任务与后台作业

- 相关文件：`internal/scheduler/scheduler.go`、`internal/server/runtime.go`（注册处）、
  `database/tasks/*`、`internal/metricstore/maintenance.go`
- 职责：cron 调度、ping 任务、清理与维护作业。

## 备份 / 恢复 / 安装向导

- 相关文件：`web/backup/restore.go`、`web/api/admin/download.go`、`web/install/install.go`、
  `web/recovery/recovery.go`、`web/migration/*`
- 职责：备份下载与恢复、首启安装、指标库连接失败恢复、指标库迁移向导。

## 安全基础设施

- 相关文件：`web/security/{cors,origin}.go`、`web/api/admin/market_download.go`（SSRF 策略与内网判定）、
  `database/auditlog/log.go`
- 职责：CORS/WS Origin 校验、SSRF 白/黑名单、访客审计。

## 可观测与工具

- 相关文件：`utils/log/*`、`utils/geoip/*`、`utils/renewal/*`、`utils/item/*`、`pkg/timeutil/*`
- 职责：日志、GeoIP（ip-api/ipinfo/mmdb）、续费与流量计算辅助。
