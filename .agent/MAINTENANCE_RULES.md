# MAINTENANCE_RULES.md — AI 与人类维护者共用规则

本文件是 `MAINTENANCE_POLICY.md` 的**执行版**；两者冲突时以 `MAINTENANCE_POLICY.md` 为准。

## 0. 进入项目后先做什么

1. 读 `.agent/README.md` → `.agent/PROJECT_MAP.md` → 本文件；
2. 按任务类型读对应索引（见 `.agent/README.md` 的表格）；
3. **禁止**一上来就递归扫描整个仓库。

## 1. 修改范围

- 单个改动尽量 ≤ **5~10 个核心业务文件**；超出即视为过度重构，先停下来重新评估。
- 一次提交只解决一个问题；提交信息用 `fix: / security: / build: / ci: / docs: / test: / chore:`。
- 禁止：全项目格式化、无意义 rename、目录大调整、框架/ORM/数据库替换、API 重设计、UI 重写、
  "为升级而升级"的依赖变更、删除现有功能、改变用户可见行为（明确 Bug 除外）。

## 2. 必修流程（Bug）

```
Issue → 复现 → 根因 → failing test → 最小修复 → 全量测试 → 兼容性检查 → 提交
```
无法自动化复现时，必须在提交说明里写明原因与手工验证步骤。

## 3. 数据库红线（🔴）

- **未经维护者确认，禁止**改 Schema / AutoMigrate 列表 / 任何迁移逻辑 / 用户数据结构。
- 必须先报告：为什么改、影响哪些数据、是否可丢数据、能否回滚、如何备份。
- 涉及 DB 的改动必须做"旧库升级 → 数据完整性核对 → 回滚演练"三步。
- 详见 `.agent/DATABASE.md`。

## 4. 兼容性红线

- Agent v2 协议、HTTP API 字段、配置格式、Docker 部署方式 = **冻结**；只能加，不能改/删。
- 旧 Agent（1.2.x ~ 1.5.x）必须能继续连接与上报。
- Agent 由本 fork 的镜像仓库 `xinian5216/komari-agent-stable` 维护（安装/更新通道同此）；
  Agent 侧改动只允许 Bug/安全修复，协议仍为冻结的 v2。

## 5. 危险操作（未经允许禁止）

force push、删除 Git tag / branch、重写已发布历史、覆盖不可变 Docker tag、删除 Release、
修改/部署生产环境、删除数据库或用户数据。

## 6. 安全

- 提交前跑：`gitleaks git . --log-opts="--all" --config .gitleaks.toml --redact`（CI 也会跑）。
- 任何内容离开本机前往公开位置前，先做密钥/隐私扫描；发现真实 secret 必须**先吊销再清理**，
  历史重写必须获得确认。
- 依赖升级只做安全驱动，一次一个模块族。

## 7. 技术债

- 发现问题 → 记入 `TECH_DEBT.md`，**不要顺手重构**。

## 8. 索引维护（本目录）

代码出现以下变化时，必须同步检查并更新 `.agent/`：

| 变化 | 需要更新的文件 |
| --- | --- |
| 新增/删除一级目录 | `PROJECT_MAP.md` |
| 模块职责变化、文件搬移 | `MODULE_INDEX.md`、`ENTRY_POINTS.md` |
| 启动流程/入口变化 | `ENTRY_POINTS.md`、`ARCHITECTURE.md` |
| 数据库结构/迁移变化 | `DATABASE.md`、`CHANGE_IMPACT_MAP.md` |
| API / RPC 变化 | `API.md`、`CHANGE_IMPACT_MAP.md` |
| Agent 协议变化 | `AGENT_PROTOCOL.md` |
| 构建/测试/发布方式变化 | `BUILD_TEST.md`、`INSTALLERS.md` |
| 依赖增删（关键依赖） | `DEPENDENCY_MAP.md` |
| 风险区变化 | `KNOWN_RISKS.md` |

CI（`stable-ci.yml` 的 `agent-index` 作业）只做轻量校验：索引文件是否齐全、`PROJECT_MAP.md`
提到的一级目录是否存在。**不做语义校验，避免误报**。

## 9. 自检清单（提交前）

- [ ] 读了相关索引，改动范围最小
- [ ] failing test → 修复 → 全量测试通过（`go test ./... -count=1`）
- [ ] `go vet ./...`、`govulncheck ./...` 无新增问题
- [ ] 未触碰数据库/协议红线（或已获确认并写入 CHANGELOG）
- [ ] `.agent/` 索引已按需更新
- [ ] 提交信息规范、单一问题
