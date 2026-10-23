# AGENTS.md — AI Coding Agent 工作规则

本仓库是 **Komari Stable**（社区维护的 Komari 稳定分支）。
只做 **Bug 修复 / 安全修复 / 兼容性维护 / 构建 CI / 文档**，不做重构、不加功能。

## 1. 进入仓库后的强制阅读顺序

1. `.agent/README.md`
2. `.agent/PROJECT_MAP.md`
3. `.agent/MAINTENANCE_RULES.md`

然后**按任务按需**读取 `.agent/` 下的其它索引（模块索引 / 入口 / 数据库 / API / Agent 协议 / 构建测试 /
依赖 / 风险 / 变更影响）。

涉及提交、CI 或发布时先读 `.github/REQUIRED_CHECKS.md`；检查名称和失败策略以该文件与实际 workflow
为准。工具链版本以 `go.mod`、构建 workflow 和资产锁文件为准，不在本文件重复维护版本号。

> ⛔ **禁止**一开始就递归读取整个仓库或在全仓库范围内搜索。
> 正确路径：读索引 → 定位模块 → 读相关源码 → 必要时扩展范围。

## 2. 红线（违反即视为错误提交）

- ⛔ 未经确认不得修改数据库 Schema / 迁移逻辑 / 用户数据结构（先报告风险并等待确认）。
- ⛔ 不得修改 Agent v2 线协议、HTTP API 字段、配置格式、Docker 部署方式（只能做向后兼容的增量）。
- ⛔ Agent 与前端分别由镜像仓库 `xinian5216/komari-agent-stable` / `xinian5216/komari-web-stable` 维护；
  对 Agent 的任何改动都必须保持 **v2 协议冻结并向后兼容**（服务端不得拒绝旧 Agent）。
- ⛔ 远程命令、终端与 Agent 文件管理已从 Stable 移除：不论 Agent 是否上报 capability，
  Server 均须过滤远控能力并拒绝下发远控方法；旧 Agent 未上报是“未知”，只用于监控能力的兼容判断，
  绝不能据此重新开放远控。保留路由 `410 Gone`、RPC 注销与事件下发拒绝，不依赖 Web 隐藏入口。
- ⛔ 禁止：全项目格式化、无意义重命名、目录大调整、框架/ORM/数据库替换、API 重设计、UI 重写、
  "为升级而升级"的依赖变更、删除现有功能、改变用户可见行为（明确 Bug 除外）。
- ⛔ 禁止未经允许：force push、删除 tag/branch、重写已发布历史、覆盖不可变 Docker tag、删除 Release。

## 3. 修复流程（强制）

```text
Issue → 复现 → 根因 → failing test → 最小修复 → 全量测试 → 兼容性检查 → 提交
```

- 单个改动尽量 ≤ 5~10 个核心业务文件；一次提交只解决一个问题。
- 提交信息前缀：`fix:` / `security:` / `build:` / `ci:` / `docs:` / `test:` / `chore:`。
- 无法自动化复现时，须在提交说明中写明原因与手工验证步骤。

## 4. 提交前自检

先按 `.agent/BUILD_TEST.md` 准备锁定的构建期资产，并配置 CGO / C 编译器，再运行下列 Go 检查。
不得把缺少嵌入资产或工具链不匹配误判成代码回归；govulncheck 使用安全 workflow 固定的版本。

```bash
python scripts/prepare-assets.py all
go vet ./...
go test ./... -count=1
govulncheck ./...
gitleaks git . --log-opts="--all" --config .gitleaks.toml --redact
python scripts/check_agent_index.py
python scripts/check_remote_control_removed.py
python .github/scripts/check-workflow-pins.py
```

并对接口/数据库/协议改动复核 `.agent/CHANGE_IMPACT_MAP.md`。
根据改动执行 `.agent/BUILD_TEST.md` 中的安装、迁移、随包主题与真实浏览器回归；未执行或因环境
跳过的项目必须说明，不能把静态检查等同于端到端验证。纯文档改动也必须通过分支要求的 CI。

## 5. 索引维护

代码的模块、目录、入口、协议、数据库、构建方式发生变化时，**同步更新 `.agent/`** 对应文件
（规则见 `.agent/MAINTENANCE_RULES.md` §8）。

## 6. 技术债

发现问题 → 记入 `TECH_DEBT.md`，不要顺手重构。

## 7. 公开发布

任何内容离开本机前往公开位置前，先做密钥/隐私扫描。推送、合并、发布和生产部署按本次任务
明确授权的范围执行；已有授权无需重复确认，仓库发布不代表获得线上部署授权。

- 所有变更经 PR 进入 `stable`；必须等待 `.github/REQUIRED_CHECKS.md` 中全部门禁成功，
  不能用成功的 CodeQL 代替依赖扫描，也不能接受失败、取消或跳过。
- 发布前复核合并后精确 commit 的检查、tag 目标及资产来源；发布后下载核对校验和和镜像，
  不覆盖既有 tag、资产或固定版本镜像。细节见 `.agent/BUILD_TEST.md` 和 `MAINTENANCE_POLICY.md`。
- Web / Next 新版本发布后，Server 不会自动采用；集成时需更新 `bundled-themes.lock.json`
  的 tag、commit、资产 SHA256 等关联字段，准备资产并执行兼容回归后重新发布。
  不得自动替换已安装站点主题或修改用户数据。
