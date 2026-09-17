# KNOWN_RISKS.md — 高风险区域

> 进入以下区域前：先读 `MAINTENANCE_RULES.md`，涉及数据库的还要读 `DATABASE.md`。
> 风险等级：🔴 极高（可能造成数据损坏/安全问题） · 🟠 高（可能破坏兼容性） · 🟡 中（容易出回归）

| 区域 | 等级 | 位置 | 风险点 | 进入时的要求 |
| --- | --- | --- | --- | --- |
| 数据库 Schema / 迁移 | 🔴 | `database/dbcore/dbcore.go`、`internal/migrations/*`、`pkg/metric/migrations.go`、`internal/metricstore/store_migration.go` | 升级失败或数据损坏；AutoMigrate 行为变化；旧库兼容 | **必须先报告并获得确认**；用真实旧库副本做升级+回滚演练 |
| 指标存储引擎 | 🔴 | `pkg/metric/*`（仓库最大模块） | 聚合/摘要/回收逻辑错误 → 历史数据错误或丢失；大库性能 | 改动要带回归测试；避免触碰 rollup/digest 语义 |
| Agent 协议 / 上报 | 🟠 | `protocol/v2/*`、`web/api/client/*` | 旧 Agent 掉线、数据丢失、事件重复/丢失 | 只做向后兼容增量；参考 `komari-protocol` 冻结测试；本 fork 维护 `xinian5216/komari-agent-stable`，Agent 侧也只允许 Bug/安全修复 |
| Agent capability / 远控门禁 | 🔴 | `web/api/client/report_v2.go`、`internal/agent_runtime/*`、`web/rpc/jsonrpc/admin.{client,system,xtermjs,file}.go` | “未知”被误判为禁用会破坏旧 Agent；虚报能力会绕过 UI/Server 门禁 | 保持三态语义；Server 必须在下发侧再次拒绝，不只依赖前端隐藏 |
| 原地迁移脚本 | 🔴 | `install-komari.sh`、`.github/workflows/migration-test.yml` | 在线打包 SQLite、磁盘不足、仅检查进程未检查 API、失败后数据/二进制不一致 | 下载先于停机；停机后冷备份；验证归档、目标版本和 API；失败同时恢复数据与二进制 |
| 升级 / 备份路径 | 🔴 | `dbcore.backupOnVersionUpgrade()`、`web/backup/*`、`web/recovery/*` | 备份失败导致无法回滚 | 不得弱化备份行为；改动要有测试 |
| Web 终端 | 🟠 | `web/api/terminal/*`、`admin.xtermjs.go` | 会话劫持、权限绕过、2FA 流程缺陷 | 关注 Origin 校验与会话归属校验 |
| 插件系统 | 🟠 | `internal/plugin/*`、`pkg/jsruntime/*` | 插件权限绕过、宿主崩溃、市场下载 SSRF | 检查权限清单与路径限制；勿放宽文件/网络访问 |
| JS 运行时 Node 兼容层 | 🟠 | `pkg/jsruntime/fs/*`、`pkg/jsruntime/child_process/*`、`pkg/jsruntime/stream/*` | 参数解析与 Node 语义不一致（如 options 位置传字符串）导致文件权限错误；子进程 stdio 与回收顺序错误导致输出丢失/流不触发 EOF | 修改前先确认 Node 的实际语义；**子进程回收必须等 stdout/stderr 泵读完再 `cmd.Wait()`**（Go 文档明确要求）；Linux 与 Windows 行为不同，需两端验证 |
| 文件管理 / 上传 | 🟠 | `web/filemanager/*`、`web/upload/*` | 路径穿越、越权读写、大小/分片校验缺陷 | 校验路径规范化与令牌归属 |
| 认证与权限 | 🔴 | `web/api/Auth.go`、`principal.go`、`database/accounts/*` | 鉴权绕过、会话固定、Cookie 属性缺失 | 改动需覆盖 admin/client/guest 三种身份与 API Key |
| 安装 / 恢复向导 | 🟠 | `web/install/*`、`web/recovery/*` | 无认证窗口被利用、DSN 泄漏 | 保持 `requireActive` 门控与脱敏逻辑；不要把向导暴露在常规路由 |
| 删除类操作 | 🔴 | 记录清理、节点删除、插件/主题卸载、`/api/admin/record/clear/*` | 不可逆数据丢失 | 确认幂等性、确认不影响历史备份；不允许"顺手清理" |
| 配置项默认值 | 🟠 | `internal/config/settings.go` | 默认值变化 = 改变用户可见行为 | 默认值只允许在明确决策下变更并写入 CHANGELOG |
| 前端构建/嵌入 | 🟡 | `web/public/*`、`bundled-themes.lock.json`、`scripts/prepare-assets.py`、`.github/actions/prepare-assets/*` | 资产 ref/tag 漂移导致构建不可复现；未校验资产进入二进制 | 只通过锁文件固定 repository/tag/commit/sha256；不要改 embed 结构；CI 会断言产物哈希与锁一致 |
| 随包主题 seed | 🟠 | `internal/themebundle/*`、`internal/bundledtheme/*`、`web/install/install.go` | 误在存量实例上 seed/改写主题；半解压残留；覆盖用户已删除的主题；安装失败残留半成品主题 | **只在全新安装（零用户）路径调用**；`data/theme/<short>` 已存在时绝不覆盖；解压走临时目录 + 原子 rename；任何失败都不得让安装失败（仅 warning，保持 `default`）；设置写入失败时只回滚 `Result.Created` 的目录（先校验路径形状 + manifest，绝不误删已存在的主题） |
| CI / 发布 | 🟡 | `.github/workflows/*` | 覆盖不可变 tag、误发版 | 遵守 `MAINTENANCE_POLICY.md` §9 |

## 上游已知未修复问题（记录，不必然要修）

见 `BASELINE_AUDIT.md` §8：`#667` 历史流量统计异常（TB/PB 级）、`#673` 私有站点终端 404 + cookie 失效、
`#670` 默认主题历史统计 `unknown metric key: memory.total`、`#666` 续费后仍显示过期、`#655` Windows 用户名含空格时
Agent 安装命令不兼容。
