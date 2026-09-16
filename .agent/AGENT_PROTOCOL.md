# AGENT_PROTOCOL.md — Server ↔ Agent 协议（v2）

> 结论先行：**1.5.0 起只支持 v2**（v1 上报接口已删除）。协议**冻结**，服务端改动必须保持对旧 Agent 的兼容。
> 协议冻结对照：上游 `komari-monitor/komari-protocol`（含 freeze tests）。

## 1. 连接流程

```text
Agent 启动
 1) POST  /api/clients/v2/rpc   method=agent.basicInfo     （上报静态信息，之后默认每 5 分钟）
 2) GET   /api/clients/v2/rpc?token=…  → WebSocket 升级     （首选长连接，双向）
       ├─ WS 可用：按 interval（默认 1s）发 agent.report，并接收服务器下发事件
       └─ WS 失败：按 --max-retries / --reconnect-interval 重试 → 进入 POST fallback
 3) POST fallback：POST /api/clients/v2/rpc（JSON-RPC，可 gzip），
       服务器可在响应 result.events[] 中夹带事件；Agent 用 agent.pull 长轮询拉取
 4) 恢复：fallback 期间持续尝试 WS，成功即切回
```

## 2. 传输与鉴权

| 项 | 值 |
| --- | --- |
| 端点 | `GET|POST /api/clients/v2/rpc?token=<节点 token>` |
| 协议 | JSON-RPC 2.0（`jsonrpc: "2.0"`，`id` 为 null 时是 notification） |
| 鉴权 | URL `token` → `RequireRole(admin, client)`；token 存 `clients.token` |
| 压缩 | v2 默认允许 gzip（`Content-Encoding: gzip`） |
| 自动发现 | `POST /api/clients/register`（`Authorization: Bearer <AutoDiscoveryKey>`，密钥 <12 位拒绝） |

## 3. 方法（代码：`protocol/v2/jsonrpc.go`）

| 方向 | 方法 | 说明 |
| --- | --- | --- |
| Agent→Server | `agent.report` | 实时监控数据（cpu/ram/swap/load/disk/network/connections/gpu/uptime/process） |
| Agent→Server | `agent.basicInfo` | 静态信息（os/kernel/arch/cpu/内存/磁盘/GPU/ipv4/ipv6/version） |
| Agent→Server | `agent.pingResult` | ping 探测结果 |
| Agent→Server | `agent.taskResult` | 远程执行结果（含 exit_code / finished_at RFC3339） |
| Agent→Server | `agent.event` / `agent.pull` | 事件上报 / POST fallback 长轮询拉取 |
| Agent→Server | `agent.file` / `agent.file.result` | 文件管理相关 |
| Server→Agent | `agent.exec` / `agent.ping` / `agent.message` / `agent.event` | 下发消息（message 字段区分类型） |
| Server→Agent | `agent.terminal.request` | 请求 Agent 建立独立终端 WS（`/api/clients/terminal?id=`） |

事件结构（`protocol/v2/jsonrpc.go:Event`）：`id / method / params / created_at / expires_at`；
Agent 通过下次 `agent.report` 的 `ack_event_ids` 确认；服务端保证 at-least-once，Agent 需按 id 幂等。

## 4. 其它通道

| 通道 | 端点 | 说明 |
| --- | --- | --- |
| 终端流量 | `GET /api/clients/terminal?id=<session>`（WS） | 会话 ID 由 `agent.terminal.request` 协商；代码 `web/api/terminal/*` |
| 文件传输 | `GET|POST /api/clients/transfer/:id` | 短时令牌 + 原始流（`web/filemanager/transfer.go`） |
| 前端实时 | `GET /api/clients`（WS） | 面向浏览器的只读数据流 |

## 5. 版本兼容要求（改代码前必读）

1. **不得**修改 `protocol/v2` 中的方法与字段名；新增字段必须可选、旧 Agent 可忽略。
2. 服务端必须同时接受 WS 与 POST 两种传输，且 POST 响应里的 `result.events[]` 语义不变。
3. `agent.report` 缺失/多出字段要能容错（旧 Agent 不发送 GPU 等新字段）。
4. Server 版本与 Agent 版本**解耦**：Agent 独立发版（上游 `komari-agent`，含自动更新）。
5. 涉及协议改动的 PR：同步更新本文件 + `CHANGELOG.md` 的 Compatibility 段。
