# AGENTS.md — AI Coding Agent 工作规则

本仓库是 **Komari Stable**（社区维护的 Komari 稳定分支）。
只做 **Bug 修复 / 安全修复 / 兼容性维护 / 构建 CI / 文档**，不做重构、不加功能。

## 1. 进入仓库后的强制阅读顺序

1. `.agent/README.md`
2. `.agent/PROJECT_MAP.md`
3. `.agent/MAINTENANCE_RULES.md`

然后**按任务按需**读取 `.agent/` 下的其它索引（模块索引 / 入口 / 数据库 / API / Agent 协议 / 构建测试 /
依赖 / 风险 / 变更影响）。

> ⛔ **禁止**一开始就递归读取整个仓库或在全仓库范围内搜索。
> 正确路径：读索引 → 定位模块 → 读相关源码 → 必要时扩展范围。

## 2. 红线（违反即视为错误提交）

- ⛔ 未经确认不得修改数据库 Schema / 迁移逻辑 / 用户数据结构（先报告风险并等待确认）。
- ⛔ 不得修改 Agent v2 线协议、HTTP API 字段、配置格式、Docker 部署方式（只能做向后兼容的增量）。
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

```bash
go vet ./...
go test ./... -count=1
govulncheck ./...
gitleaks git . --log-opts="--all" --config .gitleaks.toml --redact
python scripts/check_agent_index.py
```

并对接口/数据库/协议改动复核 `.agent/CHANGE_IMPACT_MAP.md`。

## 5. 索引维护

代码的模块、目录、入口、协议、数据库、构建方式发生变化时，**同步更新 `.agent/`** 对应文件
（规则见 `.agent/MAINTENANCE_RULES.md` §8）。

## 6. 技术债

发现问题 → 记入 `TECH_DEBT.md`，不要顺手重构。

## 7. 公开发布

任何内容离开本机前往公开位置前，先做密钥/隐私扫描；历史重写、发布、推送必须获得维护者确认。
