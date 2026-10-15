# Komari Stable 插件源

一个固定源地址，汇总两个分别维护的插件仓库：

```text
https://raw.githubusercontent.com/xinian5216/komari-stable/stable/plugin-market/v1.json
```

在管理后台「插件市场 → 管理源」添加，名称可填 `Komari Stable`。

| 插件 | 独立维护仓库 | 插件标识 |
| --- | --- | --- |
| Komari DDNS | [komari-ddns-stable](https://github.com/xinian5216/komari-ddns-stable) | `cf-ddns` |
| 通行密钥 | [komari-passkey-stable](https://github.com/xinian5216/komari-passkey-stable) | `komari-passkey` |

作者署名保留为原作者。此源由 xinian5216 独立维护，不属于官方插件市场。

## 安装与迁移

1. 备份 Komari 数据库及 `data/plugin-data/cf-ddns/`、`data/plugin-data/komari-passkey/`。
2. 添加并启用此源，从对应的 Stable 卡片安装或更新；不要先卸载，也不要删除插件数据。
3. 原标识、配置键、记录格式与 Passkey 凭据格式保持不变。批准插件权限后，验证 DDNS 和通行密钥登录；验证前保留可用的密码登录方式。
4. 对这两个插件使用本源维护版时，建议关闭官方市场源，保留其配置以备需要。当前后台按插件标识判断是否安装；从官方源重装同标识插件会覆盖维护版。关闭源不会卸载已经安装的插件。

私有站点可能拦截上游 Passkey 匿名登录接口。此问题没有通过放开全部 API 来绕过；确认部署兼容后再将 Passkey 作为日常登录方式。

这里仅发布目录与说明，不改变 Server 的默认市场地址，不自动修改任何已部署实例，也不需要为此重新编译或升级 Server。

## 更新规则

- 先在各插件仓库审核上游差异，测试、扫描并发布自己的固定版本。
- 发布包必须通过类型检查、回归测试、真实 Komari Stable Goja 运行时检查、依赖与密钥扫描。
- 确认 GitHub 实际资产 SHA-256 后，才更新本目录相应条目的版本、下载地址和哈希。
- 一次只改需要更新的插件。禁止指向 `main`、`latest` 或未经检查的临时包。
- 已发布 tag 和资产不得覆盖。回滚通过经校验的旧包完成，不删除凭据或记录。
- 自建源不会缩小插件的管理员 RPC 权限。DDNS Token、数据库及备份仍需保护。

本地检查：`node plugin-market/validate.cjs`。在提供 `unzip` 的环境中增加 `--remote` 可下载正式资产，核验大小、SHA-256、ZIP 根 manifest、插件版本和最小 Komari 要求。
