# Changelog

本文件记录 Komari Stable 的每个版本。格式约定：

- **Fixed**：修复了什么（含 issue 引用）。
- **Security**：安全修复（含 CVE/GHSA 编号）。
- **Compatibility**：对数据库 / Agent / Docker / API / 配置的影响（无影响也要写明）。
- **Upgrade**：升级方法。
- **Rollback**：回滚方法。

版本规则：`<上游版本>-stable.<序号>`，序号从 0 递增；上游版本基线为 `1.5.0`（基于 tag `1.5.0-fix1`）。

---

## 1.5.0-stable.0 — 首个稳定基线（候选，未发布）

**基线**：上游 `komari-monitor/komari` tag `1.5.0-fix1`，commit `0ca87aafd184ed75f9030ede0902772142af5eec`
（上游于 2026-09-15 归档）。

相对上游基线的实质改动只有三类：两个 `pkg/jsruntime` 缺陷修复、两个可达依赖漏洞的最小升级、
以及维护文档与 CI。数据库、Agent 协议、API、配置格式与 Docker 部署方式均未改动。

### Fixed

- **`pkg/jsruntime/fs`：编码字符串被当成文件权限位**（`fs.writeFileSync(file, data, "utf8")`）。
  Node 允许在 options 位置传编码字符串，而 fs shim 会把它按八进制解析成 mode；解析失败时又退化为
  数值 0，导致文件在 Unix 上以 `0000` 创建（随后的读取直接 `EACCES`，这正是 Linux 上
  `TestNodeCoreModulesAndECMAScriptBuiltins` 与 `TestStorageDirIsConfinedAdditionalRoot` 失败的原因），
  在 Windows 上则被创建为只读文件。现在无法解析的字符串会退回默认权限。
  影响面：调用 `fs.writeFileSync` / `fs.appendFileSync` / `fs.openSync` 且传编码字符串的插件。
- **`pkg/jsruntime/child_process`：回收子进程时丢失 stdout/stderr**。
  `exec.Cmd.Wait` 会在进程退出时关闭 `StdoutPipe`/`StderrPipe`，Go 文档明确要求必须在读取完成后再调用；
  原实现与 stdout/stderr 泵并发调用 `Wait`，导致子进程已写出的输出被丢弃、流可能永远不触发 EOF。
  在 Linux 上的表现是 `TestChildProcessPermissionAndExecution` 约 8% 概率读到空输出、
  `TestChildProcessStdioAreStreams` 偶发挂起直到 8 秒超时。现在回收协程先等两个泵结束再 `Wait`，
  这也与 Node 的 `close` 语义一致（stdio 关闭后才触发）。
- 新增两个回归测试：`TestNodeFileCreationModeFromEncodingArgument`、
  `TestChildProcessOutputSurvivesProcessExit`。

### Security

- `GO-2026-5970` `golang.org/x/text` 升级 `v0.33.0 → v0.39.0`（该漏洞在本代码库中可达）。
  连带的必要传递升级：`golang.org/x/sync v0.19.0 → v0.21.0`。
- `GO-2026-5026` `golang.org/x/net` 升级 `v0.41.0 → v0.55.0`（可达）。
  连带的必要传递升级：`golang.org/x/crypto v0.39.0 → v0.51.0`、`golang.org/x/sys v0.33.0 → v0.45.0`；
  同时补齐 `go.sum` 中这两个模块在 Windows 构建与 `pkg/jsruntime/crypto` 用到的包所缺少的校验和。
- 升级后 `govulncheck ./...`：**可达漏洞 0 个**、导入包级漏洞 0 个
  （仍有 19 个仅在依赖模块中、本代码不可达的漏洞）。
- 未做依赖大扫除：除上述 5 个 `golang.org/x/*` 模块外，没有任何第三方模块版本变化。

### Compatibility

- 数据库：无变化（不修改 Schema、不新增迁移）。
- Agent：无变化（v2 协议保持冻结；服务端继续兼容官方 Agent）。
- Docker：无变化（沿用上游 `Dockerfile`、端口与数据卷约定）。
- API / 配置文件格式：无变化。
- Go 依赖：`golang.org/x/*` 五个模块升级，均为主版本内兼容升级；对外行为无变化。

### Upgrade

- 从上游 `1.5.0` / `1.5.0-fix1` 升级：直接替换二进制或镜像，无需数据迁移。
- 服务端会在检测到版本变化时自动把 `data/` 备份为 `data/backup/upgrade-<时间戳>.zip`。
- 升级后建议确认：面板可登录、节点数据正常、插件（如使用 JS 插件且传编码字符串写文件）行为正常。

### Rollback

- 换回上游 `1.5.0-fix1` 二进制/镜像即可；本次改动不含数据库迁移，无需回滚数据。
- 如已由新版本触发自动备份，可用 `data/backup/upgrade-*.zip` 恢复 `data/` 目录。

---

## Unreleased

（暂无。后续 `1.5.0-stable.N` 的候选改动会先记录在此处。）
