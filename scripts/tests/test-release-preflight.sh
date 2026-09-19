#!/usr/bin/env bash
#
# Deterministic self-test for scripts/release-preflight.sh. The test creates a
# tiny local Git history and replaces `gh` with a fixture reader, so it never
# contacts GitHub.
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PREFLIGHT="${ROOT}/scripts/release-preflight.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

expect_success() { # <description> <command...>
	local description="$1"
	shift
	if "$@" > "${WORK}/last.out" 2>&1; then
		printf '  ok   %s\n' "$description"
		pass=$((pass + 1))
	else
		printf '  FAIL %s\n' "$description"
		cat "${WORK}/last.out"
		fail=$((fail + 1))
	fi
}

expect_failure() { # <description> <expected text> <command...>
	local description="$1" expected="$2"
	shift 2
	local status=0
	"$@" > "${WORK}/last.out" 2>&1 || status=$?
	if [ "$status" -ne 0 ] && grep -Fq "$expected" "${WORK}/last.out"; then
		printf '  ok   %s\n' "$description"
		pass=$((pass + 1))
	else
		printf '  FAIL %s\n' "$description"
		cat "${WORK}/last.out"
		fail=$((fail + 1))
	fi
}

BIN="${WORK}/bin"
REPO="${WORK}/repo"
CHECKS="${WORK}/checks.json"
mkdir -p "$BIN" "$REPO"

cat > "${BIN}/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
[ "${1:-}" = "api" ] || { echo "stub gh: unexpected command: $*" >&2; exit 9; }
cat "${FAKE_CHECKS_JSON:?FAKE_CHECKS_JSON must be set}"
STUB
chmod +x "${BIN}/gh"

git -C "$REPO" init -q -b stable
git -C "$REPO" config user.name "Release Preflight Test"
git -C "$REPO" config user.email "release-preflight@example.invalid"
printf 'protected\n' > "${REPO}/payload"
git -C "$REPO" add payload
git -C "$REPO" commit -q -m 'protected commit'
GOOD_SHA="$(git -C "$REPO" rev-parse HEAD)"
git -C "$REPO" tag v1.5.0-stable.99
git -C "$REPO" update-ref refs/remotes/origin/stable "$GOOD_SHA"

git -C "$REPO" switch -q -c unprotected
printf 'unprotected\n' >> "${REPO}/payload"
git -C "$REPO" commit -qam 'unprotected commit'
BAD_SHA="$(git -C "$REPO" rev-parse HEAD)"
git -C "$REPO" tag v1.5.0-stable.100
git -C "$REPO" switch -q stable

write_checks() { # <required-gate conclusion> <include secret gate> <secret app slug>
	local gate_conclusion="$1" include_secret="$2" secret_app="$3"
	local secret=''
	if [ "$include_secret" = yes ]; then
		secret=",{\"id\":40,\"name\":\"secret-gate\",\"head_sha\":\"${GOOD_SHA}\",\"status\":\"completed\",\"conclusion\":\"success\",\"app\":{\"slug\":\"${secret_app}\"}}"
	fi
	printf '[{"check_runs":[' > "$CHECKS"
	printf '{"id":10,"name":"required-gate","head_sha":"%s","status":"completed","conclusion":"success","app":{"slug":"github-actions"}},' "$GOOD_SHA" >> "$CHECKS"
	printf '{"id":20,"name":"admin-sw-gate","head_sha":"%s","status":"completed","conclusion":"success","app":{"slug":"github-actions"}},' "$GOOD_SHA" >> "$CHECKS"
	printf '{"id":30,"name":"security-gate","head_sha":"%s","status":"completed","conclusion":"success","app":{"slug":"github-actions"}}' "$GOOD_SHA" >> "$CHECKS"
	printf '%s' "$secret" >> "$CHECKS"
	if [ "$gate_conclusion" != success ]; then
		printf ',{"id":50,"name":"required-gate","head_sha":"%s","status":"completed","conclusion":"%s","app":{"slug":"github-actions"}}' "$GOOD_SHA" "$gate_conclusion" >> "$CHECKS"
	fi
	printf ']}]\n' >> "$CHECKS"
}

run_preflight() {
	(
		cd "$REPO"
		FAKE_CHECKS_JSON="$CHECKS" KOMARI_GH_BIN="${BIN}/gh" \
			bash "$PREFLIGHT" xinian5216/komari-stable "$@"
	)
}

printf '== release preflight ==\n'
write_checks success yes github-actions
expect_success "a protected stable ancestor with every required check succeeds" \
	run_preflight v1.5.0-stable.99 stable required-gate admin-sw-gate security-gate secret-gate

expect_failure "an invalid release tag is rejected" "invalid stable release tag" \
	run_preflight stable stable required-gate admin-sw-gate security-gate secret-gate

expect_failure "a tag outside stable history is rejected" "is not an ancestor" \
	run_preflight v1.5.0-stable.100 stable required-gate admin-sw-gate security-gate secret-gate

write_checks success no github-actions
expect_failure "a missing required check is rejected" "missing required check: secret-gate" \
	run_preflight v1.5.0-stable.99 stable required-gate admin-sw-gate security-gate secret-gate

write_checks success yes untrusted-app
expect_failure "a lookalike check from another app is rejected" "missing required check: secret-gate" \
	run_preflight v1.5.0-stable.99 stable required-gate admin-sw-gate security-gate secret-gate

write_checks failure yes github-actions
expect_failure "the newest failed rerun overrides an older success" "required check did not succeed: required-gate" \
	run_preflight v1.5.0-stable.99 stable required-gate admin-sw-gate security-gate secret-gate

printf '\n== workflow contract ==\n'
RELEASE_YML="${ROOT}/.github/workflows/stable-release.yml"
expect_success "the workflow can read GitHub check runs" \
	grep -Fq 'checks: read' "$RELEASE_YML"
expect_success "the preflight policy is checked out from protected stable" \
	grep -Fq 'ref: stable' "$RELEASE_YML"
expect_success "the protected branch checkout includes complete history and tags" \
	grep -Fq 'fetch-depth: 0' "$RELEASE_YML"
expect_success "the workflow invokes the release preflight" \
	grep -Fq 'bash scripts/release-preflight.sh' "$RELEASE_YML"
expect_success "asset preparation cannot run before the preflight" \
	grep -Fq 'needs: release-preflight' "$RELEASE_YML"

printf '\nrelease preflight self-test: %s passed, %s failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
