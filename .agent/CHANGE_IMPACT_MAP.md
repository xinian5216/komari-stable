# CHANGE_IMPACT_MAP.md — 改 A 会影响 B

> 用法：**改代码前**先查本表，确认影响面；改完在 PR/提交信息中说明已评估这些影响。
> 表中"影响"= 需要一起检查/测试/更新文档的对象。

## 1. 核心影响链

```text
Database Schema (AutoMigrate / models)
   ↓
Migration（internal/migrations、pkg/metric/migrations）
   ↓
API 响应字段 / RPC 返回结构
   ↓
Metric Store（series/rollup/digest 结构）
   ↓
Backup / Restore（data/backup/upgrade-*.zip、web/backup）
   ↓
Rollback（旧版本二进制能否读新库）

Agent Protocol (protocol/v2)
   ↓
Agent 兼容性（1.2.x~1.5.x 旧 Agent）
   ↓
WebSocket / POST fallback 传输
   ↓
Authentication（token 鉴权、RequireRole）
   ↓
Telemetry（上报数据 → 指标写入）

配置项（internal/config/settings.go）
   ↓
默认值变化 = 用户可见行为变化
   ↓
configs 键值表数据 / 前端 /api/public 输出
   ↓
CHANGELOG 的 Compatibility 段

指标写入路径（web/api/client → internal/metricstore → pkg/metric）
   ↓
实时状态缓存（/api/clients WS、getNodesLatestStatus）
   ↓
历史查询（getRecords / getNodeRecentStatus）
   ↓
通知（流量/负载/离线告警阈值判定）
   ↓
前端图表（默认主题 & 第三方主题）

认证/会话（web/api/Auth.go、accounts/sessions）
   ↓
终端会话归属校验（terminal 会话 id 与 client_uuid 绑定）
   ↓
文件传输令牌校验
   ↓
管理端所有 /api/admin/* 与 RPC admin: 方法
```

## 2. 快速对照表

| 修改对象 | 必须一起检查 |
| --- | --- |
| `database/models/*` | `dbcore.doInitialize` 的 AutoMigrate 列表、`internal/migrations`、API DTO、前端展示 |
| `internal/migrations/*` | 旧库升级演练、版本标记、`CHANGELOG` Compatibility/Rollback |
| `pkg/metric/*` | 指标迁移、维护作业、查询接口、大库性能、回滚可行性 |
| `protocol/v2/*` | Agent 兼容、`web/api/client/*`、文档 `AGENT_PROTOCOL.md`、`komari-protocol` 对照 |
| Agent capability / 远控门禁 | `agent_runtime` 内存态、`admin:getNodes`、任务下发、终端/文件 RPC、`komari-web-stable` 三态 UI |
| `web/router/router.go` | API.md、权限矩阵（RequireRole）、前端调用点 |
| `web/rpc/jsonrpc/*` | RPC2 方法名（冻结）、`rpc.methods` 输出、前端调用、插件 `server.call` 可用范围 |
| `web/api/terminal/*` | 会话所有权校验、Origin 校验、2FA 流程、Agent 侧 `agent.terminal.request` |
| `web/filemanager/*`、`web/upload/*` | 路径规范化、令牌归属、大小限制、预览下载令牌 |
| `internal/plugin/*`、`pkg/jsruntime/*` | 权限清单、市场下载（SSRF）、插件 API 稳定性（插件生态依赖） |
| `internal/config/settings.go` | 默认值变更影响（用户可见）、`/api/public` 输出、前端设置页 |
| `web/security/*` | CORS/Origin 行为、API Key 例外规则、前端跨域部署场景 |
| `utils/notifier/*`、`utils/messageSender/*` | 通知模板变量、渠道凭据字段、告警判定阈值 |
| `utils/version.go` / 构建 ldflags | `/api/version`、升级备份触发（版本标记变化会触发备份！） |
| `.github/workflows/*` | 发布资产命名（与 install-komari.sh 下载名对应）、Docker tag 策略 |
| `install-komari.sh` | 下载文件名/路径、REPO_OWNER/REPO_NAME/RELEASE_BASE 参数 |
| `.agent/*` | 本索引自身的更新规则（见 MAINTENANCE_RULES §8） |

## 3. 特别注意：会触发"自动备份"的改动

`utils.CurrentVersion` 注入值变化 → 用户升级后启动即触发 `backupOnVersionUpgrade()`
（打包整个 `data/`）。**不要**把该机制当成"版本号变化无所谓"：备份在磁盘上会产生完整副本，
大库用户的磁盘占用需要考虑；同时**不得**在未确认情况下修改其触发条件。
