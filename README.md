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

### 版本与分支

| 分支 | 用途 |
| --- | --- |
| `upstream-baseline` | 上游官方基线（`1.5.0-fix1` / `0ca87aa`）的只读镜像，**永不修改** |
| `stable` | 长期维护分支，所有修复提交在这里 |

版本号格式：`1.5.0-stable.1`、`1.5.0-stable.2`…… 依次递增；除非出现大型功能变化，不会升级为 `1.6.0`。

### 部署与升级

部署方式与上游完全一致（Docker / 二进制 / 源码构建），升级只需替换二进制或镜像并保留 `data` 目录：

```bash
# Docker（示例，镜像名以 Releases 页面为准）
docker run -d -p 25774:25774 -v $(pwd)/data:/app/data --name komari --restart unless-stopped <image>:stable
```

- 服务端在检测到版本变化时，会**自动把整个 `data` 目录备份**到 `data/backup/upgrade-<时间戳>.zip`。
- 升级前仍建议自行备份 `data` 目录（含 `komari.db`、`metrics.db`）。
- 回滚：换回旧二进制/旧镜像 tag，并用上述备份恢复 `data` 目录。

### 从源码构建

1. 构建前端（本分支固定引用自维护的前端镜像：`xinian5216/komari-web-stable`，ref 固定为发版 tag）：

```bash
git clone https://github.com/xinian5216/komari-web-stable
cd komari-web-stable && npm install && npm run build
```

2. 打包主题并构建后端（**顺序不可颠倒**，后端通过 `//go:embed` 嵌入前端产物）：

```bash
mkdir -p web/public/defaultTheme
tar -cf /tmp/komari-dist.tar -C ../komari-web-stable/dist .
zstd -19 -T0 -f /tmp/komari-dist.tar -o web/public/defaultTheme/dist.tar.zst
cp ../komari-web-stable/komari-theme.json web/public/defaultTheme/

CGO_ENABLED=1 go build -o komari .   # 需要 C 编译器（跨平台编译可用 zig cc）
./komari server -l 0.0.0.0:25774
```

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

## Upstream & Credits / 上游与致谢

本分支的全部代码来自上游项目 **Komari**，版权归原作者与贡献者所有：

- Upstream repository: <https://github.com/komari-monitor/komari>（MIT License，已归档）
- Frontend: <https://github.com/komari-monitor/komari-web>
- Agent: <https://github.com/komari-monitor/komari-agent>
- Documentation: <https://www.komari.wiki/> · 文档源：<https://github.com/komari-monitor/komari-document>
- Upstream README（原作者版）可在上游仓库或本仓库 git 历史（`main` 分支）中查看。

原项目的 `LICENSE`（MIT）与 `NOTICE` 原样保留，未做任何移除。

> 免责声明：Komari 是自托管监控/控制类应用，请仅在你有权管理的系统上部署使用。上游作者与本
> 分支维护者均不对使用方式及其后果承担责任。
