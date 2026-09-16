# BUILD_TEST.md — 最短构建 / 测试路径

> 顺序是**硬性**的：**前端 → 主题打包 → 后端**。
> 原因：`web/public/public.go` 用 `//go:embed defaultTheme/dist.tar.zst` 与
> `defaultTheme/komari-theme.json` 把前端产物嵌进二进制 —— 缺文件时 `go build` 直接失败。

## 0. 前置

| 组件 | 版本 |
| --- | --- |
| Go | **1.25.x**（`go.mod` 要求 ≥1.25.0） |
| C 编译器 | 必需（CGO：`mattn/go-sqlite3`）；跨平台编译用 **zig 0.14.1**（CI 同款） |
| Node | 22 或 23（前端构建） |
| zstd | 任意近期版本（打包前端产物） |

## 1. 前端（独立仓库）

```bash
# fork 维护的前端镜像；ref 必须是固定 tag/commit（CI 用 v1.5.0-stable.0）
git clone https://github.com/xinian5216/komari-web-stable
cd komari-web-stable && npm install && npm run build
```

## 2. 打包主题（在**后端**仓库根执行）

```bash
mkdir -p web/public/defaultTheme
tar -cf /tmp/komari-dist.tar -C ../komari-web-stable/dist .
zstd -19 -T0 -f /tmp/komari-dist.tar -o web/public/defaultTheme/dist.tar.zst
cp ../komari-web-stable/komari-theme.json web/public/defaultTheme/
# 自检：压缩包内必须含 index.html
tar -tf <(zstd -dc web/public/defaultTheme/dist.tar.zst) | grep -q '^./index.html' && echo OK
```

> Windows/git-bash 注意：GNU tar 遇到 Windows 盘符路径（形如 `<盘符>:/…`）会把盘符当成远程主机，
> 需要加 `--force-local`（或改用 MSYS 风格路径）。

## 3. 后端

```bash
export CGO_ENABLED=1
export CC="zig cc -target x86_64-windows-gnu"     # Windows；Linux 用 x86_64-linux-musl 等
go build -trimpath \
  -ldflags="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=<tag> \
            -X github.com/komari-monitor/komari/utils.VersionHash=<sha>" \
  -o komari .
```

## 4. 测试 / 静态检查 / 扫描（提交前必跑）

```bash
go vet ./...                       # 静态检查（CI 同款）
go test ./... -count=1             # 全量测试（约 1 分钟；pkg/metric 最重）
govulncheck ./...                  # 依赖漏洞（CI 同款；有"可达"漏洞时返回非 0）
gitleaks git . --log-opts="--all" --config .gitleaks.toml --redact   # 密钥扫描
actionlint .github/workflows/*.yml # workflow 语法
```

说明：PostgreSQL/MySQL/MariaDB 集成测试需 `METRIC_POSTGRES_DSN` 等环境变量，否则自动 skip。

## 5. 运行冒烟

```bash
mkdir -p /tmp/komari-smoke && cd /tmp/komari-smoke   # 不要在仓库目录里跑（会生成 ./data）
/path/to/komari server -l 127.0.0.1:25774
# 首启为安装模式：POST /api/install/complete {username,password,sitename,metric_dsn:"./data/metrics.db"}
# 完成后：/api/version、/、/api/public、POST /api/login 均应正常
```

## 6. Docker

```bash
# 需要预编译 linux 二进制放在仓库根（CI 就是这样做的）
docker build -t komari-stable:local .
docker run --rm -p 25774:25774 -v "$PWD/data:/app/data" komari-stable:local
```

## 7. CI 对应关系

| 目的 | 本地命令 | CI |
| --- | --- | --- |
| 测试 + vet | 第 4 节 | `stable-ci.yml` 的 `test` 作业 |
| 漏洞扫描 | `govulncheck` | `stable-ci.yml` 的 `vuln` 作业 |
| 多平台构建 | 第 3 节（换 CC target） | `stable-ci.yml` 的 `build` 作业 |
| 密钥扫描 | `gitleaks` | `secret-scan.yml` |
| 索引一致性 | `python scripts/check_agent_index.py` | `stable-ci.yml` 的 `agent-index` 作业 |
| 发布（二进制 + Docker） | 第 3/6 节 | `stable-release.yml`（release published 触发） |
