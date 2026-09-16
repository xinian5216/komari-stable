# KNOWN_RISKS.md — 高风险区域

> 进入以下区域前：先读 `MAINTENANCE_RULES.md`，涉及数据库的还要读 `DATABASE.md`。
> 风险等级：🔴 极高（可能造成数据损坏/安全问题） · 🟠 高（可能破坏兼容性） · 🟡 中（容易出回归）

| 区域 | 等级 | 位置 | 风险点 | 进入时的要求 |
| --- | --- | --- | --- | --- |
| 数据库 Schema / 迁移 | 🔴 | `database/dbcore/dbcore.go`、`internal/migrations/*`、`pkg/metric/migrations.go`、`internal/metricstore/store_migration.go` | 升级失败或数据损坏；AutoMigrate 行为变化；旧库兼容 | **必须先报告并获得确认**；用真实旧库副本做升级+回滚演练 |
| 指标存储引擎 | 🔴 | `pkg/metric/*`（30k 行） | 聚合/摘要/回收逻辑错误 → 历史数据错误或丢失；大库性能 | 改动要带回归测试；避免触碰 rollup/digest 语义 |
| Agent 协议 / 上报 | 🟠 | `protocol/v2/*`、`web/api/client/*` | 旧 Agent 掉线、数据丢失、事件重复/丢失 | 只做向后兼容增量；参考 `komari-protocol` 冻结测试 |
| 升级 / 备份路径 | 🔴 | `dbcore.backupOnVersionUpgrade()`、`web/backup/*`、`web/recovery/*` | 备份失败导致无法回滚 | 不得弱化备份行为；改动要有测试 |
| Web 终端 | 🟠 | `web/api/terminal/*`、`admin.xtermjs.go` | 会话劫持、权限绕过、2FA 流程缺陷 | 关注 Origin 校验与会话归属校验 |
| 插件系统 | 🟠 | `internal/plugin/*`、`pkg/jsruntime/*` | 插件权限绕过、宿主崩溃、市场下载 SSRF | 检查权限清单与路径限制；勿放宽文件/网络访问 |
| 文件管理 / 上传 | 🟠 | `web/filemanager/*`、`web/upload/*` | 路径穿越、越权读写、大小/分片校验缺陷 | 校验路径规范化与令牌归属 |
| 认证与权限 | 🔴 | `web/api/Auth.go`、`principal.go`、`database/accounts/*` | 鉴权绕过、会话固定、Cookie 属性缺失 | 改动需覆盖 admin/client/guest 三种身份与 API Key |
| 安装 / 恢复向导 | 🟠 | `web/install/*`、`web/recovery/*` | 无认证窗口被利用、DSN 泄漏 | 保持 `requireActive` 门控与脱敏逻辑；不要把向导暴露在常规路由 |
| 删除类操作 | 🔴 | 记录清理、节点删除、插件/主题卸载、`/api/admin/record/clear/*` | 不可逆数据丢失 | 确认幂等性、确认不影响历史备份；不允许"顺手清理" |
| 配置项默认值 | 🟠 | `internal/config/settings.go` | 默认值变化 = 改变用户可见行为 | 默认值只允许在明确决策下变更并写入 CHANGELOG |
| 前端构建/嵌入 | 🟡 | `web/public/*`、`.github/actions/build-frontend/*` | 前端 ref 漂移导致构建不可复现 | 固定 ref；不要改 embed 结构 |
| CI / 发布 | 🟡 | `.github/workflows/*` | 覆盖不可变 tag、误发版 | 遵守 `MAINTENANCE_POLICY.md` §9 |

## 上游已知未修复问题（记录，不必然要修）

见 `BASELINE_AUDIT.md` §8：`#667` 历史流量统计异常（TB/PB 级）、`#673` 私有站点终端 404 + cookie 失效、
`#670` 默认主题历史统计 `unknown metric key: memory.total`、`#666` 续费后仍显示过期、`#655` Windows 用户名含空格时
Agent 安装命令不兼容。
