# .agent/ — AI Coding Agent 索引

本目录是给 AI Coding Agent 用的**轻量索引**，不是项目百科全书。

**目标：用几千 Token 理解项目结构，而不是用几十万 Token 扫描整个仓库。**

## 强制阅读顺序

任何 Agent 进入本仓库后，**按顺序**读取：

1. `.agent/README.md`（本文件）
2. `.agent/PROJECT_MAP.md`（一级目录职责）
3. `.agent/MAINTENANCE_RULES.md`（修改规则与红线）

然后**按任务需要**再读：

| 任务类型 | 追加阅读 |
| --- | --- |
| 找代码位置 | `MODULE_INDEX.md`、`ENTRY_POINTS.md` |
| 改数据库 / 迁移 | `DATABASE.md`、`CHANGE_IMPACT_MAP.md`、`KNOWN_RISKS.md` |
| 改 HTTP/RPC 接口 | `API.md`、`CHANGE_IMPACT_MAP.md` |
| 改 Agent 协议 / 上报 | `AGENT_PROTOCOL.md`、`KNOWN_RISKS.md` |
| 构建 / 测试 / 发布 | `BUILD_TEST.md`、`INSTALLERS.md` |
| 依赖 / 升级 | `DEPENDENCY_MAP.md` |
| 理解整体设计 | `ARCHITECTURE.md` |

## 工作方式（禁止事项）

- ❌ 禁止一开始递归读取整个 Repository 或 `git ls-files` 全量遍历；
- ✅ 先读索引 → 确定相关模块 → 只读相关源码 → 必要时再扩展范围。

## 维护

- 代码发生**模块新增/删除、目录变化、协议变化、数据库变化、核心入口变化**时，必须同步检查本目录
  并按需更新（见 `MAINTENANCE_RULES.md` §索引维护）。
- CI 会做轻量一致性检查（`.github/workflows/stable-ci.yml` 的 `agent-index` 作业 +
  `scripts/check_agent_index.py`）：校验索引文件、PROJECT_MAP 一级路径，以及少量来自源码的关键事实
  （当前配置表名、Agent capability 文档和迁移最低版本）；它不能代替人工语义复核。
- 本目录文件**只记录位置 / 职责 / 依赖 / 入口 / 风险 / 阅读路径**；
  不要复制大段源码、函数实现或教程。
