# SECURITY_RESPONSE.md — Komari Stable 安全响应手册

本手册适用于以下四个仓库：

- Server：`xinian5216/komari-stable`
- Agent：`xinian5216/komari-agent-stable`
- Web：`xinian5216/komari-web-stable`
- Next：`xinian5216/komari-next-stable`

本项目为志愿维护，不承诺商业 SLA；下列时间是 best-effort 的内部响应目标。

## 1. 报告与保密

1. 优先使用受影响仓库的 **Security → Report a vulnerability**，创建 Private Security Advisory。
2. 不在公开 Issue、普通 fork、PR 标题、提交信息或 Actions 日志中粘贴未公开 PoC、真实 Token、账号、内网地址。
3. 跨仓问题以 Server 仓的 Advisory 为协调记录，组件仓仅保留必要的私密关联。
4. 公开 Issue 只能记录去敏后的现象和处理状态，不能包含可直接利用的细节。

报告至少应包含：受影响版本、入口与权限前提、最小复现、最坏影响、已知缓解方式。

## 2. 分级

| 等级 | 典型条件 | 响应目标 |
| --- | --- | --- |
| P0 | 已在野利用；公网可达认证绕过、RCE、任意文件读写；高危供应链投毒 | 4 小时内确认，24 小时内给出临时缓解，72 小时内力争发布 |
| P1 | 可达高危但暂无在野利用证据 | 48 小时内完成可达性判断，7 天内修复 |
| P2 | 中低危或当前不可达 | 记录证据和复查条件，进入正常维护窗口 |

可达性判断必须记录：入口、调用路径、所需身份、默认配置是否受影响、受影响版本和测试证据。

## 3. 私密修复流程

1. 在 Security Advisory 的临时私有 fork 中创建最小修复分支。
2. 先加入失败测试或去武器化复现夹具。
3. 修复只处理本漏洞，不夹带重构、功能开发或无关依赖升级。
4. P0 可先做 containment：注销路由、拒绝请求、关闭功能或限制来源，再完成根因修复。
5. 运行与普通 PR 相同的测试、构建、迁移、扫描和发布守卫；紧急不等于绕过 CI。
6. 更新 CHANGELOG 的 Security、Compatibility、Upgrade、Rollback。
7. 跨仓修复必须记录通过验证的 Server/Web/Agent/Next 版本组合。

## 4. 发布与披露

1. 只从受保护的 `stable` 历史创建新的不可变版本 tag。
2. 不移动旧 tag，不覆盖 Release 资产或固定镜像 tag。
3. 发布二进制/主题包、SHA256SUMS、SBOM 与构建证明。
4. 先验证匿名安装、旧数据升级、Agent 上报和回滚，再公开 Advisory。
5. 公告应包含：受影响版本、修复版本、升级方法、临时缓解、兼容影响、回滚方法；PoC 仅披露到足以帮助用户确认风险的程度。

## 5. 自动告警处理

- Dependabot、CodeQL、govulncheck、npm audit 的结果以 GitHub Security/Code scanning 为准。
- 不自动把漏洞详情写入公开 Issue。
- 若需公开跟踪，只创建去敏摘要并回链私密 Advisory。
- 不为“清零告警”运行 `npm audit fix --force`、`go get -u ./...` 或无边界大版本升级。
- 不可达告警也要记录工具版本、不可达证据和重新评估条件。

## 6. 演练

至少每 6 个月，或在 Ruleset/Release workflow 大改后，用无害模拟漏洞走完：

`Private Advisory → 私有修复 → CI → 测试 tag → 安装/升级验证 → 回滚 → 去敏复盘`

演练不得使用真实生产密钥、真实用户数据或可公开滥用的 PoC。