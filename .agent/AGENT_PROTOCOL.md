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
| Agent→Server | `agent.report` | 实时监控数据；可选上报 `capabilities` / `privilege_level`（Stable.2 增量） |
| Agent→Server | `agent.basicInfo` | 静态信息（os/kernel/arch/cpu/内存/磁盘/GPU/ipv4/ipv6/version） |
| Agent→Server | `agent.pingResult` | ping 探测结果 |
| Agent→Server | `agent.taskResult` | 兼容保留：服务端确认后丢弃，不解析、不入库 |
| Agent→Server | `agent.event` / `agent.pull` | 事件上报 / POST fallback 长轮询拉取 |
| Agent→Server | `agent.file` / `agent.file.result` | 兼容保留：服务端确认后丢弃 |
| Server→Agent | `agent.ping` / `agent.message` / `agent.event` | 允许下发的监控与消息事件 |
| Server→Agent | `agent.exec` / `agent.terminal.request` / `agent.file` | 仅保留协议常量；服务端永不下发 |

事件结构（`protocol/v2/jsonrpc.go:Event`）：`id / method / params / created_at / expires_at`；
Agent 通过下次 `agent.report` 的 `ack_event_ids` 确认；服务端保证 at-least-once，Agent 需按 id 幂等。

## 4. capability 与远程控制移除边界

- capability 仅保存在 `web/agent` 内存态，并通过管理员节点 DTO 暴露；数据库 Schema 不变。
- 新 Agent 在 `agent.report` 中上报实际能力和权限级别；字段可选，旧 Agent 不发送时仍可连接。
- 服务端丢弃 `exec` / `terminal` / `file` capability；无论 Agent 是否上报、是否为旧版本，都不会恢复远控。
- `remote_control_known` DTO 字段与远控方法常量仅为旧前端/旧 Agent 兼容保留，不能作为重新开放入口的依据。
- 旧 Agent 仍可连接、上报监控、执行 ping，并接收普通消息；发送的旧任务/文件结果会收到成功确认后被忽略。

## 5. 其它通道

| 通道 | 端点 | 说明 |
| --- | --- | --- |
| 旧终端流量 | `GET /api/clients/terminal` | 已移除，固定返回 `410 Gone` |
| 旧文件传输 | `GET|POST /api/clients/transfer/:id` | 已移除，固定返回 `410 Gone` |
| 前端实时 | `GET /api/clients`（WS） | 面向浏览器的只读数据流 |

## 6. 版本兼容要求（改代码前必读）

1. **不得**修改 `protocol/v2` 中的方法与字段名；新增字段必须可选、旧 Agent 可忽略。
2. 服务端必须同时接受 WS 与 POST 两种传输，且 POST 响应里的 `result.events[]` 语义不变。
3. `agent.report` 缺失/多出字段要能容错（旧 Agent 不发送 GPU 等新字段）。
4. capability 仍可选；服务端必须过滤远控 capability，并以路由 tombstone、RPC 注销和事件下发拒绝三层门禁阻止恢复。
5. Server 版本与 Agent 版本**解耦**：Agent 由本 fork 的 `xinian5216/komari-agent-stable` 独立发版
   （安装/更新通道同指向该仓库）；上游 `komari-agent` 仅作为择优移植来源。协议仍冻结为本文档描述的 v2。
6. 涉及协议改动的 PR：同步更新本文件 + `CHANGELOG.md` 的 Compatibility 段。
