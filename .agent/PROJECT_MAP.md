# PROJECT_MAP.md — 一级目录职责

> 只做索引，不含实现细节。行数约为 2026-09 基线（1.5.0-fix1）规模。

```text
main.go        程序入口（打印版本 → cmd.Execute）
cmd/           CLI 层：Cobra 命令（server / chpasswd / disable2FA / permitPasswordLogin）、全局 flag
database/      GORM 数据层：dbcore（连接/迁移/升级备份）、models（表结构）、accounts（账号/会话/2FA）、
               clients（节点）、records（旧记录 DTO）、tasks（ping 任务）、notification、auditlog、clipboard
internal/      服务端内部逻辑（不对外复用）：
               server/ 启动生命周期与路由装配；config/ 配置项定义；migrations/ 一次性历史迁移；
               metricstore/ 指标库存取与生命周期；plugin/ 插件宿主；scheduler/ 定时任务；
               lifecycle/ 重启；managedconfig/ 主题受管配置；sqlitetune/ SQLite 调优
pkg/           可复用库：
               metric/ 指标存储引擎（最大模块，规模会随维护变化）；jsruntime/ goja JS 运行时（插件用）；
               rpc/ RPC 主体与 principal；timeutil/
protocol/v2/   Agent 线协议定义（JSON-RPC 2.0 方法与结构体，v1 已移除）
utils/         通用工具：log、notifier（通知调度）、messageSender（各渠道）、geoip、item、renewal、
               version（构建期注入的版本变量）
web/           HTTP 层（Gin）：
               router/ 路由注册（含已移除远控路径的 410 tombstone）；
               api/（admin/client/public/upload/backup/install/recovery/migration/oauth）；
               rpc/jsonrpc/ RPC2 方法实现；upload/ 本地分块上传；security/ CORS+Origin；
               connection/ 安全 WebSocket 封装；public/ 嵌入的前端主题与静态资源服务
web/public/    构建期资产嵌入点：
               defaultTheme/  嵌入式核心前端（dist.tar.zst + komari-theme.json，构建期生成，不入库）
               bundledTheme/  随包首选主题（next.zip + provenance.json，构建期生成，不入库）；
                              仅含 README.md 时也可编译（运行时跳过 seed）
internal/themebundle/   随包主题包的校验与安全解压（sha256/根 manifest/short/上限/穿越/符号链接）
internal/bundledtheme/  把随包主题 seed 到 data/theme/<short> 的胶水（只在全新安装路径被调用）
bundled-themes.lock.json  构建期资产的唯一事实来源（repository/tag/commit/asset/sha256）
.agent/        AI Agent 索引（本目录）
scripts/       维护脚本（.agent 索引一致性、远控移除守卫、prepare-assets.py 构建期资产准备）
e2e/           Playwright 浏览器 E2E（fresh install + Next 首页 vs `/admin` + Service Worker）
.github/       CI/CD：11 个上游 workflow + stable-ci / stable-release / secret-scan（本 fork 新增）
```

## 构建期目录（不入库）

```text
web/public/defaultTheme/   前端产物（tar+zstd），由 komari-web 构建后嵌入
web/public/bundledTheme/   随包主题包（next.zip + provenance.json），由 prepare-assets 下载校验后嵌入
web/public/assets.provenance.json  两个构建期资产的来源与哈希记录（不入库）
data/                      运行期数据目录（komari.db / metrics.db / plugin / theme / backup）
```

## 相关仓库（不在本仓库内）

| 仓库 | 用途 |
| --- | --- |
| `xinian5216/komari-web-stable` | 嵌入式核心前端（本 fork 维护，CI 固定引用其 tag） |
| `xinian5216/komari-agent-stable` | Agent（本 fork 维护的镜像：安装/更新通道；**v2 协议冻结、向后兼容**） |
| `xinian5216/komari-next-stable` | **Bundled preferred frontend theme**（Komari Next 的稳定镜像；Server 按不可变 tag 固定其 `dist-release.zip`，见 `bundled-themes.lock.json`） |
| `komari-monitor/komari-agent` | Agent 的上游来源（仍活跃，作为择优移植来源） |
| `komari-monitor/komari-protocol` | v1/v2 线协议冻结与冻结测试（对照基准） |
| `komari-monitor/komari-document` | 官方文档源（komari.wiki） |
