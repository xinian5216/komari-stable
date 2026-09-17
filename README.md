# Komari Stable

**Community-maintained stable fork of [Komari](https://github.com/komari-monitor/komari), focused on bug fixes, security patches and long-term compatibility.**

> ⚠️ **非官方项目 / Unofficial fork**
> 本仓库是 **社区维护** 的 Komari 稳定分支，**与原作者及 komari-monitor 组织无直接关系**。
> 上游仓库已于 2026-09-15 被作者归档（read-only），本分支基于上游最后一个发布版本
> **`1.5.0-fix1`（commit `0ca87aa`）** 建立。
>
> 目标只有一个：让已经部署 Komari 的用户**继续获得 Bug 修复与安全补丁**，而不是重写或扩展 Komari。

---

## 中文说明

### 定位

- ✅ **只做**：Bug 修复、安全修复、兼容性维护、构建/CI 修正、文档。
- ❌ **不做**：重构、框架/依赖大版本迁移、API 重设计、UI 重写、大型新功能。
- 🔒 **冻结**：数据库 Schema、Agent v2 线协议、HTTP API 行为、配置格式、Docker 部署方式。
- 🎨 **新安装默认前台主题**：**Komari Next**（随包内嵌，原作者 **Tony Liu / `tonyliuzj`**，
  <https://github.com/tonyliuzj/komari-next> 的稳定镜像 `xinian5216/komari-next-stable`）。
  本 fork 只做稳定镜像与兼容维护，不主张主题著作权。管理后台、终端、恢复页面与 fallback 仍使用
  嵌入式核心前端 `xinian5216/komari-web-stable`；**既有实例升级不受影响**，任何时候都能在
  后台把前台切回内置 `default` 主题。

### 版本与分支

| 分支 | 用途 |
| --- | --- |
| `upstream-baseline` | 上游官方基线（`1.5.0-fix1` / `0ca87aa`）的只读镜像，**永不修改** |
| `stable` | 长期维护分支，所有修复提交在这里 |

版本号格式：`1.5.0-stable.1`、`1.5.0-stable.2`…… 依次递增；除非出现大型功能变化，不会升级为 `1.6.0`。

### 部署、迁移与升级

一键安装脚本：`install-komari.sh`（本仓库自带，发布源即本仓库，不依赖上游）。

部署入口一览：**一键安装 Server** / **Docker 安装** / **Docker Compose** / **从官方 Server 原地迁移** /
**全新安装 Agent** / **从官方 Agent 接管** / **更新** / **回滚** / **从源码构建**。

#### 1. 一键安装 Server

```bash
# 交互式（与官方脚本体验一致）
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-stable/stable/install-komari.sh | sudo bash

# 非交互（默认 25774 端口）
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-stable/stable/install-komari.sh | sudo bash -s -- --install --yes

# 查看当前版本 / 来源 / 服务状态（不需要 root）
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-stable/stable/install-komari.sh | bash -s -- --status
```

#### 2. Docker 安装（Docker Run）

镜像发布在 GHCR：`ghcr.io/xinian5216/komari-stable`。**无需登录即可拉取**（匿名拉取已实测通过）。
容器内工作目录 `/app`，数据在 `/app/data`，监听 `25774`。

```bash
# 浮动 tag：始终跟随最新 stable 版本
docker run -d --name komari \
  -p 25774:25774 \
  -v komari-data:/app/data \
  --restart unless-stopped \
  ghcr.io/xinian5216/komari-stable:stable

# 固定版本 tag（生产环境推荐：可复现、可回滚）
docker run -d --name komari \
  -p 25774:25774 \
  -v komari-data:/app/data \
  --restart unless-stopped \
  ghcr.io/xinian5216/komari-stable:v1.5.0-stable.0
```

- `-p 25774:25774`：面板端口（镜像内 `KOMARI_LISTEN=0.0.0.0:25774`）。
- `-v ...:/app/data`：**必须挂载**，主库、metrics 库、配置、插件与主题都在这里；镜像本身未声明 VOLUME。
- `--restart unless-stopped`：随主机重启自动恢复。
- 强调可复现性的生产环境，建议固定到 `v1.5.0-stable.0` 这类**不可变版本 tag**，而不是 `stable` 浮动 tag。

#### 3. Docker Compose

仓库根目录提供最小可用的 [`compose.yaml`](./compose.yaml)：

```yaml
services:
  komari:
    image: ghcr.io/xinian5216/komari-stable:stable
    container_name: komari
    ports:
      - "25774:25774"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

```bash
docker compose up -d
```

**从官方镜像切换过来**（原先使用 `ghcr.io/komari-monitor/komari`）：两者数据都在容器内 `/app/data`，
因此**复用原来的挂载即可**，但必须先备份。下面同时覆盖 **named volume** 与 **bind mount**，
并用"**重命名旧容器**"的方式保留回滚可能、同时避免容器名冲突。

```bash
# 0) 先确认旧容器用的是哪种挂载（记下卷名或宿主机目录）
docker inspect <旧容器名> --format '{{json .Mounts}}'
#    named volume → 看 "Name"（例如 komari-data，也可能是你自己的命名，不要想当然）
#    bind mount   → 看 "Source"（宿主机目录，例如 /srv/komari/data）

# 1) 备份数据
#    named volume（<卷名> 换成上一步看到的 Name）：
docker run --rm -v <卷名>:/data -v "$(pwd)":/backup alpine \
  tar czf /backup/komari-data-$(date +%F).tar.gz -C /data .
#    bind mount（<宿主机目录> 换成上一步看到的 Source）：
sudo tar czf ./komari-data-$(date +%F).tar.gz -C <宿主机目录> .

# 2) 停止旧容器后【重命名】它：既保留完整旧容器用于回滚，又释放 komari 这个名称
docker stop <旧容器名>
docker rename <旧容器名> komari-old-$(date +%F)

# 3) 用原挂载创建新的 komari 容器（此时名称已空闲，不会 name conflict）
#    named volume：
docker run -d --name komari -p 25774:25774 -v <卷名>:/app/data \
  --restart unless-stopped ghcr.io/xinian5216/komari-stable:v1.5.0-stable.0
#    bind mount（宿主目录与旧容器保持一致）：
docker run -d --name komari -p 25774:25774 -v <宿主机目录>:/app/data \
  --restart unless-stopped ghcr.io/xinian5216/komari-stable:v1.5.0-stable.0

# 4) 验证新实例：面板可登录、节点在线、历史数据完整
# 5) 验证成功后再由你决定是否删除旧容器/旧镜像；本流程不自动删除任何容器或数据卷
```

如果确实需要回到旧镜像：`docker stop komari && docker rename komari komari-new-<失败日期> &&
docker rename komari-old-<日期> komari && docker start komari`（旧容器与其挂载一直在，数据未动）。

#### 4. 从官方 Server 原地迁移（原地升级，不重装）

安装路径与服务名与官方一致（`/opt/komari`、`komari.service`），因此**官方脚本安装的实例可被直接接管**：

```bash
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-stable/stable/install-komari.sh -o /tmp/install-komari.sh
sudo bash /tmp/install-komari.sh --migrate --yes
```

迁移过程会：

- 显示**当前版本 / 目标版本 / 旧来源 / 目标来源**并向你确认；
- 升级前把整个 `data/`（主库 `komari.db`、`metrics.db`、配置、用户、节点、插件与 `plugin-data`、主题）打包为
  `/opt/komari/backup/komari-migrate-<时间戳>.tar.gz`（失败即中止，不改动任何东西）；
- 备份旧二进制为 `/opt/komari/komari.backup.<时间戳>`；
- 下载新二进制到暂存文件，**校验 `.sha256`**（发布方提供时强制校验）后再落位；
- 启动失败时**自动回滚**旧二进制。

> **推荐顺序（重要）**：① 先接管 Agent（见 §6），逐个确认 `Github Repo:` 已变为
> `xinian5216/komari-agent-stable` 且节点以 v2 在线；② 再原地迁移 Server。
> **升级主控不会自动改变既有 Agent 的更新源**——Server 升级后，旧 Agent 不会自动跟随本 fork，
> 必须每个 Agent 执行一次接管（§6）。

> ⚠️ **升级前请确认 Agent 兼容性**：服务端 1.5.0 起只接受 **v2 协议**。Agent 的 v2 支持自
> **1.2.10** 引入（当时可选），**1.5.0 起 v2 成为唯一协议**。因此迁移前请确保每个节点的 Agent
> **不低于 1.5.0**；如果个别节点仍在 1.2.10–1.4.x，需先确认它实际以 v2 上报，否则升级后无法上报。
> 最稳妥的做法：先把所有 Agent 升级到 **v1.5.10-stable.0**（本 fork），再升级面板。
> 逐个确认方式：面板 nodes 页，或 `journalctl -u komari-agent | grep -i "protocol\|version"`。

#### 5. 全新安装 Agent

```bash
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-agent-stable/stable/install.sh | sudo bash -s -- -e <ENDPOINT> -t <TOKEN>
```

`<ENDPOINT>` / `<TOKEN>` 从面板「节点 → 添加节点」处获取。

#### 6. 从官方 Agent 接管

主控升级**不会**改变已安装 Agent 的自更新来源，因此每个既有 Agent 需要执行一次接管：

```bash
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-agent-stable/stable/migrate-komari-agent.sh | sudo bash -s -- -y
```

该脚本会读取现有 systemd 单元中的**全部启动参数（endpoint / token / interval 等）原样保留**，
只替换二进制与更新来源，然后重启服务；最后打印 Agent 日志中的 `Github Repo:` 行以确认
来源已变为 `xinian5216/komari-agent-stable`。**不需要重新添加节点或重新生成 token。**

#### 7. 更新

```bash
# Server（systemd 安装）：再次执行迁移/升级动作
sudo bash /tmp/install-komari.sh --migrate --yes
# 或交互菜单：curl -fsSL <同上> | sudo bash → 选 2) 升级

# Server（Docker）：
docker compose pull && docker compose up -d
#   或 docker run：先 docker pull 新 tag，重建容器时必须保持同一挂载
#   （named volume 名或 bind mount 宿主机路径不变；必要时先 docker rename 旧容器以释放名称）
#   ⚠️ 更新过程中禁止误删持久化数据，具体是：
#      - 不要执行 `docker volume rm <卷名>`
#      - 不要执行 `docker compose down -v`（-v 会删除 compose 声明的 named volume）
#      - 不要把 bind mount 的宿主机目录当作临时目录清理
#   （注：`docker run -v` 是挂载参数，本身不是删除操作）

# Agent：自更新默认开启，无需手动操作；也可手动执行
sudo bash migrate-komari-agent.sh -y
```

#### 8. 回滚

```bash
# Server（systemd）：改回旧二进制（数据无需回滚，升级不改 Schema）
sudo systemctl stop komari
sudo cp /opt/komari/komari.backup.<时间戳> /opt/komari/komari
sudo systemctl start komari
# 如需回到指定版本（例如重装某个固定版本）：
sudo bash /tmp/install-komari.sh --migrate --yes --target-version v1.5.0-stable.0

# Server（Docker）：把 image tag 改回旧固定版本，用原挂载重建容器
# （compose：修改 compose.yaml 的 image tag 后 docker compose up -d；
#  docker run：用旧 tag 重新 run，挂载/端口与原来一致；若名称被占用先 docker rename 旧容器）
# 若上一轮迁移保留了旧容器：docker stop komari && docker rename komari komari-new-<日期> \
#   && docker rename komari-old-<日期> komari && docker start komari

# Agent：用备份二进制回滚
sudo systemctl stop komari-agent
sudo cp /opt/komari/agent.backup.<时间戳> /opt/komari/agent
sudo systemctl start komari-agent
```

> 服务端在检测到版本变化时还会**自动把整个 `data` 目录备份**到 `data/backup/upgrade-<时间戳>.zip`。

### 从源码构建

1. 准备构建期资产（**唯一入口**，幂等；版本与哈希只由 `bundled-themes.lock.json` 决定）：

```bash
./scripts/prepare-assets.sh all        # Windows: powershell -File scripts\prepare-assets.ps1 all
```

它会同时准备两样东西：

- `web/public/defaultTheme/` —— 嵌入式核心前端（自维护镜像 `xinian5216/komari-web-stable`，固定 tag）；
- `web/public/bundledTheme/next.zip` —— 随包首选主题 **Komari Next**（自维护镜像
  `xinian5216/komari-next-stable`，固定 tag，**校验 SHA256**）。

2. 构建后端（后端通过 `//go:embed` 嵌入上述产物，因此顺序不可颠倒）：

```bash
CGO_ENABLED=1 go build -o komari .   # 需要 C 编译器（跨平台编译可用 zig cc）
./komari server -l 0.0.0.0:25774
```

> 全新安装完成后，前台默认使用随包内嵌的 **Komari Next**；`/admin`、`/terminal` 与恢复页面始终使用
> 嵌入式核心前端。既有实例升级不受影响（详见 [`CHANGELOG.md`](./CHANGELOG.md) 与
> [`MAINTENANCE_POLICY.md`](./MAINTENANCE_POLICY.md) 的"发行版打包 / 默认行为调整"一节）。

### 组件与仓库

| 组件 | 本分支使用的仓库 | 上游来源 | 许可状态 |
| --- | --- | --- | --- |
| 服务端 + 默认主题载体 | **本仓库**（`xinian5216/komari-stable`） | `komari-monitor/komari`（已归档） | MIT（上游 `LICENSE` 原样保留） |
| 前端默认主题 | `xinian5216/komari-web-stable`（CI 固定 tag） | `komari-monitor/komari-web` | 上游根目录无 `LICENSE` 文件；依上游作者在其仓库内的明示为 MIT，取证见 [`komari-web-stable/LICENSE_AUDIT.md`](https://github.com/xinian5216/komari-web-stable/blob/stable/LICENSE_AUDIT.md) |
| 随包首选前台主题 | `xinian5216/komari-next-stable`（构建期按不可变 tag 固定，见 `bundled-themes.lock.json`） | `tonyliuzj/komari-next`（作者 Tony Liu） | MIT（上游 `LICENSE` 与 Credits 原样保留）；本 fork 只做稳定镜像与兼容维护 |
| Agent | `xinian5216/komari-agent-stable`（本 fork 维护的安装/更新通道；**v2 协议冻结、向后兼容**） | `komari-monitor/komari-agent` | MIT（上游 `LICENSE` 原样保留） |

### 文档索引

| 文件 | 内容 |
| --- | --- |
| [`BASELINE_AUDIT.md`](./BASELINE_AUDIT.md) | 基线审计报告（结构、构建/测试实测、风险地图、已知问题） |
| [`MAINTENANCE_POLICY.md`](./MAINTENANCE_POLICY.md) | 维护策略：什么能进 stable、什么不能、如何发布与回滚 |
| [`SECURITY.md`](./SECURITY.md) | 安全策略与漏洞报告方式 |
| [`CHANGELOG.md`](./CHANGELOG.md) | 每个版本的 Fixed / Security / Compatibility / Upgrade / Rollback |
| [`TECH_DEBT.md`](./TECH_DEBT.md) | 已知技术债（记录，不主动重构） |

---

## English

**Komari Stable** is a community-maintained stable fork of Komari. It tracks the last upstream release
(`1.5.0-fix1`, commit `0ca87aa`; upstream archived on 2026-09-15) and accepts **bug fixes, security
patches, compatibility and build/CI fixes only**. Database schema, the Agent v2 wire protocol, HTTP API
behavior, config format and Docker deployment are treated as **frozen**.

Versioning: `1.5.0-stable.N`. Branches: `upstream-baseline` (read-only mirror of upstream) and `stable`
(maintenance). Deploy and upgrade exactly like upstream; the server automatically backs up `data/` to
`data/backup/upgrade-<ts>.zip` when it detects a version change.

---

## Install / Migrate / Upgrade (English)

```bash
# fresh server install
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-stable/stable/install-komari.sh | sudo bash

# in-place migration from an official Komari install (keeps data/, databases,
# config, users, nodes, plugins, plugin-data and themes; backs everything up
# first and rolls back automatically if the new binary fails to start)
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-stable/stable/install-komari.sh -o /tmp/install-komari.sh
sudo bash /tmp/install-komari.sh --migrate --yes

# fresh agent install
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-agent-stable/stable/install.sh | sudo bash -s -- -e <ENDPOINT> -t <TOKEN>

# take over an existing official agent (keeps endpoint/token/interval, swaps the
# binary and the self-update source, then restarts the service)
curl -fsSL https://raw.githubusercontent.com/xinian5216/komari-agent-stable/stable/migrate-komari-agent.sh | sudo bash -s -- -y

# docker (no login required; anonymous pull verified)
docker run -d --name komari -p 25774:25774 -v komari-data:/app/data \
  --restart unless-stopped ghcr.io/xinian5216/komari-stable:stable
# pin a release tag for reproducible production deployments:
#   ghcr.io/xinian5216/komari-stable:v1.5.0-stable.0
# compose: see compose.yaml; update with `docker compose pull && docker compose up -d`;
# roll back by switching the image tag back and recreating the container (same data volume).
```

Migrate the agents first (they do not follow the panel automatically): take over every existing
agent, confirm `Github Repo:` shows `xinian5216/komari-agent-stable` and the node reports over v2,
then migrate the panel.

Note: server 1.5.0 and later speak only the v2 protocol. Agent-side v2 support landed in
**1.2.10** (opt-in) and became the only protocol in agent **1.5.0**, so make sure every node runs
agent **1.5.0 or newer** before migrating the panel - safest is to move the agents to
**v1.5.10-stable.0** first.

## Upstream & Credits / 上游与致谢

本分支的全部代码来自上游项目 **Komari**，版权归原作者与贡献者所有：

- Upstream repository: <https://github.com/komari-monitor/komari>（MIT License，已归档）
- Frontend: <https://github.com/komari-monitor/komari-web>
- Agent: 本分支维护 <https://github.com/xinian5216/komari-agent-stable>（源：<https://github.com/komari-monitor/komari-agent>）
- Documentation: <https://www.komari.wiki/> · 文档源：<https://github.com/komari-monitor/komari-document>
- Upstream README（原作者版）可在上游仓库或本仓库 git 历史（`main` 分支）中查看。

原项目的 `LICENSE`（MIT）与 `NOTICE` 原样保留，未做任何移除。

> 免责声明：Komari 是自托管监控/控制类应用，请仅在你有权管理的系统上部署使用。上游作者与本
> 分支维护者均不对使用方式及其后果承担责任。
