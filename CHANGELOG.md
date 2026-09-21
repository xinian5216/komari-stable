# Changelog

本文件记录 Komari Stable 的每个版本。格式约定：

- **Fixed**：修复了什么（含 issue 引用）。
- **Security**：安全修复（含 CVE/GHSA 编号）。
- **Compatibility**：对数据库 / Agent / Docker / API / 配置的影响（无影响也要写明）。
- **Upgrade**：升级方法。
- **Rollback**：回滚方法。

版本规则：`<上游版本>-stable.<序号>`，序号从 0 递增；上游版本基线为 `1.5.0`（基于 tag `1.5.0-fix1`）。

---

## Unreleased

### Fixed / Security

- **新增 `/api/login` 登录限流（为后续 Argon2id 迁移提供入口保护）**：未鉴权登录端点现以两个
  独立维度限流——来源维度（TCP 直连对端，burst 10、持续 30 次/分钟）与账号维度（sha256(原始
  用户名) 键控，连续失败 5 次后进入 5s→10s→20s→40s→60s 封顶的短时退避冷却；无永久锁定；
  15 分钟无失败自动恢复）。触发限流返回 `429 Too Many Requests`（可计算时附 `Retry-After`），
  响应不区分触发维度、不泄露用户名是否存在。**账号失败计数仅在完整登录成功（密码 + 2FA +
  会话）后清零**：密码正确但 2FA 缺失/错误同样累计失败，防止持有密码者以错误 2FA 空转未来的
  内存强化 KDF。来源维度**不使用 `c.ClientIP()`**（gin 未配置可信代理，转发头可伪造），
  改用不可伪造的 `RemoteAddr` 直连对端；代价是同一反向代理/NAT 后的用户共享来源桶
  （`SECURITY.md` 运维建议已说明，残余风险见 `TECH_DEBT.md` 条目 5/6）。两个 limiter 均为
  内存有界（10000/20000 条目上限 + TTL 清理 + 容量压力下淘汰过期/最旧条目）、并发安全、
  零新增依赖。行为变化：过量登录请求可能收到 `429`；认证成功条件、错误密码（`401`）、
  2FA 交互流程与密码禁用（`403`）语义均不变。Compatibility：无数据库 / Agent / Docker /
  API 字段影响，无密码格式变化。
- **升级 `golang.org/x/crypto` 至 `v0.52.0`（模块级加固）**：关闭 Dependabot 当前开放的全部 13 条
  x/crypto/ssh 公告（CVE-2026-39827~39835、42508、46595~46598，first_patched_version 均为
  v0.52.0）。本仓库仅导入 blake2b/blake2s/chacha20poly1305/ripemd160/scrypt/sha3 子包
  （`pkg/jsruntime/crypto`），未导入受影响的 ssh 子包，漏洞代码路径不可达，属供应链与纵深加固。
  v0.52.0 的模块依赖（x/net v0.54.0 / x/sys v0.45.0 / x/term v0.43.0 / x/text v0.37.0）均不高于
  当前选择版本，无连带升级。GO-2026-5932（openpgp 子包，上游标注无修复版本）未导入、继续跟踪。
  Compatibility：无数据库 / Agent / Docker / API / 配置影响，无用户可见行为变化。
- **修复一键安装管道模式立即退出**：`curl ... | sudo bash` 从 stdin 执行时不再被误判为
  sourced 脚本，也不会把脚本正文占用的 stdin 当成交互输入。存在控制终端时，语言选择与后续菜单
  统一从 `/dev/tty` 读取；下载到文件执行、无终端参数模式和测试 source 行为保持兼容。
- **移除高风险远程控制面（Issue #2）**：服务端不再提供远程命令、Web 终端、任务下发和 Agent
  远程文件管理。相关实现与 `admin:exec`、任务、终端、远程文件 RPC 已删除；旧 HTTP 路径统一返回
  `410 Gone`，未知旧 RPC 返回 `Method not found`。服务端过滤 Agent 上报的 `exec` / `terminal` /
  `file` capability，并在事件下发层再次拒绝这些方法；CI 新增静态守卫，防止后续误恢复。
- **theme=next 时后台被 Service Worker 显示成公开首页**：fresh install 仍默认 `theme=next`，`/` 仍由
  Komari Next 提供。`/admin`、`/manage` 等核心路径强制使用嵌入式 default frontend，
  并从 HTML 中去掉根作用域 SW 注册。`/sw.js` 与 `/registerSW.js` 不被公开主题覆盖；升级后的
  `/sw.js` 在 Workbox 脚本前追加 core-route network bypass（不改写生成代码），旧 fallback
  不再把 Next 首页当成 `/admin` 返回。浏览器 E2E 见 `e2e/`。
- **原地迁移改为可验证的离线事务**：目标二进制先在旧服务运行期间下载并强制校验本仓库发布的
  `.sha256` / `SHA256SUMS`；通过磁盘空间门禁后才停止服务，离线打包并验证 `data/`，不再在线复制
  SQLite/WAL。历史二进制备份不再在迁移前删除。
- **启动成功判定升级**：不再仅等待三秒检查 systemd；现在轮询 systemd、`/ping` 和
  `/api/version`，并核对目标版本。超时或版本不符时，同时恢复旧二进制与离线数据归档，失败版本的
  data 会另存供取证。
- **迁移文档与门禁校准**：Agent 最低安全建议统一为 1.5.0；AI Agent 索引修正真实 `configs`
  表名并补齐 capability 三态、远控门禁和迁移风险。Web、Agent、Komari Next 三个配套仓库新增
  各自的 `AGENTS.md`。

### Changed

- **Server 更新改为仅提醒、必须人工触发（Issue #22）**：GitHub 正式 Release 仍供管理后台检测并显示
  新版本与 Release Notes，但发布工作流不再创建或移动 Docker `stable` / `latest` 浮动标签，只推送
  与 Release 同名的不可变 `v1.5.0-stable.N` 标签。历史 `:stable` 保持冻结，不删除、不重写。
- **内嵌默认前端重新 pin**：`bundled-themes.lock.json` 的 `embedded_default_frontend`
  先从 `komari-web-stable@v1.5.0-stable.1` 更新到修复 PWA fallback 的
  `v1.5.0-stable.2`，再更新到彻底删除远程命令、终端、Agent 文件管理 UI 与专用依赖的
  `v1.5.0-stable.3`，本次继续更新到修复同基础版本 stable 修订比较的
  `v1.5.0-stable.4`（`ad7ad0557805d445e638efa4acecd964eb0df1b8`，不可变 tag）。后台现在会把
  `1.5.0-stable.3 -> stable.5/stable.6` 识别为新版本并显示红色更新图标；检查只提醒，不自动升级。
  `/admin`、`/manage`、`/install`、`/database-recovery` 继续避开 Workbox navigation fallback，
  `/terminal` 请求继续到达 Server 的 `410 Gone` tombstone。Server 构建按锁定 commit 执行
  `npm ci` 与 `npm run build`。
- **随包首选主题重新 pin**：`bundled_preferred_theme` 从
  `komari-next-stable@v1.4.19-stable.1` 更新为 `v1.4.19-stable.5`
  （`9b579fe4e4801d1388e3c7db5189710de50b3c86`，不可变 tag）。锁定的 `dist-release.zip`
  SHA256 为 `3abf9f4246be148686517874edbc874dc6482114391c5db5d8f77914164bde68`，包内 manifest
  版本为 `1.4.19-stable.5`，因此全新安装的主题管理页能显示准确 stable 修订号。既有实例不会被
  自动覆盖，需管理员点击“更新主题”。

### Compatibility

- 不修改数据库、Agent/API、配置格式或 Docker 部署方式。后台版本提示仅修正 stable 修订比较；
  使用不可变 Docker tag 的部署不受影响；仍引用历史 `:stable` 的容器将停留在最后一次已发布摘要，
  不会再被新 Release 自动推动升级。
- 不修改数据库 Schema、Agent v2 已有字段、配置格式或 Docker 部署方式。任务与结果表保留，便于回滚；
  启动时仅把 `exit_code IS NULL` 的遗留任务结果标记为取消，已完成历史不变。
- 旧 Agent 仍可连接、上报监控、执行 ping 和接收普通消息。旧 `agent.taskResult` / `agent.file.result`
  报文会得到成功确认后被忽略；服务端不会向任何版本 Agent 下发远控事件。
- 旧远控 HTTP API 有意改为 `410 Gone`，旧远控 RPC 有意改为 `Method not found`。这是安全移除，
  不提供兼容开关。
- 外置 MySQL/PostgreSQL 指标库仍需用户按数据库自身方式备份；Server 脚本只负责本机 `/opt/komari`。

### Upgrade

- 以后升级 Server 必须由管理员显式执行：裸机重新运行升级脚本；Docker 先把镜像 tag 改为目标
  `v1.5.0-stable.N`，再手动 pull/recreate。后台红色更新图标只提示，不执行升级。
- 按常规方式替换二进制或更新镜像。首次启动会在版本升级备份完成后取消遗留未完成远控任务；
  无需 Schema 迁移。升级前应确认自动备份所需磁盘空间。

### Rollback

- 如需恢复旧版行为，先停止新版本，并恢复 `data/backup/upgrade-*.zip` 后再换回旧二进制/镜像。
  直接回退二进制也能读取保留的表结构，但新版本已标记为取消的未完成任务不会自动恢复为待执行状态。

## 1.5.0-stable.2 — 已发布（2026-09-17）

**基线**：同 `1.5.0-stable.0`（上游 `1.5.0-fix1` / `0ca87aa`），在 `1.5.0-stable.1` 之上增量。

### Security

- **远控能力按 Agent 自报的 capability 门禁**：Server 现在保存 Agent 上报的 capability 与
  `privilege_level`（内存态，无数据库变更），并在下发 `agent.exec` / `agent.terminal.request` /
  `agent.file` 前核对：Agent 明确声明不含该能力 → **拒绝下发**并返回明确的 `capability unavailable`
  错误（exec 还会在任务里记录被跳过的节点），不再依赖"用户点了以后 Agent 再拒绝"。
  未上报 capability 的旧 Agent 维持历史行为（不因为"没有声明"而被禁止）。
- **发布不可变策略随版本冻结**：`stable-release.yml` 的二进制、校验和与 Docker 作业全部
  使用**被发布 tag 自己**的 `scripts/release-guard.sh`；`workflow_dispatch` 的补缺重跑同样 checkout
  该 tag（此前 checksums 作业会回退到触发分支的脚本）。历史 tag 不含 guard 时 **FAIL CLOSED**
  （`this legacy release does not contain the immutable release guard; automatic repair is refused`），
  未来 `stable` 分支的改动无法再改变过去 Release 的不可变规则。

### Changed

- **内嵌默认前端重新 pin**：`bundled-themes.lock.json` 的 `embedded_default_frontend`
  从 `komari-web-stable@v1.5.0-stable.0`（`c9d4749`）更新为
  `komari-web-stable@v1.5.0-stable.1`（`b69e706a147887c96290068e9f1bad9ebe37fe55`，不可变 tag）。
  该前端按 Agent capability 隐藏/禁用终端、文件管理器与工作台入口，并显示"该 Agent 未启用远程控制"；
  对未上报 capability 的旧节点保持原 UI；安装命令生成器改为 `--enable-remote-control` 显式 opt-in。
  随包首选主题 `komari-next-stable@v1.4.19-stable.1` **保持不变**。

### Compatibility

- Agent v2 协议不变：只新增可选字段（`agent.report` 的 `capabilities` / `privilege_level`），
  旧 Agent 不发送时不改变任何行为；数据库 schema 不变；API 与配置格式不变；Docker 部署方式不变。
- 已发布的 `v1.5.0-stable.0` / `v1.5.0-stable.1` tag 与资产均未改动。

### Upgrade

- 常规升级：替换二进制或更新镜像即可，无需数据库迁移。首次启动后会逐步从 Agent 的上报中获知
  capability；在获知之前按旧行为处理。

### Rollback

- 回退到 `1.5.0-stable.1` 的二进制/镜像即可；数据库无 schema 变化，无需回滚数据。

## 1.5.0-stable.1 — 已发布（2026-09-17）

**基线**：同 `1.5.0-stable.0`（上游 `1.5.0-fix1` / `0ca87aa`）。

### Fixed

- **全新安装失败时的 bundled theme 回滚边界**：安装向导的设置在写入失败时，原先只回滚刚创建的账户，
  已 seed 的 `data/theme/next` 会残留；重试时 seed 因目录已存在而跳过，使本应默认 Next 的新实例退回 `default`。
  现在 `themebundle.Seed` 的结果显式区分 `Created` / `Skipped`（并带 `Short` / `Version` / `Path`），
  写入失败时按该结果回滚**本次**创建的目录：先删账户、再清本次 seed 的主题，回到可重新安装的干净状态；
  **安装开始前就已存在的 `data/theme/next` 绝不删除或修改**（回滚会重新校验路径形状与目录内 manifest），
  单个目录不可信时放弃回滚并记录错误，绝不误删。可用且已存在的 next 目录只被"采用"（写 `theme=next`），内容保持原样。
- **多资产 Release 的主题更新选择错误**：`web/api/admin/theme.go` 的 `getGitHubReleaseDownloadURL()`
  原先直接取 `assets[0]`，当 Release 同时包含主题包与校验和（如本 fork 的 `dist-release.zip` +
  `SHA256SUMS` + `dist-release.zip.sha256`）时可能下载到校验和文件。现在**精确优先 `dist-release.zip`**、
  与资产顺序无关；多资产且无该文件时明确报错；仅"单资产旧主题"保留兼容 fallback，
  且校验和/签名/元数据类资产永不作为主题包；最终仍由既有主题 ZIP 校验器把关。

### Compatibility

- **数据库**：无 Schema / 迁移改动。
- **Agent / API**：无变化（v2 协议、HTTP API 与配置格式均未改动）。
- **Docker 部署方式**：无变化。
- **新安装默认前台**：全新安装的首选前台主题为随包内嵌的 **Komari Next**（`short = next`）；
  `/admin`、`/terminal`、recovery/restricted 页面与 fallback 仍使用嵌入式核心前端
  （`xinian5216/komari-web-stable`）。**既有实例不受影响**：升级不 seed 主题、不创建
  `data/theme/next`、不改 `theme` 配置、不动任何既有主题目录；用户手动删除该主题后也不会被自动恢复。

### Upgrade

- 原地升级方式不变（`install-komari.sh --migrate --yes`，或 Docker 换 tag）。升级过程不触碰主题配置。

### Rollback

- 回滚方式不变（换回旧二进制/旧镜像 tag，复用同一 data 卷）。`theme` 配置与主题目录不受升级影响，
  因此回滚也不需要恢复主题数据。若在新装实例上不再需要 Komari Next，后台切回 `default` 即可
  （或删除 `data/theme/next`，不会被自动恢复）。

## 1.5.0-stable.0 — 首个稳定基线（已发布，2026-09-16）

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
