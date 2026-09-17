# MAINTENANCE_POLICY.md — Komari Stable 维护策略

本文件定义 **哪些改动允许进入 `stable` 分支**、如何评审、如何发布与回滚。
任何与本文冲突的做法都应先修改本文，再执行。

## 1. 优先级顺序（冲突时按此排序）

```
数据库兼容 > Agent/API 兼容 > 安全 > 严重 Bug > 普通 Bug > 性能 > 代码整洁 > 新功能
```

## 2. 允许进入 `stable` 的改动

| 类别 | 说明 | 提交前缀 |
| --- | --- | --- |
| Bug 修复 | 有明确复现路径、有根因分析、最小改动 | `fix:` |
| 安全修复 | CVE / GHSA / 自审发现，且修复方式最小化 | `security:` |
| 兼容性修复 | 让旧数据、旧 Agent、旧配置、旧部署方式继续可用 | `fix:` |
| CI / 构建 | 补齐测试、静态检查、漏洞扫描；修正构建可复现性 | `ci:` / `build:` |
| 文档 | README、审计、策略、变更记录 | `docs:` |
| 测试 | 为已修复问题补充回归测试 | `test:` |
| 杂项维护 | 版本号、元数据等无行为影响的改动 | `chore:` |

## 3. 禁止进入 `stable` 的改动（除非维护者明确决策并记录）

- 全项目格式化、批量重命名、目录大规模调整；
- 框架迁移、ORM 替换、数据库替换、API 重设计、UI 全面重写；
- 为"现代化"而升级依赖；依赖大版本跳跃；
- 新增大型功能；删除现有功能；改变用户可见行为（明确 Bugs 除外）；
- 修改数据库 Schema / 迁移路径 / 用户数据结构（见 §6，需单独审批）；
- 修改 Agent v2 线协议或使其不向后兼容（见 §7）。

技术债一律先记入 `TECH_DEBT.md`，不做"顺手修复"。

## 4. 缺陷处理流程（强制）

```
Issue → 复现 → 根因 → 写失败测试（failing test） → 最小修复 → 全量测试
      → 兼容性检查（DB / Agent / Docker / API / 配置） → 提交
```

- 修复前**必须先有能复现的测试**；确实无法自动化（如涉及真实 agent 网络环境）时，
  必须在 PR/提交说明中写明原因与手工验证步骤。
- 单个修复尽量只涉及 **≤ 5~10 个核心业务文件**；超出即视为过度重构，需重新评估。
- 提交信息遵循 `fix: / security: / build: / ci: / docs: / test: / chore:`，一次提交只解决一个问题。

## 5. 分支与版本

- `upstream-baseline`：上游 `1.5.0-fix1`(`0ca87aa`) 的只读镜像，**永不修改、永不合入**。
- `stable`：唯一维护分支，所有修复经 PR 或直接提交（小改动）进入。
- 版本号 `1.5.0-stable.N`；`N` 从 0 开始递增。不擅自升级为 `1.6.0`。
- Git 操作红线：不 force push、不删 tag/branch、不重写已发布历史、不覆盖不可变 tag。

## 6. 数据库保护（最高风险模块）

任何触碰数据库的改动（Schema、`AutoMigrate` 列表、`internal/migrations`、`pkg/metric` 迁移、
配置键迁移）必须：

1. 分析现有 Schema 与迁移链（含 0.x/1.0.x/1.1.x 兼容路径）；
2. 用**真实旧库副本**做升级测试（至少覆盖 `1.4.3` 与 `1.5.0` 数据）；
3. 核对升级后数据完整性（节点、账号、历史记录、指标库）；
4. 验证旧版本二进制能否回滚（`data/backup/upgrade-*.zip` 恢复）；
5. 在 CHANGELOG 的 `Compatibility` / `Rollback` 段写明影响。

**不可逆改动先停下来报告**：为什么必须改、哪些数据会变化、是否可能丢数据、是否可回滚、如何备份。
未经确认不得执行。

## 7. Agent / API 兼容策略

- 服务端必须尽可能兼容官方 Agent（v2 协议，`/api/clients/v2/rpc`：WebSocket 优先、POST 回退、`agent.pull`）。
- 不主动修改线协议；确需修改时必须**向后兼容**（新服务端仍接受旧 Agent 报文）。
- 不得因服务端重构导致旧 Agent 无法连接、上报、执行任务或建立终端会话。
- **Agent 由本 fork 维护镜像仓库** `xinian5216/komari-agent-stable`（源自上游 `komari-agent`）：
  其安装脚本、发布与更新通道由本 fork 负责；Agent 侧只接受 Bug/安全修复，**协议固定为冻结的 v2**，
  且服务端不得因 Agent 版本推进而拒绝旧 Agent。

## 8. 依赖与安全维护

- 定期：`govulncheck ./...`、GitHub Security Advisory、Dependabot/依赖公告、WebSocket/终端/上传/插件
  等高危面复查。
- 依赖升级只做**安全驱动**，且一次一个模块族，单独提交、单独发布，升级后必须：
  全量测试 + `govulncheck` 复核 + 记录到 CHANGELOG。
- **不要因为扫描工具报 Low Risk 就批量升级依赖**；升级必须评估兼容性。
- 扫描到的但当前不可达的漏洞：记录在 `TECH_DEBT.md` 或审计文件，随安全升级一并处理。

## 9. 发布规程

1. 更新 `CHANGELOG.md`（Fixed / Security / Compatibility / Upgrade / Rollback 五段齐全）。
2. 打 tag `1.5.0-stable.N` 并创建 GitHub Release；资产 = 各平台二进制。
3. Docker：
   - 浮动 tag `stable` 指向最新稳定发布；
   - 不可变 tag `1.5.0-stable.N` **永不覆盖**。
4. 发布资产、源码 tag、Docker tag 必须对应**同一个 commit**。

### 分支与 tag 的关系（发布后）

- **Release 二进制永远对应不可变的 release tag**：`v1.5.0-stable.0` 这类版本 tag 一旦发布即冻结，
  其指向的 commit 与上传的二进制资产都不再变化。
- **`stable` 是持续维护分支**：允许在 Release 之后继续前进（文档、测试、CI、迁移脚本等提交），
  分支 tip 与已发布 tag 不一致是**正常状态**，不需要、也不得为了让两者一致而重写历史。
- **文档更新不改变旧 Release 的二进制**：README / UPSTREAM / 审计文档的后续修改只影响分支，
  不影响已发布资产；用户按 tag 拉取的产物与其校验和保持不变。
- **禁止 force-move 已发布的 tag**；如需修正问题，发布新版本（`1.5.0-stable.1`、…），而不是移动旧 tag。
- 版本 tag（如 `v1.5.0-stable.0`）**不可覆盖**；仅浮动 tag（`stable`、`:stable` 镜像标签）随最新发布移动。
5. 发布说明中必须给出升级方法、回滚方法与兼容性影响。

## 10. 回滚规程

- 二进制：停服 → 换回旧版本可执行文件 → 启动；数据目录保持不变。
- Docker：`docker pull <image>:<旧不可变 tag>` 后重建容器，保留 `data` 卷。
- 若数据已被新版本迁移：使用 `data/backup/upgrade-<时间戳>.zip` 恢复整个 `data` 目录后再回滚二进制。
- 回滚后请在 issue 中记录现象，便于定位。

## 11. 变更评审清单（每次提交前自检）

- [ ] 有 issue / 明确问题描述与复现步骤
- [ ] 有失败测试（或说明无法自动化的原因）
- [ ] 改动范围最小（≤5~10 个核心文件）
- [ ] 不影响 DB / Agent / API / 配置 / Docker 部署（或已在 CHANGELOG 说明）
- [ ] `go vet` / `go test ./...` / `govulncheck` 全部通过
- [ ] 提交信息符合约定式前缀
