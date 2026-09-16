# Changelog

本文件记录 Komari Stable 的每个版本。格式约定：

- **Fixed**：修复了什么（含 issue 引用）。
- **Security**：安全修复（含 CVE/GHSA 编号）。
- **Compatibility**：对数据库 / Agent / Docker / API / 配置的影响（无影响也要写明）。
- **Upgrade**：升级方法。
- **Rollback**：回滚方法。

版本规则：`<上游版本>-stable.<序号>`，序号从 0 递增；上游版本基线为 `1.5.0`（基于 tag `1.5.0-fix1`）。

---

## 1.5.0-stable.0 — 基线版本（未发布）

**基线**：上游 `komari-monitor/komari` tag `1.5.0-fix1`，commit `0ca87aafd184ed75f9030ede0902772142af5eec`
（上游于 2026-09-15 归档）。

本版本**不含任何业务代码改动**：与上游 `1.5.0-fix1` 的差异仅为维护文档与 CI 草案。

### Fixed

- 无（与上游基线一致）。

### Security

- 无新增修复。基线含上游历史安全修复（见 `SECURITY.md`）。
- 已知未处理项（计划在后续 `stable.N` 中按最小升级原则处理）：
  - `GO-2026-5970` `golang.org/x/text@v0.33.0`（可达）→ 修复于 `v0.39.0`；
  - `GO-2026-5026` `golang.org/x/net@v0.41.0`（可达）→ 修复于 `v0.55.0`；
  - 另有 9 个导入包级 + 22 个模块级不可达漏洞（明细见 `BASELINE_AUDIT.md`）。

### Compatibility

- 数据库：无变化（不修改 Schema / 迁移）。
- Agent：无变化（v2 协议保持冻结）。
- Docker：无变化（沿用上游 `Dockerfile` 与 `25774` 端口约定）。
- API / 配置：无变化。

### Upgrade

- 从上游 `1.5.0` / `1.5.0-fix1` 升级：直接替换二进制或镜像即可，无需数据迁移。
- 服务端会在检测到版本变化时自动把 `data/` 备份为 `data/backup/upgrade-<时间戳>.zip`。

### Rollback

- 换回上游 `1.5.0-fix1` 二进制/镜像；如已由本版本触发自动备份，
  可用 `data/backup/upgrade-*.zip` 恢复 `data/` 目录后再回滚。

---

## Unreleased

（待办：`1.5.0-stable.1` 计划处理 govulncheck 可达漏洞的最小依赖升级。）
