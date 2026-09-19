#!/usr/bin/env python3
"""Fail CI if Server-side remote control is reintroduced."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def main() -> int:
    errors: list[str] = []

    for relative in (
        "web/rpc/jsonrpc/admin.file.go",
        "web/rpc/jsonrpc/admin.task.go",
        "web/rpc/jsonrpc/admin.xtermjs.go",
        "database/tasks/tasks.go",
    ):
        if (ROOT / relative).exists():
            errors.append(f"removed implementation exists: {relative}")

    for relative in ("web/api/terminal", "web/filemanager"):
        directory = ROOT / relative
        if directory.exists() and any(path.is_file() for path in directory.rglob("*")):
            errors.append(f"removed implementation directory is not empty: {relative}")

    config_source = (ROOT / "internal/config/settings.go").read_text(encoding="utf-8")
    if "XtermjsSettingsKey" in config_source or '"xtermjs_settings"' in config_source:
        errors.append("removed xterm.js setting is still defined")

    event_source = (ROOT / "web/agent/v2_events.go").read_text(encoding="utf-8")
    if "func EnqueueV2Event(" in event_source:
        errors.append("generic event queue bypass is exported")

    registrations = {
        "exec",
        "getTasks",
        "getTaskById",
        "getTasksByClientId",
        "getSpecificTaskResult",
        "getTaskResultsByTaskId",
        "fileList",
        "fileListRoots",
        "fileStat",
        "fileMkdir",
        "fileDelete",
        "fileMove",
        "fileCopy",
        "fileChmod",
        "fileChown",
        "fileSearch",
        "getXtermjsSettings",
        "setXtermjsSettings",
    }
    rpc_source = "\n".join(
        path.read_text(encoding="utf-8", errors="replace")
        for path in (ROOT / "web/rpc/jsonrpc").glob("*.go")
        if not path.name.endswith("_test.go")
    )
    for method in sorted(registrations):
        if re.search(rf'(?:reg|RegisterWithGroupAndMeta)\(\s*"{re.escape(method)}"', rpc_source):
            errors.append(f"removed RPC method is registered: admin:{method}")

    router_source = (ROOT / "web/router/router.go").read_text(encoding="utf-8")
    for forbidden in ("web/api/terminal", "web/filemanager", "terminal.", "filemanager."):
        if forbidden in router_source:
            errors.append(f"router references removed implementation: {forbidden}")
    for tombstone in (
        "/terminal",
        "/api/clients/terminal",
        "/api/clients/transfer/:id",
        "/api/admin/task/*path",
        "/api/admin/client/:uuid/file/*path",
    ):
        if tombstone not in router_source:
            errors.append(f"missing 410 tombstone route: {tombstone}")

    for path in (ROOT / "web").rglob("*.go"):
        if path.name.endswith("_test.go") or path == ROOT / "web/agent/capabilities.go":
            continue
        source = path.read_text(encoding="utf-8", errors="replace")
        for symbol in ("MethodAgentExec", "MethodAgentTerminal", "MethodAgentFile"):
            if re.search(rf"\b{symbol}\b", source):
                errors.append(f"{path.relative_to(ROOT)} references remote dispatch symbol {symbol}")

    if errors:
        print("remote-control removal guard: FAIL", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1
    print("remote-control removal guard: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
