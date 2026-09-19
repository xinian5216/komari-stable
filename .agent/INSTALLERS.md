# INSTALLERS.md — 安装 / 更新 / 下载来源清单

> 目标（§20/§21）：所有安装路径都能指向本 fork 自己的仓库，且**不在多处硬编码账号**。
> 原则：先登记现状与参数化位置，**等仓库真正存在后再写死地址**。

## 1. 现状清单

| 入口 | 位置 | 下载来源 | 状态 |
| --- | --- | --- | --- |
| 裸机安装脚本 | `install-komari.sh` | `${RELEASE_BASE}/${REPO}/releases/{latest/download|download/<tag>}`；API `${GITHUB_API_BASE}/repos/${REPO}/releases` | ✅ 已参数化（`REPO_OWNER` / `REPO_NAME` / `RELEASE_BASE` / `GITHUB_API_BASE`，均可用环境变量覆盖） |
| Docker 镜像 | `.github/workflows/stable-release.yml` → `ghcr.io/${IMAGE_NAME}` | ghcr | ✅ 使用 `${{ github.repository }}`，自动指向 fork |
| 发布资产（二进制） | `stable-release.yml` | GitHub Release 资产 `komari-<os>-<arch>[.exe]` + `SHA256SUMS` | ✅ 命名与 `install-komari.sh` 期望一致 |
| Agent 安装命令 | 前端仓库 `xinian5216/komari-web-stable`：`src/lib/repoSources.ts`（集中定义）+ `src/components/admin/NodeTable/NodeFunction.tsx`、`src/pages/admin/index.tsx` | `https://raw.githubusercontent.com/xinian5216/komari-agent-stable/refs/heads/stable/install.{sh,ps1}`（由 `AGENT_INSTALL_RAW_BASE` 生成） | ⚠️ 下载源已迁移；远控参数清理由配套前端/Agent 变更完成，Server 已忽略该能力 |
| 更新说明 | 官方文档 `komari-document`（en/install/update.md） | 上游文档 | ⏳ fork 文档（README.md 已有简述） |

## 2. 待办（等仓库/镜像就绪后执行）

1. **前端镜像**：`xinian5216/komari-web-stable`（源自上游 `komari-web`，固定 tag）——✅ 已创建，CI 已按此固定（当前 `v1.5.0-stable.3`）。
2. **Agent 镜像（2026-09-16 决策）**：`xinian5216/komari-agent-stable`——✅ 已创建（当前 `v1.5.10-stable.1`），
   安装/更新 URL 已指向该仓库。约束与现状：
   - Agent 侧仍使用 **v2 冻结协议**，改动仅限 Bug/安全修复；服务端必须继续兼容旧 Agent；
   - 仓库已公开，raw URL、匿名 Release 下载与校验和闭环已由 E2E 验证。
3. **脚本自身分发地址**：如提供"一键安装"命令，形如
   `curl -fsSL <raw 地址>/install-komari.sh | bash`，raw 地址指向 `xinian5216/komari-stable`。
   README 已提供一键安装、官方实例原地迁移、状态检查和回滚命令。
4. **Docker tag 策略**：`stable`（浮动）+ `1.5.0-stable.N`（不可变，永不覆盖）。
5. **镜像代理**：如需国内加速，通过环境变量 `KOMARI_RELEASE_BASE` / `KOMARI_GITHUB_API_BASE` 覆盖，
   不要改动脚本正文。

## 3. 修改安装相关代码时的规则

- 新增下载地址时，必须复用 `REPO_OWNER` / `REPO_NAME` / `RELEASE_BASE` 变量，禁止再次硬编码
  `xinian5216`（账号迁移、转 Organization 时只改一处）。
- 发布资产**命名不得变更**（`komari-<os>-<arch>`、`komari-windows-amd64.exe` 等），
  否则安装脚本与文档会失效；如需变更，先改 `install-komari.sh` 与 `BUILD_TEST.md`。
- 任何安装脚本改动都要复核 `.agent/BUILD_TEST.md` §6 的 CI 对应关系，并运行 §7 的迁移 E2E。
- Server 原地迁移必须先下载并校验，停机后再做离线数据归档；新版本只有在 systemd、`/ping` 和
  `/api/version` 均通过时才算成功，否则同时恢复旧二进制和离线数据备份。
