# ARCHITECTURE.md — 架构速览

## 分层

```text
┌──────────────────────────── web/ (Gin HTTP 层) ─────────────────────────────┐
│ router/ 路由装配 → api/（登录/OAuth/上传/安装/恢复）                         │
│ rpc/jsonrpc/  RPC2 方法（admin: / common: / public: 三个命名空间）            │
│ upload/、public/（嵌入主题与静态资源）；远控旧路径由 router 返回 410          │
└──────────────────────────────┬──────────────────────────────────────────────┘
                               │
┌────────────────────────── internal/（业务与生命周期） ──────────────────────┐
│ server/ 启动阶段编排、路由构建、后台任务注册                                  │
│ config/ 配置项（存 DB 的 configs 键值表） migrations/ 一次性迁移              │
│ metricstore/ 指标库读写门面  plugin/ 插件宿主(goja)  scheduler/ 定时任务     │
└──────────────────────────────┬──────────────────────────────────────────────┘
                               │
┌────────────── database/（GORM，SQLite 主库） │ pkg/metric（指标存储引擎）────┐
└──────────────────────────────────────────────┴──────────────────────────────┘
```

## 启动生命周期（`cmd/server.go` → `internal/server/`）

```text
main.go
 → cmd.Execute()
 → cmd/server.go RunServer()
   → app.Bootstrap()                # 建 data/ 目录、初始化主库、读 settings
   → app.InstallRequired()          # 首次运行 → web/install 安装向导（阻塞）
   → [循环] DatabaseMigrationRequired() / RunDatabaseMigration()   # 主库迁移，失败可进恢复向导
   → app.ConnectMetricStoreWithRetry() → 失败进 web/recovery 恢复向导
   → InitStores → InitProviders → StartBackground → BuildRouter
   → app.Run()                      # 监听 HTTP
```

## 三条主要请求路径

1. **控制台用户**：`GET /` → 嵌入主题（`web/public`）→ 前端 SPA；数据走 `/api/*` 与 `/api/rpc2`。
2. **Agent 上报**：`/api/clients/v2/rpc`（WS 优先 / POST 回退）→ `web/api/client/*` → 状态缓存 + 指标写库；
   服务器下发事件走 WS 或 POST 响应 `result.events[]`。
3. **管理操作**：`/api/admin/*`（RequireRole(admin)）→ `web/rpc/jsonrpc/admin.*` → DB/系统操作。

## 数据流（监控指标）

```text
Agent 上报 → web/api/client (ingest/report_v2) → internal/metricstore
   ├─ 实时状态缓存（内存，供 /api/clients WS、getNodesLatestStatus）
   └─ pkg/metric 管道：raw_points → rollup(热/冷/归一并) → digest 回收 → retention/compaction
```

## 插件与主题

- **插件**：`internal/plugin` 用 `pkg/jsruntime`（goja）在宿主内运行 JS，按 manifest 权限清单
  （allowRoutes/allowHooks/allowHTMLInject/allowSystemRPC）开放 route/hook/injectHTML/call；
  市场下载在 `web/api/admin/plugin_market.go`（受 SSRF 开关约束）。
- **主题**：`web/public` 服务嵌入的默认主题（`dist.tar.zst` 内存解压）；第三方主题放
  `data/theme/<short>/`，受管配置见 `internal/managedconfig`。

## 部署形态

- 单二进制（静态自足，内嵌前端）+ SQLite；可选把**指标库**放到 MySQL/PostgreSQL。
- Docker：`Dockerfile`（alpine + 预编译二进制 + `data` 卷），端口 `25774`。
- 版本注入：`utils/version.go` 的 `CurrentVersion`/`VersionHash` 由构建期 `-ldflags` 写入。
