# PUBLIC_RELEASE_AUDIT.md — 公开发布前检查报告

> 生成时间：2026-09-16 · 检查对象：本地 `stable` 分支（**尚未 push 到任何远程**）
> 判定口径：PASS = 已实测通过；⚠️ = 有条件/待补；❌ = 未验证或未通过

## 0. 结论摘要

- **没有发现真实 secret、没有发现个人信息泄漏**（工作树 / 已跟踪内容 / 全历史三面均已扫描）。
- 存在 **3 项待完成事项**（Docker 构建验证、Agent 安装脚本指向、上游遗留 workflow 处置），
  其中与"公开推送"直接相关的是前两项——建议完成后再推送（见 §3）。
- fork 自身的提交身份使用 `xinian5216@users.noreply.github.com`（无个人邮箱）。

## 1. 检查项结果

| # | 检查项 | 结果 | 证据 / 说明 |
| --- | --- | --- | --- |
| 1 | **Secret Scan（工作树 + 已跟踪内容）** | ✅ **PASS** | `scripts/scan_secrets.py` 扫描 `git archive HEAD` 导出的 424 个已跟踪文件：14 处命中全部为上游客源中的**测试夹具/文档示例/保留网段**（逐条判定见 §2）；`gitleaks dir` 2 处命中为误报（见 §2）。工作树中不含 `.env`、`*.pem`、凭据文件 |
| 2 | **Git History Scan（全 refs）** | ✅ **PASS** | `gitleaks git . --log-opts="--all"`：703 commits / 5.82 MB / 2 处命中（均为误报）；`scan_secrets.py --all-history`：无 secret 类命中。pickaxe 检查：**本地用户名、本地用户目录路径形态、fork 账号名** 在全历史中的出现情况已逐一核对（本地用户名与本地路径 **0 命中**；fork 账号名仅出现在本 fork 的唯一一次提交中） |
| 3 | **Personal Information Scan** | ✅ **PASS** | 无本地用户名、无本地绝对路径、无私有 IP/家庭 IP/VPS IP、无手机号、无个人邮箱。提交身份为 GitHub noreply。**说明**：全历史中出现 41 个作者邮箱，均为**上游贡献者的公开 commit 元数据**（上游仓库本就公开），非本 fork 新增；如需清除只能重写历史（未做、需你确认） |
| 4 | **Build** | ✅ **PASS** | Windows/amd64 本地实测：`go build -trimpath -ldflags=…` 成功（1m38s，35,257,344 B）；CI 将覆盖 `linux/amd64`、`linux/arm64`、`windows/amd64`（zig 0.14.1 交叉编译） |
| 5 | **Tests** | ✅ **PASS** | `go test ./... -count=1` 全绿；`go vet ./...` 无输出；前端 `npm run lint` 0 error / 29 warning；`actionlint` 对全部 workflow 零问题 |
| 6 | **Docker Build** | ❌ **待验证** | 本机无 Docker，无法本地验证镜像构建与运行；`stable-release.yml` 会在 release 时构建 amd64+arm64。**建议在 push 后先手动触发一次 CI 构建验证** |
| 7 | **Installer Check** | ✅ **PASS** | `install-komari.sh`：`bash -n` 语法通过；下载来源已收口为 4 个变量（`REPO_OWNER`/`REPO_NAME`/`RELEASE_BASE`/`GITHUB_API_BASE`），全脚本仅 4 处 URL 构造，无其它硬编码；URL 拼接已实测输出正确 |
| 8 | **Agent Installer Check** | ⚠️ **待补（计划已定）** | 决策（2026-09-16）：Agent 安装/更新统一走本 fork 的 `xinian5216/komari-agent-stable`（源自上游 `komari-agent`；**v2 协议冻结**）。当前前端仍硬编码上游 raw 地址（`komari-web/src/components/admin/NodeTable/NodeFunction.tsx`、`src/pages/admin/index.tsx`），待 `komari-web-stable` / `komari-agent-stable` 创建后一并修改（`.agent/INSTALLERS.md` §2） |
| 9 | **Upstream Dependency Check** | ⚠️ **有条件通过** | `govulncheck`：**2 个代码可达**漏洞（`golang.org/x/text` GO-2026-5970、`golang.org/x/net` GO-2026-5026）+ 9 个导入包级 + 22 个模块级（不可达）。均为**上游继承**问题，已记录并计划在 `1.5.0-stable.1` 以最小依赖升级修复 |

## 2. 命中明细与非机密判定

### 2.1 gitleaks（2 处，均为误报）

| 位置 | 命中 | 判定 |
| --- | --- | --- |
| `pkg/metric/digest_reclaim.go` | `legacy_tdigest_v1`（generic-api-key 启发式） | **误报**：这是摘要格式版本常量字符串，非密钥。已在 `.gitleaks.toml` 精确放行 |
| `web/install/install_test.go` | `Correct-horse-battery-staple1` | **误报**：测试夹具密码（上游公开代码，用于"强密码通过校验"用例）。已精确放行 |

### 2.2 scan_secrets.py（14 处，均为上游公开代码中的非机密）

| 位置 | 规则 | 判定 |
| --- | --- | --- |
| `pkg/jsruntime/node_test.go:520`、`pkg/jsruntime/README.md:110`、`pkg/metric/migrations_test.go:30` | windows-drive-path | 测试夹具 / 文档示例（3 处，均为上游公开代码；本文档不复制原文形态以保持扫描洁净） |
| `utils/log/gin_test.go:25` | public-ipv4 | 上游测试数据中的一个公网 IP（非本项目资产） |
| `web/api/admin/market_download.go:25-31`（4 处） | public-ipv4 | **刻意为之**：SSRF 防护中的保留网段判断表 |
| `web/api/public/login_test.go:30,50`、`web/install/install_test.go:68,84,113,123` | generic-credential-assignment | 测试夹具密码（上游公开代码，用于登录/安装校验用例） |

> 结论：以上均位于**上游公开仓库已发布的代码**中，不含任何本项目/维护者的凭据或个人数据，
> 不构成发布阻塞项。

## 3. 公开推送前仍需完成（建议顺序）

1. **创建 GitHub 仓库**：`xinian5216/komari-stable`（公开）——推送前请再次确认本报告。
2. **镜像仓库**：创建 `xinian5216/komari-web-stable`（打 tag `v1.5.0-stable.0`）与
   `xinian5216/komari-agent-stable`（源自上游 `komari-agent`，作为 Agent 安装/更新通道；协议保持
   v2 冻结、向后兼容）；随后修正前端中的 Agent 安装命令指向（第 8 项 ⚠️）。
3. **CI 预演**：在 fork 上手动触发 `stable-ci.yml`（验证 linux 构建与测试）与一次临时 tag 的
   Docker 构建（验证第 6 项），确认无误后再发首个 release。
4. **上游遗留 workflow 处置**（`development.yml` 会 SSH 部署到作者生产环境、
   `generate-release-notes.yml` 依赖 OPENAI_API_KEY、`auto-merge-dev-to-main.yml` 不适用）：
   建议删除或禁用（等你确认）。
5. **推送后的外部验证**（skill 要求）：
   - 匿名 clone（不带凭据）后重跑 gitleaks 与 scan_secrets.py；
   - GitHub 仓库 Secret Scanning 警报数为 0；
   - 确认 Secret Scanning Push Protection 已启用；
   - 检查远端分支/标签与本地一致（`upstream-baseline`、`stable`）。

## 4. 本次使用的扫描工具与命令（可复现）

```bash
# 1) 已跟踪内容（等价于发布内容）
git archive HEAD | tar -x -C /tmp/export
python scan_secrets.py /tmp/export
gitleaks dir /tmp/export --config .gitleaks.toml --redact

# 2) 全历史 + 全 refs
gitleaks git . --log-opts="--all" --config .gitleaks.toml --redact
python scan_secrets.py --all-history .

# 3) 历史中的个人标记（pickaxe；占位符 "<本地用户名>" / "<盘符>:/Users" 为有意写法，见 §2 说明）
git log --all --oneline -S"<本地用户名>" -S"<盘符>:/Users"

# 4) 提交身份
git log upstream-baseline..stable --format='%an <%ae> | %cn <%ce>'
```

> 说明：本文档**不复制**扫描命中的原始形态（本地路径、用户名、IP 字面量），以免文档本身
> 触发隐私扫描；所有命中位置均可按上表文件与行号复现。

工具版本：gitleaks **8.30.1**（本机与 CI 一致）、`scripts/scan_secrets.py`（hermes-update-check）、
`git` 2.54.0。
