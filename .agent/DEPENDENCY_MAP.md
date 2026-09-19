# DEPENDENCY_MAP.md — 关键依赖（只列重要的）

> 完整清单见 `go.mod` / `go.sum`。依赖**升级策略**见 `MAINTENANCE_POLICY.md` §8：
> 只做安全驱动的升级，一次一个模块族，单独提交与发布。

## 核心框架

| 依赖 | 用途 | 备注 |
| --- | --- | --- |
| `github.com/gin-gonic/gin` | HTTP 框架 | 路由、中间件、静态服务 |
| `gorm.io/gorm` + `gorm.io/driver/sqlite` | ORM | 主库；AutoMigrate 建表 |
| `github.com/spf13/cobra` | CLI | `cmd/*` |
| `github.com/gorilla/websocket` | WebSocket | Agent v2、前端实时流 |
| `github.com/google/uuid` | UUID | 节点/用户标识 |

## 构建期资产（不是 Go 依赖，但决定 `//go:embed` 能否编译）

| 资产 | 来源（唯一事实来源：`bundled-themes.lock.json`） | 产物 |
| --- | --- | --- |
| 嵌入式核心前端 | `xinian5216/komari-web-stable` @ 固定 tag（构建：`npm ci` + `npm run build` + tar/zstd） | `web/public/defaultTheme/dist.tar.zst` |
| 随包首选主题 | `xinian5216/komari-next-stable` @ 固定 tag 的 `dist-release.zip`（**校验 sha256**） | `web/public/bundledTheme/next.zip` |

准备入口只有一个：`scripts/prepare-assets.py`（`all` / `frontend` / `theme`；幂等，哈希正确即复用）。
CI 中所有会解析 `//go:embed` 的命令（`go test` / `go vet` / `go build` / `govulncheck`）之前都必须先准备资产。

## 数据库

| 依赖 | 用途 |
| --- | --- |
| `github.com/mattn/go-sqlite3` | SQLite 驱动（**CGO 依赖**，跨平台构建需 C 编译器/zig） |
| `github.com/jackc/pgx/v5` | PostgreSQL 驱动（指标库可选） |
| `github.com/go-sql-driver/mysql` | MySQL/MariaDB 驱动（指标库可选） |

## 认证 / 安全

| 依赖 | 用途 |
| --- | --- |
| `github.com/pquerna/otp` | TOTP 2FA |
| `golang.org/x/crypto` | ssh/加密原语（历史上有 CVE，`go.mod` 内有升级注释） |
| `golang.org/x/net`、`golang.org/x/text` | 网络/文本（govulncheck 关注对象） |

## 插件系统 / JS 运行时

| 依赖 | 用途 |
| --- | --- |
| `github.com/dop251/goja` + `goja_nodejs` | 在宿主内运行插件 JS（`pkg/jsruntime` 封装 Node 风格 API） |
| `github.com/klauspost/compress` | zstd（嵌入主题解压） |

## Agent 协议

| 位置 | 用途 |
| --- | --- |
| `protocol/v2/*`（本仓库） | 线协议结构与方法名（冻结） |
| `xinian5216/komari-agent-stable`（本 fork 镜像仓库） | Agent 二进制及其安装/更新通道；协议仍为冻结 v2 |
| `komari-monitor/komari-agent`（上游来源） | 择优移植来源（仅作参考） |
| `komari-monitor/komari-protocol`（外部仓库） | 冻结对照 + freeze tests |

## 其它服务端

| 依赖 | 用途 |
| --- | --- |
| `github.com/oschwald/maxminddb-golang` | mmdb GeoIP 读取（`utils/geoip`） |
| `github.com/patrickmn/go-cache` | 内存缓存 |
| `golang.org/x/sync` | 并发原语 |
| `github.com/stretchr/testify` | 测试断言 |

## 前端（独立仓库 `komari-web`）

React 19 + Vite + TypeScript + Radix UI + Tailwind 4 + recharts；构建产物以 tar+zstd 嵌入后端。
⚠️ 前端 repo 的默认分支会漂移（现为 `radix`），**必须固定 tag/commit**（本 fork 固定自维护镜像）。

## 构建工具链（不入 go.mod）

| 工具 | 版本 | 用途 |
| --- | --- | --- |
| Go | 1.25.x | 编译 |
| zig | 0.14.1 | CGO 交叉编译（linux/windows 全平台矩阵） |
| Node | 22/23 | 前端构建 |
| zstd | 任意近期版 | 主题打包 |
| gitleaks | 8.30.1 | 密钥扫描（本地 + CI） |
| actionlint | 1.7.x | workflow 校验 |

## 已知依赖问题（详见 TECH_DEBT.md）

- `golang.org/x/text@v0.33.0`、`golang.org/x/net@v0.41.0`：govulncheck **代码可达**漏洞；
- `golang.org/x/crypto@v0.39.0`、`github.com/klauspost/compress@v1.17.11`：当前不可达，随安全升级处理；
- `github.com/golang/protobuf`：已弃用（间接依赖）。
