# INSTALLERS.md — 安装 / 更新 / 下载来源清单

> 目标（§20/§21）：所有安装路径都能指向本 fork 自己的仓库，且**不在多处硬编码账号**。
> 原则：先登记现状与参数化位置，**等仓库真正存在后再写死地址**。

## 1. 现状清单

| 入口 | 位置 | 下载来源 | 状态 |
| --- | --- | --- | --- |
| 裸机安装脚本 | `install-komari.sh` | `${RELEASE_BASE}/${REPO}/releases/{latest/download|download/<tag>}`；API `${GITHUB_API_BASE}/repos/${REPO}/releases` | ✅ 已参数化（`REPO_OWNER` / `REPO_NAME` / `RELEASE_BASE` / `GITHUB_API_BASE`，均可用环境变量覆盖） |
| Docker 镜像 | `.github/workflows/stable-release.yml` → `ghcr.io/${IMAGE_NAME}` | ghcr | ✅ 使用 `${{ github.repository }}`，自动指向 fork |
| 发布资产（二进制） | `stable-release.yml` | GitHub Release 资产 `komari-<os>-<arch>[.exe]` + `SHA256SUMS` | ✅ 命名与 `install-komari.sh` 期望一致 |
| Agent 安装命令 | 前端仓库（镜像后为 `komari-web-stable`）：`src/components/admin/NodeTable/NodeFunction.tsx`、`src/pages/admin/index.tsx` | 当前硬编码上游 `raw.githubusercontent.com/komari-monitor/komari-agent/...`；**决策：改为指向本 fork 的 `xinian5216/komari-agent-stable`** | 🚧 计划已定，待两个镜像仓库创建后落地（见 §2） |
| 更新说明 | 官方文档 `komari-document`（en/install/update.md） | 上游文档 | ⏳ fork 文档（README.md 已有简述） |

## 2. 待办（等仓库/镜像就绪后执行）

1. **前端镜像**：`xinian5216/komari-web-stable`（源自上游 `komari-web`，固定 tag）——CI 已按此固定。
2. **Agent 镜像（2026-09-16 决策）**：建立并维护 `xinian5216/komari-agent-stable`（源自上游
   `komari-agent`），作为本 fork 的 Agent **安装与更新通道**；随后把前端生成的安装命令 URL
   指向该仓库的 raw 地址。
   ⚠️ 约束：Agent 侧仍使用 **v2 冻结协议**，改动仅限 Bug/安全修复；服务端必须继续兼容旧 Agent。
3. **脚本自身分发地址**：如提供"一键安装"命令，形如
   `curl -fsSL <raw 地址>/install-komari.sh | bash`，raw 地址指向 `xinian5216/komari-stable`。
   目前 README 未写死一键命令——**发布后再补**。
4. **Docker tag 策略**：`stable`（浮动）+ `1.5.0-stable.N`（不可变，永不覆盖）。
5. **镜像代理**：如需国内加速，通过环境变量 `KOMARI_RELEASE_BASE` / `KOMARI_GITHUB_API_BASE` 覆盖，
   不要改动脚本正文。

## 3. 修改安装相关代码时的规则

- 新增下载地址时，必须复用 `REPO_OWNER` / `REPO_NAME` / `RELEASE_BASE` 变量，禁止再次硬编码
  `xinian5216`（账号迁移、转 Organization 时只改一处）。
- 发布资产**命名不得变更**（`komari-<os>-<arch>`、`komari-windows-amd64.exe` 等），
  否则安装脚本与文档会失效；如需变更，先改 `install-komari.sh` 与 `BUILD_TEST.md`。
- 任何安装脚本改动都要在 `.agent/BUILD_TEST.md` §7 的"发布对应关系"里复核一遍。
