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
- 若涉及上游仍然活跃的组件（`komari-agent` / `komari-web` / `komari-protocol`），我们会在修复的同时
  向上游同步（若上游仍然接受报告）。

本分支为社区志愿维护，**不承诺 SLA**；但安全类问题会被优先处理，修复会以 `security:` 前缀提交并
在 `CHANGELOG.md` 的 `Security` 段中说明。

## 范围（Scope）

在范围内：

- 服务端（本仓库）中的认证/鉴权绕过、注入（SQL/命令/路径）、SSRF、XSS、CSRF/CSWSH、权限提升、
  文件读写越界、上传/下载校验缺陷；
- Web 终端（WebSocket 会话）与文件管理器、插件宿主（JS 运行时与权限清单）的安全问题；
- 指标数据库（SQLite/MySQL/PostgreSQL）访问与 DSN 处理相关问题；
- 依赖中的**可达**漏洞（`govulncheck` 能给出调用路径的）。

通常不在范围内：

- 需要攻击者已拥有服务器 root/管理员凭据的场景；
- 与上游 Agent 二进制本身相关的问题（应报告到 `komari-agent`）；
- 纯配置错误（如把面板直接暴露在公网且关闭鉴权、未启用 HTTPS 等）；
- 依赖中**不可达**的漏洞（会记录并随安全升级处理，不单独算安全问题）。

## 已知安全历史（供运维参考）

| 公告 | 主题 | 影响版本 | 修复版本 |
| --- | --- | --- | --- |
| GHSA-hxjg-93wc-h8p8 | 管理接口 CSRF（cookie 未设 SameSite/Secure；管理端无 CSRF token） | ≤ 1.2.0 | 1.2.2 |
| GHSA-q355-h244-969h | 终端 WebSocket 跨站劫持（CSWSH） | < 1.0.4 | 1.0.4-fix1 |
| GHSA-jhmr-57cj-q6g9 | 2FA 校验逻辑错误（任意验证码通过） | < 1.0.4 | 1.0.4-fix1 |

当前基线（`1.5.0-fix1`）已包含上述修复；登录 Cookie 使用 `HttpOnly + SameSite=Lax`，
CORS 与 WebSocket Origin 校验默认开启。

## 运维加固建议

1. **始终使用 HTTPS/WSS**（反向代理），否则会话 Cookie 与终端流量可被中间人截获。
2. 保持 `CORS Origin 校验` 与 `WebSocket Origin 校验` 开启；仅在确有需要时使用允许列表。
3. 按需开启 **SSRF 防护**（`ssrf_protection_enabled`，默认关闭；影响主题/插件市场与远程导入）。
4. 为管理员账号启用 **2FA**；不要把面板直接裸露在公网，建议加访问控制（VPN / IP 白名单 / Basic Auth 前置）。
5. 定期备份 `data/` 目录（含 `komari.db` 与 `metrics.db`），并在升级后确认
   `data/backup/upgrade-*.zip` 已生成。
6. 关注本仓库的 Release 与 `CHANGELOG.md` 的 `Security` 段。
