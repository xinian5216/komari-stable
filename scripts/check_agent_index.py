#!/usr/bin/env python3
"""Lightweight .agent/ index consistency check (CI + local).

做轻量、可从源码确定的校验，避免主观语义误报：
  1) .agent/ 下必需索引文件是否齐全；
  2) PROJECT_MAP.md 里提到的一级目录/文件是否真的存在；
  3) 仓库里出现的一级目录是否被 PROJECT_MAP.md 覆盖（只提示，不失败）。
  4) 少量容易漂移的事实：配置表名、capability 文档、迁移 Agent 最低版本。

用法：
    python scripts/check_agent_index.py            # 在仓库根执行
退出码：0 = 通过；1 = 有缺失项。
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

REQUIRED_AGENT_FILES = [
    "README.md",
    "PROJECT_MAP.md",
    "ARCHITECTURE.md",
    "MODULE_INDEX.md",
    "ENTRY_POINTS.md",
    "DATABASE.md",
    "API.md",
    "AGENT_PROTOCOL.md",
    "BUILD_TEST.md",
    "DEPENDENCY_MAP.md",
    "KNOWN_RISKS.md",
    "MAINTENANCE_RULES.md",
    "CHANGE_IMPACT_MAP.md",
]

# 仓库根下允许存在、但不需要写进 PROJECT_MAP 的目录
IGNORED_TOP_DIRS = {
    ".git", ".github", ".agent", "node_modules", "data", "dist", "build", "out", "tmp",
}


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for candidate in (here.parent, *here.parents):
        if (candidate / "go.mod").is_file():
            return candidate
    return Path.cwd()


def check_required(root: Path) -> list[str]:
    missing = [
        name for name in REQUIRED_AGENT_FILES
        if not (root / ".agent" / name).is_file()
    ]
    return missing


def project_map_targets(map_file: Path) -> list[str]:
    """从 PROJECT_MAP.md 首个 ```text 代码块中提取形如 `dir/` 或 `file.ext` 的条目。"""
    if not map_file.is_file():
        return []
    text = map_file.read_text(encoding="utf-8", errors="replace")
    block = re.search(r"```text\n(.*?)```", text, flags=re.S)
    if not block:
        return []
    targets: list[str] = []
    for line in block.group(1).splitlines():
        if not line.strip():
            continue
        # 只统计顶格（未缩进）的条目；缩进行是上一项的子项说明
        if line != line.lstrip():
            continue
        token = line.split()[0]
        if re.match(r"^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*/?$", token):
            targets.append(token.rstrip("/") or token)
    return targets


def check_targets(root: Path, targets: list[str]) -> list[str]:
    missing = []
    for target in targets:
        if not (root / target).exists():
            missing.append(target)
    return missing


def check_source_facts(root: Path) -> list[str]:
    """只检查能从同仓库源码直接证明、且曾发生过文档漂移的事实。"""
    errors: list[str] = []
    agent_dir = root / ".agent"
    all_docs = "\n".join(
        path.read_text(encoding="utf-8", errors="replace")
        for path in agent_dir.glob("*.md")
    )

    config_source = (root / "internal/config/config.go").read_text(
        encoding="utf-8", errors="replace"
    )
    if 'return "configs"' in config_source and "config_items" in all_docs:
        errors.append(".agent 文档仍把当前物理配置表写成 config_items；实际 TableName() 为 configs")

    protocol_source = (root / "protocol/v2/jsonrpc.go").read_text(
        encoding="utf-8", errors="replace"
    )
    protocol_doc = (agent_dir / "AGENT_PROTOCOL.md").read_text(
        encoding="utf-8", errors="replace"
    )
    if "Capabilities" in protocol_source:
        for required in ("capabilities", "privilege_level", "remote_control_known"):
            if required not in protocol_doc:
                errors.append(f"AGENT_PROTOCOL.md 未记录当前协议事实：{required}")

    installer = (root / "install-komari.sh").read_text(
        encoding="utf-8", errors="replace"
    )
    readme = (root / "README.md").read_text(encoding="utf-8", errors="replace")
    match = re.search(r'^V2_MIN_AGENT="([^"]+)"', installer, flags=re.M)
    if match and f"**{match.group(1)} or newer**" not in readme and f"**{match.group(1)}**" not in readme:
        errors.append(
            "install-komari.sh 的 V2_MIN_AGENT 与 README 中建议的迁移最低版本不一致"
        )

    return errors


def main() -> int:
    root = repo_root()
    failed = False

    missing_files = check_required(root)
    if missing_files:
        failed = True
        print("FAIL: 缺失 .agent/ 索引文件：")
        for name in missing_files:
            print(f"  - .agent/{name}")
    else:
        print(f"OK: .agent/ 必需文件齐全（{len(REQUIRED_AGENT_FILES)} 个）")

    targets = project_map_targets(root / ".agent" / "PROJECT_MAP.md")
    missing_targets = check_targets(root, targets)
    if missing_targets:
        failed = True
        print("FAIL: PROJECT_MAP.md 提到的路径不存在：")
        for target in missing_targets:
            print(f"  - {target}")
    else:
        print(f"OK: PROJECT_MAP.md 的 {len(targets)} 个条目均存在")

    # 提示级：仓库里的一级目录是否被 PROJECT_MAP 覆盖
    actual = {
        p.name for p in root.iterdir()
        if p.is_dir() and p.name not in IGNORED_TOP_DIRS and not p.name.startswith(".")
    }
    covered = {t.split("/")[0] for t in targets}
    uncovered = sorted(actual - covered)
    if uncovered:
        print("NOTE: 一级目录未在 PROJECT_MAP.md 中登记（仅提示）：" + ", ".join(uncovered))

    fact_errors = check_source_facts(root)
    if fact_errors:
        failed = True
        print("FAIL: .agent/ 与源码关键事实不一致：")
        for error in fact_errors:
            print(f"  - {error}")
    else:
        print("OK: 配置表、Agent capability 与迁移版本事实一致")

    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
