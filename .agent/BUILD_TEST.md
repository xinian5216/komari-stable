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

分支保护的固定检查名称与失败策略见 `.github/REQUIRED_CHECKS.md`。

| 目的 | 本地命令 | CI |
| --- | --- | --- |
| 测试 + vet | 第 3 节 | `stable-ci.yml` 的 `test` 作业 |
| 漏洞扫描 | `govulncheck` | `stable-ci.yml` 的 `vuln` 作业 |
| 多平台构建 | 第 2 节（换 CC target） | `stable-ci.yml` 的 `build` 作业 |
| 密钥扫描 | `gitleaks` | `secret-scan.yml` |
| 索引一致性 | `python scripts/check_agent_index.py` | `stable-ci.yml` 的 `agent-index` 作业 |
| 后台 SW E2E | 第 8 节 | `admin-sw-e2e.yml` |
| 发布（二进制 + Docker） | 第 2/5 节 | `stable-release.yml`（release published 触发） |

发布作业先从受保护的 `stable` 分支运行 `scripts/release-preflight.sh`：正式 tag 必须匹配
`vX.Y.Z-stable.N`、指向 `stable` 历史中的 commit，且该 commit 的 `required-gate`、
`admin-sw-gate`、`security-gate`、`secret-gate` 都必须由 GitHub Actions 成功完成。离线正反向自测：

```bash
bash scripts/tests/test-release-preflight.sh
```

## 8. 后台 Service Worker 浏览器 E2E

需要已准备的构建期资产、本机 `go build` 出的二进制，以及 Playwright Chromium：

```bash
python3 scripts/prepare-assets.py all
go build -trimpath -o komari .
cd e2e
npm ci
npx playwright install chromium
npx playwright test
```

Windows 把 `-o komari` 换成 `-o komari.exe`。测试会在临时目录做一次 fresh install，断言
`theme=next`、`/` 为 Next、`/admin/dashboard` 为嵌入式后台，且 Service Worker 不能把后台变成首页。

## 7. 原地迁移 E2E（官方 1.4.3 → Stable）

`.github/workflows/migration-test.yml` 必须用官方真实 `1.4.3` 二进制建立数据夹具，并验证：

1. 旧服务保持运行直到目标资产下载与校验结束；随后停机做离线归档；
2. 主库和指标库 `PRAGMA integrity_check`，以及用户、节点/token、配置、通知、Ping、插件、主题和
   指标 rollup 的具体值迁移前后相同（允许升级日志、版本标记等预期新增）；
3. 迁移后真实请求 `/ping` 与 `/api/version`，不能用伪造 active 标记代替服务健康；
4. 缺失校验和、错误校验和、目标进程秒退、API 不健康、版本不符、空间不足均 fail closed；
5. 回滚同时恢复二进制与离线 data，旧 1.4.3 能再次启动并读到原数据。

安装脚本、备份/版本逻辑或迁移 workflow 发生变化时必须运行该 E2E；发布固定资产仍由
`stable-release.yml` 的不可变守卫负责，绝不覆盖既有 tag。
