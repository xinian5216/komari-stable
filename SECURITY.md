# Security Policy — Komari Stable

## 支持的版本

| 版本 | 支持状态 |
| --- | --- |
| `1.5.0-stable.x` | ✅ 当前维护线 |
| 上游 `1.5.0` / `1.5.0-fix1` | ⚠️ 上游已归档（2026-09-15），不再提供修复 |
| 旧于 `1.5.0` 的官方版本 | ❌ 不再支持（请先升级到 `1.5.0-stable.x`） |

## 报告漏洞

- 首选：在本仓库 **Security → Report a vulnerability** 里提交 **私有安全公告（Private Security Advisory）**，
  并附上：影响版本、复现步骤或 PoC、影响评估、可能的修复方向。
- 请**不要**先公开披露 issue。我们会在确认与修复完成后协调公开时间。
- 组件归属：服务端（本仓库）、前端（`xinian5216/komari-web-stable`）、Agent（`xinian5216/komari-agent-stable`）
  均由本 fork 维护，可直接在对应仓库报告；`komari-protocol` 仍作为上游协议冻结的对照基准。

完整的分级、私密修复、发布与演练流程见 [SECURITY_RESPONSE.md](./SECURITY_RESPONSE.md)。

本分支为社区志愿维护，**不承诺 SLA**；但安全类问题会被优先处理，修复会以 `security:` 前缀提交并
在 `CHANGELOG.md` 的 `Security` 段中说明。

## 范围（Scope）

在范围内：

- 服务端（本仓库）中的认证/鉴权绕过、注入（SQL/命令/路径）、SSRF、XSS、CSRF/CSWSH、权限提升、
  文件读写越界、上传/下载校验缺陷；
- 已移除远程命令/终端/远程文件边界的绕过或重新暴露，以及插件宿主（JS 运行时与权限清单）的安全问题；
- 指标数据库（SQLite/MySQL/PostgreSQL）访问与 DSN 处理相关问题；
- 依赖中的**可达**漏洞（`govulncheck` 能给出调用路径的）。

通常不在范围内：

- 需要攻击者已拥有服务器 root/管理员凭据的场景；
- Agent 二进制本身的问题：报告到 `xinian5216/komari-agent-stable`（本 fork 维护的镜像仓库，源自上游）；
- 纯配置错误（如把面板直接暴露在公网且关闭鉴权、未启用 HTTPS 等）；
- 依赖中**不可达**的漏洞（会记录并随安全升级处理，不单独算安全问题）。

## 自动安全检查

- 每次相关 PR / `stable` 推送运行 CodeQL 与固定版本的 `govulncheck`；
- 每日定时重扫，以捕获代码未变化但漏洞数据库新增的公告；
- 结果进入 GitHub Security / Code scanning，漏洞详情不会自动写入公开 Issue；
- Dependabot 常规更新只分组 minor/patch；安全告警单独评估可达性，不做无边界大升级。

## 已知安全历史（供运维参考）

| 公告 | 主题 | 影响版本 | 修复版本 |
| --- | --- | --- | --- |
| GHSA-hxjg-93wc-h8p8 | 管理接口 CSRF（cookie 未设 SameSite/Secure；管理端无 CSRF token） | ≤ 1.2.0 | 1.2.2 |
| GHSA-q355-h244-969h | 终端 WebSocket 跨站劫持（CSWSH） | < 1.0.4 | 1.0.4-fix1 |
| GHSA-jhmr-57cj-q6g9 | 2FA 校验逻辑错误（任意验证码通过） | < 1.0.4 | 1.0.4-fix1 |

当前基线（`1.5.0-fix1`）已包含上述修复；登录 Cookie 使用 `HttpOnly + SameSite=Lax`，
CORS 与 WebSocket Origin 校验默认开启。

## 运维加固建议

1. **始终使用 HTTPS/WSS**（反向代理），否则会话 Cookie 与 Agent 上报流量可被中间人截获。
2. 保持 `CORS Origin 校验` 与 `WebSocket Origin 校验` 开启；仅在确有需要时使用允许列表。
3. 按需开启 **SSRF 防护**（`ssrf_protection_enabled`，默认关闭；影响主题/插件市场与远程导入）。
4. 为管理员账号启用 **2FA**；不要把面板直接裸露在公网，建议加访问控制（VPN / IP 白名单 / Basic Auth 前置）。
5. 定期备份 `data/` 目录（含 `komari.db` 与 `metrics.db`），并在升级后确认
   `data/backup/upgrade-*.zip` 已生成。
6. **登录限流**：`/api/login` 对过量请求返回 `429 Too Many Requests`（含 `Retry-After`）。
   限流按两个维度独立计数：单个来源（burst 10、持续 30 次/分钟）与单个账号（连续失败
   5 次后进入最短 5 秒、封顶 60 秒的退避冷却，无永久锁定，15 分钟无失败自动恢复）。
   完整登录成功（密码 + 2FA + 会话）后账号失败计数清零。来源维度按 **TCP 直连对端**
   聚合以防止伪造 `X-Forwarded-For` 绕过——因此同一反向代理 / NAT 后的用户共享一个
   来源桶；集中登录的团队偶发 `429` 时按 `Retry-After` 重试即可（见 `TECH_DEBT.md` 条目 5/6）。
7. 关注本仓库的 Release 与 `CHANGELOG.md` 的 `Security` 段。
