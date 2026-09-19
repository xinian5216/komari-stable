#!/usr/bin/env bash
#
# Fail-closed release provenance check. This script must be executed from the
# protected stable branch, never from the tag being evaluated.
#
#   release-preflight.sh <repo> <tag> <branch> <required-check>...
#
set -euo pipefail

log() { printf '[release-preflight] %s\n' "$*"; }

usage() {
	printf 'usage: release-preflight.sh <repo> <tag> <branch> <required-check>...\n' >&2
	exit 2
}

[ "$#" -ge 4 ] || usage

repo="$1"
tag="$2"
branch="$3"
shift 3
required_checks=("$@")

case "$repo" in
	*/*) ;;
	*) log "FAIL: invalid repository name: ${repo}"; exit 2 ;;
esac

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+-stable\.[0-9]+$ ]]; then
	log "FAIL: invalid stable release tag: ${tag}"
	exit 1
fi

GIT_BIN="${KOMARI_GIT_BIN:-git}"
GH_BIN="${KOMARI_GH_BIN:-gh}"

python_bin="${KOMARI_PYTHON:-}"
if [ -z "$python_bin" ]; then
	for candidate in python3 python; do
		if command -v "$candidate" >/dev/null 2>&1 && "$candidate" -c 'import json' >/dev/null 2>&1; then
			python_bin="$candidate"
			break
		fi
	done
fi
if [ -z "$python_bin" ]; then
	log "FAIL: python3 (or python) is required"
	exit 1
fi

tag_sha="$($GIT_BIN rev-parse --verify "refs/tags/${tag}^{commit}" 2>/dev/null)" || {
	log "FAIL: release tag does not exist locally: ${tag}"
	exit 1
}
branch_sha="$($GIT_BIN rev-parse --verify "refs/remotes/origin/${branch}^{commit}" 2>/dev/null)" || {
	log "FAIL: protected branch ref is unavailable: origin/${branch}"
	exit 1
}

if ! "$GIT_BIN" merge-base --is-ancestor "$tag_sha" "$branch_sha"; then
	log "FAIL: tag ${tag} commit ${tag_sha} is not an ancestor of origin/${branch} (${branch_sha})"
	exit 1
fi
log "OK: ${tag} resolves to ${tag_sha} in origin/${branch} history"

checks_json="$(mktemp)"
trap 'rm -f "$checks_json"' EXIT
if ! "$GH_BIN" api --paginate --slurp \
	-H "Accept: application/vnd.github+json" \
	"repos/${repo}/commits/${tag_sha}/check-runs?per_page=100" > "$checks_json"; then
	log "FAIL: cannot read check runs for ${repo}@${tag_sha}"
	exit 1
fi

if ! "$python_bin" - "$checks_json" "$tag_sha" "${required_checks[@]}" <<'PY'
import json
import sys

path, expected_sha, *required = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    payload = json.load(handle)

pages = payload if isinstance(payload, list) else [payload]
latest = {}
for page in pages:
    for run in page.get("check_runs") or []:
        if run.get("head_sha") != expected_sha:
            continue
        if (run.get("app") or {}).get("slug") != "github-actions":
            continue
        name = run.get("name")
        if name not in required:
            continue
        try:
            run_id = int(run.get("id") or 0)
        except (TypeError, ValueError):
            run_id = 0
        previous = latest.get(name)
        if previous is None or run_id > previous[0]:
            latest[name] = (run_id, run)

failed = False
for name in required:
    selected = latest.get(name)
    if selected is None:
        print(f"[release-preflight] FAIL: missing required check: {name}")
        failed = True
        continue
    run = selected[1]
    status = run.get("status")
    conclusion = run.get("conclusion")
    if status != "completed" or conclusion != "success":
        print(
            f"[release-preflight] FAIL: required check did not succeed: "
            f"{name} (status={status}, conclusion={conclusion})"
        )
        failed = True
    else:
        print(f"[release-preflight] OK: {name}=success")

if failed:
    raise SystemExit(1)
PY
then
	log "FAIL: release commit has not passed every required GitHub Actions check"
	exit 1
fi

log "OK: release provenance and required checks verified for ${tag}"
