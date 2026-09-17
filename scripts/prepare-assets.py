#!/usr/bin/env python3
"""Prepare the build-time assets that //go:embed resolves.

Single source of truth: bundled-themes.lock.json. Both CI and local builds use this
entry point, and nothing here ever follows main, latest, or an unverified asset.

Prepared assets:
  1. embedded_default_frontend -> web/public/defaultTheme/{dist.tar.zst,komari-theme.json,...}
     Built from the pinned komari-web-stable commit (git clone + npm ci + npm run build,
     then tar + zstd). The built artifact hash is recorded for idempotent reuse.
  2. bundled_preferred_theme   -> web/public/bundledTheme/next.zip
     Downloaded from the pinned release asset and verified against the locked SHA256.

Idempotent: an asset that already exists with the expected hash is reused as-is.

Usage:
  python3 scripts/prepare-assets.py [all|frontend|theme] [--force] [--lock FILE]

Test-only injection (never used by CI/release):
  --theme-zip PATH        use a local zip instead of downloading (the provenance hash is
                          then the local file's hash unless --theme-sha256 is given)
  --theme-sha256 HEX      record this hash as the expected bundle hash (used to build
                          fixtures that exercise the runtime hash-mismatch path)
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LOCK_PATH = ROOT / "bundled-themes.lock.json"
FRONTEND_DIR = ROOT / "web" / "public" / "defaultTheme"
BUNDLED_DIR = ROOT / "web" / "public" / "bundledTheme"
PROVENANCE_PATH = ROOT / "web" / "public" / "assets.provenance.json"


def log(message: str) -> None:
    print(f"[prepare-assets] {message}", flush=True)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_lock(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        data = json.load(handle)
    assets = data.get("assets") or {}
    for key in ("embedded_default_frontend", "bundled_preferred_theme"):
        if key not in assets:
            raise SystemExit(f"FAIL: lock file is missing the {key!r} entry")
    return assets


def load_provenance() -> dict:
    if PROVENANCE_PATH.exists():
        try:
            return json.loads(PROVENANCE_PATH.read_text(encoding="utf-8"))
        except Exception:  # noqa: BLE001 - a corrupt provenance file must not block a rebuild
            return {}
    return {}


def save_provenance(entry: str, payload: dict) -> None:
    data = load_provenance()
    data[entry] = payload
    PROVENANCE_PATH.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def run(cmd: list[str], cwd: Path | None = None) -> None:
    log("$ " + " ".join(cmd))
    result = subprocess.run(cmd, cwd=cwd)
    if result.returncode != 0:
        raise SystemExit(f"FAIL: command failed with exit code {result.returncode}: {' '.join(cmd)}")


def find_zstd() -> str:
    candidate = os.environ.get("ZSTD") or shutil.which("zstd")
    if not candidate:
        raise SystemExit(
            "FAIL: zstd is required to pack the embedded default frontend. "
            "Install zstd (apt-get install zstd) or set the ZSTD environment variable."
        )
    return candidate


def prepare_frontend(assets: dict, force: bool) -> None:
    spec = assets["embedded_default_frontend"]
    commit = spec["commit"]
    tarball = FRONTEND_DIR / "dist.tar.zst"
    manifest = FRONTEND_DIR / "komari-theme.json"
    provenance = load_provenance().get("embedded_default_frontend") or {}

    if not force and tarball.exists() and manifest.exists():
        if provenance.get("commit") == commit:
            log(f"frontend: reuse existing build for {commit[:8]} (sha256={sha256_file(tarball)[:16]}...)")
            return
        log("frontend: existing build has different provenance, rebuilding")

    build = spec.get("build") or {}
    node_version = str(build.get("node_version") or "")
    with tempfile.TemporaryDirectory(prefix="komari-web-") as tmp:
        checkout = Path(tmp) / "komari-web"
        log(f"frontend: cloning {spec['repository']} at {commit[:8]}")
        run(["git", "clone", "--quiet", "--no-checkout", f"https://github.com/{spec['repository']}.git", str(checkout)])
        run(["git", "-c", "advice.detachedHead=false", "checkout", "--quiet", commit], cwd=checkout)

        if node_version:
            log(f"frontend: expected node major version {node_version} (CI pins the toolchain)")

        install_cmd = ["npm", "ci", "--no-audit", "--no-fund"] if (checkout / "package-lock.json").exists() else ["npm", "install", "--no-audit", "--no-fund"]
        run(install_cmd, cwd=checkout)
        run(["npm", "run", "build"], cwd=checkout)

        dist = checkout / "dist"
        if not (dist / "index.html").exists():
            raise SystemExit("FAIL: frontend build did not produce dist/index.html")

        FRONTEND_DIR.mkdir(parents=True, exist_ok=True)
        staging_tar = Path(tmp) / "dist.tar"
        with tarfile.open(staging_tar, "w") as archive:
            for entry in sorted(dist.iterdir()):
                archive.add(entry, arcname=entry.name)

        zstd = find_zstd()
        tmp_out = FRONTEND_DIR / "dist.tar.zst.tmp"
        run([zstd, "-19", "-T0", "-q", "-f", str(staging_tar), "-o", str(tmp_out)])
        os.replace(tmp_out, tarball)

        for name in ("komari-theme.json", "preview.png", "perview.png"):
            candidate = checkout / name
            if candidate.exists():
                shutil.copyfile(candidate, FRONTEND_DIR / name)
        preview = FRONTEND_DIR / "preview.png"
        perview = FRONTEND_DIR / "perview.png"
        if preview.exists() and not perview.exists():
            shutil.copyfile(preview, perview)
        if perview.exists() and not preview.exists():
            shutil.copyfile(perview, preview)

    digest = sha256_file(tarball)
    save_provenance(
        "embedded_default_frontend",
        {
            "repository": spec["repository"],
            "tag": spec["tag"],
            "commit": commit,
            "artifact": "web/public/defaultTheme/dist.tar.zst",
            "sha256": digest,
        },
    )
    log(f"frontend: built {commit[:8]} -> dist.tar.zst (sha256={digest[:16]}...)")


def download(url: str, destination: Path) -> None:
    log(f"theme: downloading {url}")
    with urllib.request.urlopen(url, timeout=120) as response:  # noqa: S310 - locked https URL
        with open(destination, "wb") as handle:
            shutil.copyfileobj(response, handle)


def prepare_theme(assets: dict, force: bool, local_zip: str | None, expected_override: str | None) -> None:
    spec = assets["bundled_preferred_theme"]
    target = BUNDLED_DIR / "next.zip"
    provenance = load_provenance().get("bundled_preferred_theme") or {}
    expected = (expected_override or spec["sha256"]).lower()

    if not force and target.exists():
        actual = sha256_file(target)
        if provenance.get("sha256", "").lower() == actual and (
            expected_override is not None or actual == expected
        ):
            log(f"theme: reuse existing next.zip (sha256={actual[:16]}...)")
            return

    BUNDLED_DIR.mkdir(parents=True, exist_ok=True)
    tmp_zip = BUNDLED_DIR / "next.zip.tmp"
    if local_zip:
        shutil.copyfile(Path(local_zip), tmp_zip)
        actual = sha256_file(tmp_zip)
        log(f"theme: using local zip {local_zip} (sha256={actual[:16]}...)")
        if expected_override is None and actual != expected:
            raise SystemExit(
                f"FAIL: local zip hash {actual} does not match the locked sha256 {expected}. "
                "Pass --theme-sha256 to record a fixture hash deliberately."
            )
        recorded = actual
    else:
        download(spec["asset_url"], tmp_zip)
        actual = sha256_file(tmp_zip)
        if expected_override is None and actual != expected:
            raise SystemExit(f"FAIL: downloaded asset hash {actual} does not match the locked sha256 {expected}")
        recorded = actual

    os.replace(tmp_zip, target)

    manifest = {}
    try:
        import zipfile

        with zipfile.ZipFile(target) as archive:
            manifest = json.loads(archive.read("komari-theme.json"))
    except Exception as exc:  # noqa: BLE001 - surfaced to the caller as a warning
        log(f"theme: warning: could not read komari-theme.json from the zip: {exc}")

    payload = {
        "repository": spec["repository"],
        "tag": spec["tag"],
        "commit": spec["commit"],
        "asset": spec["asset"],
        "sha256": recorded,
        "theme_short": manifest.get("short") or spec.get("theme_short"),
        "theme_version": manifest.get("version") or spec.get("theme_version"),
        "embedded_path": "web/public/bundledTheme/next.zip",
    }
    save_provenance("bundled_preferred_theme", payload)
    # 随包内嵌的 provenance（运行时据此校验嵌入的 zip；不是仓库级记录）
    embedded = dict(payload)
    embedded.pop("embedded_path", None)
    (BUNDLED_DIR / "provenance.json").write_text(
        json.dumps(embedded, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    log(f"theme: prepared {target.relative_to(ROOT)} (sha256={recorded[:16]}...)")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("target", nargs="?", default="all", choices=["all", "frontend", "theme"])
    parser.add_argument("--lock", default=str(LOCK_PATH))
    parser.add_argument("--force", action="store_true", help="rebuild/re-download even if the asset looks current")
    parser.add_argument("--theme-zip", default=None, help="test only: use a local zip instead of downloading")
    parser.add_argument("--theme-sha256", default=None, help="test only: record this hash instead of the locked one")
    args = parser.parse_args()

    assets = load_lock(Path(args.lock))
    if args.target in ("all", "frontend"):
        prepare_frontend(assets, args.force)
    if args.target in ("all", "theme"):
        prepare_theme(assets, args.force, args.theme_zip, args.theme_sha256)
    log("done")


if __name__ == "__main__":
    main()
