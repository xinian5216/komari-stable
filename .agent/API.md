# API.md — 接口地图（HTTP + RPC2）

> 兼容性规则：监控与管理 API 默认冻结。因 Issue #2 的安全决策，远程命令、终端和远程文件接口已移除；
> 旧 HTTP 路径固定返回 `410 Gone`，旧 RPC 方法不再注册。
> 详细字段说明见官方文档 `komari-monitor/komari-document`（`dev/api.md`、`dev/rpc.md`）。

## 1. 路由总表（代码：`web/router/router.go`）

### 公开（无需登录）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/ping` | 健康检查（返回 `pong`） |
| POST | `/api/login` | 登录（可能返回需 2FA 的下一步） |
| GET | `/api/logout`、`/api/oauth`、`/api/oauth_callback` | 登出 / OAuth 流程 |
| GET | `/api/me`、`/api/nodes`、`/api/public`、`/api/version` | 公开信息（RPC2 绑定） |
| GET | `/api/recent/:uuid`、`/api/records/{load,ping}`、`/api/task/ping` | 记录查询（RPC2 绑定） |
| GET | `/api/plugin/:short/*filepath` | 插件公开页面（iframeless） |
| GET/HEAD | `/api/preview/client/:uuid/file/download` | 已移除（`410 Gone`） |
| GET | `/api/clients`（WebSocket） | 前端实时数据流（发 `get` / `get <uuid>`） |
| GET/POST | `/api/rpc2` | JSON-RPC 2.0 直连入口 |

### Agent（`RequireRole(admin, client)`，token 鉴权）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/clients/register` | 自动发现注册（`Authorization: Bearer <AutoDiscoveryKey>`） |
| GET/POST | `/api/clients/v2/rpc` | **v2 主通道**（WS / POST 回退） |
| GET/POST | `/api/clients/transfer/:id` | 已移除（`410 Gone`） |
| GET | `/api/clients/terminal` | 已移除（`410 Gone`） |

### 管理（`RequireRole(admin)`，`/api/admin/*`）

- 二进制/流类保留 REST handler：`/download/backup`、`/upload/{init,chunk,merge,cancel}`、
  `/update/{mmdb,user,favicon}` 等；
- 其余 JSON 接口统一经 `jsonRpc.Bind("admin:xxx")` 桥接到 RPC2（见 `web/rpc/jsonrpc/admin.*.go`）。

## 2. RPC2 命名空间（代码：`web/rpc/jsonrpc/register.go` 等）

| 命名空间 | 用途 | 代表方法 |
| --- | --- | --- |
| `rpc.` | 内省 | `rpc.methods`、`rpc.help`、`rpc.ping`、`rpc.version` |
| `common:` | 登录后通用 | `getNodes`、`getNodesLatestStatus`、`getRecords`、`getNodeRecentStatus`、`getMe`、`getPublicInfo` |
| `public:` | 访客 | `getMe`、`getNodesInformation`、`getPublicSettings`、`getVersion`、`getClientRecentRecords`、`getRecordsByUUID`、`getPingRecords`、`getPublicPingTasks` |
| `admin:` | 管理员 | 节点/设置/通知/插件/主题/备份/数据库维护/指标管理等；远程命令、终端、任务和远程文件方法不再注册 |

绑定与传输：`transport.go`（HTTP 解析/鉴权/错误包装）、`jsonrpc/bridge.go`（REST↔RPC 桥）、
`dispatch.go`（方法分发）、`principal.go`（调用方身份）。

## 3. 鉴权方式（三种）

| 方式 | 说明 | 代码 |
| --- | --- | --- |
| 会话 Cookie `session_token` | 浏览器登录（`HttpOnly` + `SameSite=Lax`） | `web/api/public/login.go`、`database/accounts/sessions.go` |
| API Key | `Authorization: Bearer <api_key>`（≥12 位才生效） | `web/security/origin.go:IsAPIKeyRequest`、`web/api/Auth.go` |
| Agent Token | 每节点独立 token（URL `?token=`） | `web/api/client/*`、`database/clients/*` |

## 4. 修改接口时的检查清单

1. 只做**向后兼容**的增量（新字段可选、旧字段保留）；安全决策移除的远控接口不得恢复；
2. 同步更新 `.agent/API.md`、`CHANGELOG.md` 的 Compatibility 段；
3. 公开接口（`public:`/`common:`）尤其注意**信息泄露**（token、ip、remark、version 一律不下发）；
4. 涉及写操作时检查是否受 CORS/Origin 与角色校验覆盖。
