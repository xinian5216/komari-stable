# BUILD_TEST.md — 最短构建 / 测试路径

> 顺序是**硬性**的：**准备构建期资产 → 后端**。
> 原因：`web/public/public.go` 用 `//go:embed defaultTheme/dist.tar.zst` 与
> `defaultTheme/komari-theme.json` 嵌入核心前端，`web/public/bundled.go` 还用
> `//go:embed all:bundledTheme` 嵌入随包首选主题（Komari Next）—— 缺文件时 `go build` 直接失败。
> 资产的版本、来源与哈希**只由 `bundled-themes.lock.json` 决定**，禁止手工指定 main / latest。

## 0. 前置

| 组件 | 版本 |
| --- | --- |
| Go | **1.25.x**（`go.mod` 要求 ≥1.25.0） |
| C 编译器 | 必需（CGO：`mattn/go-sqlite3`）；跨平台编译用 **zig 0.14.1**（CI 同款） |
| Node | 22 或 23（前端构建） |
| zstd | 任意近期版本（打包前端产物） |

## 1. 准备构建期资产（**唯一入口**）

```bash
# 所有平台一致；脚本幂等：已存在且哈希正确的资产会直接复用
./scripts/prepare-assets.sh all          # Windows: powershell -File scripts\prepare-assets.ps1 all

./scripts/prepare-assets.sh frontend     # 只准备"嵌入式核心前端"（komari-web-stable 固定 tag）
./scripts/prepare-assets.sh theme        # 只准备"随包首选主题"（komari-next-stable 固定 tag + sha256 校验）
```

脚本（`scripts/prepare-assets.py`，`.sh` / `.ps1` 只是薄包装）会：

| 输出 | 来源（锁文件 `bundled-themes.lock.json`） |
| --- | --- |
| `web/public/defaultTheme/dist.tar.zst` + `komari-theme.json` | `xinian5216/komari-web-stable` @ 固定 tag：`npm ci` + `npm run build` + tar/zstd |
| `web/public/bundledTheme/next.zip` + `provenance.json` | `xinian5216/komari-next-stable` @ 固定 tag 的 `dist-release.zip`，**校验 SHA256** |

生成物不入 Git；`web/public/assets.provenance.json` 记录实际使用的来源与哈希。
任何 CI 步骤只要可能解析 `//go:embed`（`go test` / `go vet` / `go build` / `govulncheck`），
都必须先执行本入口 —— `stable-ci.yml` / `stable-release.yml` / `docker-preview.yml` 已内置。

> 主题包内 `komari-theme.json` 的 `url` 指向本 fork 的镜像仓库（供应链已接管）；
> `author` 与 MIT License / Credits 始终保持原作者（Tony Liu / `tonyliuzj`）不动。

## 2. 后端

```bash
export CGO_ENABLED=1
export CC="zig cc -target x86_64-windows-gnu"     # Windows；Linux 用 x86_64-linux-musl 等
go build -trimpath \
  -ldflags="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=<tag> \
            -X github.com/komari-monitor/komari/utils.VersionHash=<sha>" \
  -o komari .
```

## 3. 测试 / 静态检查 / 扫描（提交前必跑）

```bash
go vet ./...                       # 静态检查（CI 同款）
go test ./... -count=1             # 全量测试（约 1 分钟；pkg/metric 最重）
govulncheck ./...                  # 依赖漏洞（CI 同款；有"可达"漏洞时返回非 0）
gitleaks git . --log-opts="--all" --config .gitleaks.toml --redact   # 密钥扫描
actionlint .github/workflows/*.yml # workflow 语法
```

说明：PostgreSQL/MySQL/MariaDB 集成测试需 `METRIC_POSTGRES_DSN` 等环境变量，否则自动 skip。

## 4. 运行冒烟

```bash
mkdir -p /tmp/komari-smoke && cd /tmp/komari-smoke   # 不要在仓库目录里跑（会生成 ./data）
/path/to/komari server -l 127.0.0.1:25774
# 首启为安装模式：POST /api/install/complete {username,password,sitename,metric_dsn:"./data/metrics.db"}
# 完成后：/api/version、/、/api/public、POST /api/login 均应正常
```

## 5. Docker

```bash
# 需要预编译 linux 二进制放在仓库根（CI 就是这样做的）
docker build -t komari-stable:local .
docker run --rm -p 25774:25774 -v "$PWD/data:/app/data" komari-stable:local
```

## 6. CI 对应关系

| 目的 | 本地命令 | CI |
| --- | --- | --- |
| 测试 + vet | 第 3 节 | `stable-ci.yml` 的 `test` 作业 |
| 漏洞扫描 | `govulncheck` | `stable-ci.yml` 的 `vuln` 作业 |
| 多平台构建 | 第 2 节（换 CC target） | `stable-ci.yml` 的 `build` 作业 |
| 密钥扫描 | `gitleaks` | `secret-scan.yml` |
| 索引一致性 | `python scripts/check_agent_index.py` | `stable-ci.yml` 的 `agent-index` 作业 |
| 发布（二进制 + Docker） | 第 2/5 节 | `stable-release.yml`（release published 触发） |
