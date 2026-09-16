# TECH_DEBT.md — 已知技术债（记录，不主动重构）

> 规则：发现技术债 → **记录到本文件**；除非它直接导致 Bug / 安全 / 兼容性问题，否则不在稳定分支动手。

## 安全相关

| # | 位置 | 问题 | 影响 | 建议 |
| --- | --- | --- | --- | --- |
| 1 | `web/api/public/login.go` | 会话 Cookie 未设置 `Secure` 属性；管理接口无 CSRF token，仅依赖浏览器 `SameSite=Lax` 兜底 | 在 HTTP（非 HTTPS）部署下可被中间人利用；老浏览器下 CSRF 风险 | 上游遗留设计。不要在稳定分支重构；建议在文档中强调必须使用 HTTPS。若未来出现实际利用，单独评估 |
| 2 | `internal/config/settings.go` | `ssrf_protection_enabled` 默认 `false`（主题/插件市场、远程导入可访问内网） | 服务端可被诱导访问内网地址 | 已在 `SECURITY.md` 运维建议中提示；是否改默认值需维护者决策（会改变用户可见行为） |
| 3 | 依赖 | `golang.org/x/text@v0.33.0`、`golang.org/x/net@v0.41.0` 存在**代码可达**漏洞（另有 9+22 个不可达） | 见 `BASELINE_AUDIT.md` §6 | 计划在 `1.5.0-stable.1` 以最小升级方式处理 |
| 4 | 依赖 | `golang.org/x/crypto@v0.39.0` 多条公告（当前不可达）；`GO-2026-5932` 上游标注"无修复版本" | 潜在 | 跟踪上游；随安全升级批次处理 |

## 构建 / CI

| # | 位置 | 问题 | 建议 |
| --- | --- | --- | --- |
| 5 | CI（上游全部 workflow） | 从不运行 `go test`；无 `go vet`/lint；无依赖/CVE 扫描 | 本分支新增 `stable-ci.yml`（测试/lint/扫描/构建） |
| 6 | CI `setup-go` | 写死 `"1.23"`，与 `go.mod` 的 `go 1.25.0` 不一致（靠 toolchain 自动下载兜底） | 新 CI 显式固定 `1.25.x` |
| 7 | 前端构建 | 上游 `build-frontend` 默认克隆 `komari-web` **默认分支**（`radix`，会漂移），仅 `X.Y.Z-fixN` tag 才解析为固定 ref | 本分支固定引用自维护前端仓库的 tag/commit |
| 8 | 上游 `development.yml` | push `dev` 会 SSH 部署到**作者的生产服务器** | Fork 中应删除该 workflow（含相关 secrets 依赖） |
| 9 | 上游 `generate-release-notes.yml` | 依赖 `OPENAI_API_KEY` 生成发版说明 | Fork 中关闭，改用人工 `CHANGELOG.md` |
| 10 | 制品 | 无 checksum/SBOM/构建来源证明 | 后期可选补充 |
| 11 | 测试覆盖 | PostgreSQL/MySQL/MariaDB 集成测试默认 skip（需 DSN 环境变量），CI 无外部数据库矩阵 | 中期可加 service container 矩阵（非阻塞） |

## 代码 / 设计

| # | 位置 | 问题 | 备注 |
| --- | --- | --- | --- |
| 12 | 全仓库 | 无 `golangci-lint` 配置、无统一格式化约束 | 若引入仅用保守规则集，且先只做 CI 检查不批量改代码 |
| 13 | 全仓库 | 仅 3 处 TODO/FIXME 标记 | 低优先级 |
| 14 | 主数据库 | 仅支持 SQLite（`SupportedDatabaseTypes()` 只返回 sqlite），而指标库支持三种数据库 | 属产品设计，不在稳定分支变更 |
| 15 | `install-komari.sh` | 内嵌第三方 Lite 分支引用（`nuomiiiii/komari`）与上游仓库地址 | Fork 需决定脚本指向（见 `BASELINE_AUDIT.md` §11） |
| 16 | 前端生成的 Agent 安装命令 | 直接引用 `raw.githubusercontent.com/komari-monitor/komari-agent/refs/heads/main/...`（会漂移；上游 Agent 仓库若归档将失效） | 若上游 Agent 停止维护，考虑镜像安装脚本 |
| 17 | 前端 | `npm run lint` 有 29 条 warning（react-hooks 依赖类） | 不阻塞；不主动修 |

## 文档

| # | 位置 | 问题 |
| --- | --- | --- |
| 18 | `web/public/readme.md` | 构建说明与官方 `install/compile.md` 内容重叠，但未说明"后端必须有前端产物才能编译"这一硬依赖 | 已在本分支 README 中补充 |
| 19 | 上游文档 | `dev/compatibility.md` 中的兼容性时间表停留在 1.3.0 计划 | Fork 以 `MAINTENANCE_POLICY.md` 为准 |
