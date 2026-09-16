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
               metric/ 指标存储引擎（30k 行，最大模块）；jsruntime/ goja JS 运行时（插件用）；
               rpc/ RPC 主体与 principal；timeutil/
protocol/v2/   Agent 线协议定义（JSON-RPC 2.0 方法与结构体，v1 已移除）
utils/         通用工具：log、notifier（通知调度）、messageSender（各渠道）、geoip、item、renewal、
               version（构建期注入的版本变量）
web/           HTTP 层（Gin）：
               router/ 路由注册；api/（admin/client/public/terminal/upload/backup/install/recovery/migration/oauth）；
               rpc/jsonrpc/ RPC2 方法实现；filemanager/ 文件管理；upload/ 分块上传；security/ CORS+Origin；
               connection/ 安全 WebSocket 封装；public/ 嵌入的前端主题与静态资源服务
web/public/    默认主题嵌入点（defaultTheme/dist.tar.zst + komari-theme.json，构建期生成，不入库）
.agent/        AI Agent 索引（本目录）
scripts/       维护脚本（如 .agent 索引一致性检查）
.github/       CI/CD：11 个上游 workflow + stable-ci / stable-release / secret-scan（本 fork 新增）
```

## 构建期目录（不入库）

```text
web/public/defaultTheme/   前端产物（tar+zstd），由 komari-web 构建后嵌入；见 BUILD_TEST.md
data/                      运行期数据目录（komari.db / metrics.db / plugin / theme / backup）
```

## 相关仓库（不在本仓库内）

| 仓库 | 用途 |
| --- | --- |
| `xinian5216/komari-web-stable` | 前端默认主题（本 fork 维护，CI 固定引用其 tag） |
| `xinian5216/komari-agent-stable` | Agent（本 fork 维护的镜像：安装/更新通道；**v2 协议冻结、向后兼容**） |
| `komari-monitor/komari-agent` | Agent 的上游来源（仍活跃，作为择优移植来源） |
| `komari-monitor/komari-protocol` | v1/v2 线协议冻结与冻结测试（对照基准） |
| `komari-monitor/komari-document` | 官方文档源（komari.wiki） |
