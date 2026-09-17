#!/usr/bin/env bash
#
# Self-test for scripts/release-guard.sh. No network, no GitHub, no registry: both
# the gh CLI and docker are replaced by stubs on PATH, so the guard's decisions can
# be checked deterministically.
#
#   bash scripts/tests/test-release-guard.sh
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="${ROOT}/scripts/release-guard.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

BIN="${WORK}/bin"
STATE="${WORK}/state"
mkdir -p "$BIN" "${STATE}/assets"

pass=0
fail=0
check() { # check <description> <command...>
	local description="$1"
	shift
	if "$@" > /dev/null 2>&1; then
		printf '  ok   %s\n' "$description"
		pass=$((pass + 1))
	else
		printf '  FAIL %s\n' "$description"
		fail=$((fail + 1))
	fi
}

# ---------------------------------------------------------------- stubs

cat > "${BIN}/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
state="${FAKE_GH_STATE:?FAKE_GH_STATE must be set}"
[ "$1" = "api" ] || { echo "stub gh: unexpected command: $*" >&2; exit 9; }
shift
if [ "${1:-}" = "-H" ]; then
	# -H "Accept: application/octet-stream" /repos/<repo>/releases/assets/<id>
	asset_path="${3:-}"
	id="${asset_path##*/}"
	[ -f "${state}/assets/${id}" ] || { echo "stub gh: no such asset ${id}" >&2; exit 1; }
	cat "${state}/assets/${id}"
else
	[ -f "${state}/release.json" ] || { echo "stub gh: no release.json" >&2; exit 1; }
	cat "${state}/release.json"
fi
STUB
chmod +x "${BIN}/gh"

cat > "${BIN}/docker" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
# usage from the guard: docker buildx imagetools inspect <ref>
if [ "${1:-}" = "buildx" ] && [ "${2:-}" = "imagetools" ] && [ "${3:-}" = "inspect" ]; then
	ref="${4:-}"
	for existing in ${FAKE_DOCKER_EXISTING:-}; do
		if [ "$existing" = "$ref" ]; then
			echo "manifest for ${ref}"
			exit 0
		fi
	done
	exit 1
fi
echo "stub docker: unexpected command: $*" >&2
exit 9
STUB
chmod +x "${BIN}/docker"

export PATH="${BIN}:${PATH}"
export FAKE_GH_STATE="$STATE"
# Point the guard at the stubs explicitly: overriding PATH is not reliable on every
# platform (Windows picks up gh.exe from the real PATH).
export KOMARI_GH_BIN="${BIN}/gh"
export KOMARI_DOCKER_BIN="${BIN}/docker"

DIST="${WORK}/dist"
mkdir -p "$DIST"
printf 'binary-amd64-payload\n' > "${DIST}/komari-linux-amd64"
printf 'checksums-payload\n' > "${DIST}/SHA256SUMS"

write_release_json() { # write_release_json <name> <id>
	local name="$1" id="$2"
	printf '{"tag_name":"v1.5.0-stable.1","assets":[{"id":%s,"name":"%s"}]}\n' "$id" "$name" > "${STATE}/release.json"
}

# ---------------------------------------------------------------- assets

echo "== assets: missing asset is planned for upload =="
printf '{"tag_name":"v1.5.0-stable.1","assets":[]}\n' > "${STATE}/release.json"
: > "${WORK}/plan.txt"
check "exit 0 when nothing is published yet" \
	bash "$GUARD" assets "xinian5216/komari-stable" "v1.5.0-stable.1" "${WORK}/plan.txt" "${DIST}/komari-linux-amd64"
check "the plan lists the missing asset" \
	grep -qx "komari-linux-amd64" "${WORK}/plan.txt"
check "the log announces the upload" \
	bash -c "bash '$GUARD' assets xinian5216/komari-stable v1.5.0-stable.1 '${WORK}/plan2.txt' '${DIST}/komari-linux-amd64' 2>&1 | grep -q 'UPLOAD komari-linux-amd64'"

echo "== assets: identical bytes are skipped =="
write_release_json "komari-linux-amd64" 111
cp "${DIST}/komari-linux-amd64" "${STATE}/assets/111"
: > "${WORK}/plan.txt"
check "exit 0 for an identical published asset" \
	bash "$GUARD" assets "xinian5216/komari-stable" "v1.5.0-stable.1" "${WORK}/plan.txt" "${DIST}/komari-linux-amd64"
check "nothing is planned for upload" \
	bash -c "[ ! -s '${WORK}/plan.txt' ]"
check "the log says SKIP" \
	bash -c "bash '$GUARD' assets xinian5216/komari-stable v1.5.0-stable.1 '${WORK}/plan2.txt' '${DIST}/komari-linux-amd64' 2>&1 | grep -q 'SKIP komari-linux-amd64'"

echo "== assets: different bytes must fail =="
write_release_json "komari-linux-amd64" 222
printf 'a different build\n' > "${STATE}/assets/222"
: > "${WORK}/plan.txt"
check "exit non-zero for a changed rebuild" \
	bash -c "! bash '$GUARD' assets xinian5216/komari-stable v1.5.0-stable.1 '${WORK}/plan.txt' '${DIST}/komari-linux-amd64'"
check "the failure names immutable release asset mismatch" \
	bash -c "bash '$GUARD' assets xinian5216/komari-stable v1.5.0-stable.1 '${WORK}/plan.txt' '${DIST}/komari-linux-amd64' 2>&1 | grep -q 'immutable release asset mismatch'"
check "nothing was planned for upload" \
	bash -c "[ ! -s '${WORK}/plan.txt' ]"

echo "== assets: SHA256SUMS is guarded the same way =="
write_release_json "SHA256SUMS" 333
printf 'a different checksum file\n' > "${STATE}/assets/333"
check "exit non-zero when SHA256SUMS changed" \
	bash -c "! bash '$GUARD' assets xinian5216/komari-stable v1.5.0-stable.1 '${WORK}/plan.txt' '${DIST}/SHA256SUMS'"
cp "${DIST}/SHA256SUMS" "${STATE}/assets/333"
check "exit 0 when SHA256SUMS is identical" \
	bash "$GUARD" assets "xinian5216/komari-stable" "v1.5.0-stable.1" "${WORK}/plan.txt" "${DIST}/SHA256SUMS"

echo "== assets: a partial retry only fills the gaps =="
write_release_json "komari-linux-amd64" 444
cp "${DIST}/komari-linux-amd64" "${STATE}/assets/444"
: > "${WORK}/plan.txt"
check "exit 0 when one asset is published and another is missing" \
	bash "$GUARD" assets "xinian5216/komari-stable" "v1.5.0-stable.1" "${WORK}/plan.txt" \
		"${DIST}/komari-linux-amd64" "${DIST}/SHA256SUMS"
check "only the missing asset is planned" \
	bash -c "grep -qx SHA256SUMS '${WORK}/plan.txt' && ! grep -qx komari-linux-amd64 '${WORK}/plan.txt'"

echo "== verify-sums: a checksum file must describe the published bytes =="
printf 'binary-amd64-payload\n' > "${DIST}/komari-linux-amd64"
( cd "${DIST}" && sha256sum komari-linux-amd64 > SHA256SUMS.true )
write_release_json "komari-linux-amd64" 555
cp "${DIST}/komari-linux-amd64" "${STATE}/assets/555"
check "exit 0 when the checksum file matches the published asset" \
	bash "$GUARD" verify-sums "xinian5216/komari-stable" "v1.5.0-stable.1" "${DIST}/SHA256SUMS.true"

printf 'checksums-for-a-different-build\n' > "${DIST}/SHA256SUMS.wrong"
printf '%s  komari-linux-amd64\n' "0000000000000000000000000000000000000000000000000000000000000000" > "${DIST}/SHA256SUMS.wrong"
check "exit non-zero when the checksum file disagrees with the published asset" \
	bash -c "! bash '$GUARD' verify-sums xinian5216/komari-stable v1.5.0-stable.1 '${DIST}/SHA256SUMS.wrong'"
check "the failure names the mismatch" \
	bash -c "bash '$GUARD' verify-sums xinian5216/komari-stable v1.5.0-stable.1 '${DIST}/SHA256SUMS.wrong' 2>&1 | grep -q 'does not describe the published assets'"

printf 'nothing  komari-linux-mips\n' > "${DIST}/SHA256SUMS.missing"
check "exit non-zero when the checksum file lists an asset that is not published" \
	bash -c "! bash '$GUARD' verify-sums xinian5216/komari-stable v1.5.0-stable.1 '${DIST}/SHA256SUMS.missing'"

# ---------------------------------------------------------------- image tags

echo "== image: an existing fixed version tag must fail =="
export FAKE_DOCKER_EXISTING="ghcr.io/xinian5216/komari-stable:v1.5.0-stable.1 ghcr.io/xinian5216/komari-stable:stable"
check "exit non-zero for an existing version tag" \
	bash -c "! bash '$GUARD' image ghcr.io/xinian5216/komari-stable:v1.5.0-stable.1 ghcr.io/xinian5216/komari-stable:stable"
check "the failure is explicit about immutability" \
	bash -c "bash '$GUARD' image ghcr.io/xinian5216/komari-stable:v1.5.0-stable.1 ghcr.io/xinian5216/komari-stable:stable 2>&1 | grep -q 'immutable image tag'"

echo "== image: a new version tag is allowed =="
check "exit 0 for a version tag that does not exist yet" \
	bash "$GUARD" image ghcr.io/xinian5216/komari-stable:v1.5.0-stable.2 ghcr.io/xinian5216/komari-stable:stable

echo "== image: an unreachable registry must not be mistaken for 'not published' =="
export FAKE_DOCKER_EXISTING=""
check "exit non-zero when the registry cannot be reached" \
	bash -c "! bash '$GUARD' image ghcr.io/xinian5216/komari-stable:v1.5.0-stable.2 ghcr.io/xinian5216/komari-stable:stable"
check "the failure names the infrastructure problem" \
	bash -c "bash '$GUARD' image ghcr.io/xinian5216/komari-stable:v1.5.0-stable.2 ghcr.io/xinian5216/komari-stable:stable 2>&1 | grep -q 'cannot reach the registry'"

# --------------------------------------------------------------- contract
#
# The immutability policy must travel with the released tag: the workflows may
# check out the tag as their build source, but they must never take the guard
# from a mutable branch.

RELEASE_YML="${ROOT}/.github/workflows/stable-release.yml"

check_file_contains() { # <desc> <file> <pattern>
	if grep -q -- "$3" "$2" 2>/dev/null; then
		printf '  ok   %s
' "$1"
		pass=$((pass + 1))
	else
		printf '  FAIL %s
' "$1"
		fail=$((fail + 1))
	fi
}
check_file_absent() { # <desc> <file> <pattern>
	if grep -q -- "$3" "$2" 2>/dev/null; then
		printf '  FAIL %s
' "$1"
		fail=$((fail + 1))
	else
		printf '  ok   %s
' "$1"
		pass=$((pass + 1))
	fi
}

printf '
== guard resolution contract ==
'
check_file_absent "stable-release.yml never derives the guard from the triggering branch" "${RELEASE_YML}" "github.ref_name"
check_file_absent "stable-release.yml never fetches a branch for the guard" "${RELEASE_YML}" "FETCH_HEAD"
check_file_contains "stable-release.yml refuses legacy tags explicitly" "${RELEASE_YML}" "[ ! -f scripts/release-guard.sh ]"

release_tag_refs="$(grep -c 'ref: \${{ env.RELEASE_TAG }}' "${RELEASE_YML}" 2>/dev/null || true)"
if [ "${release_tag_refs:-0}" -ge 4 ]; then
	printf '  ok   every job that runs repo tooling checks out the released tag (%s)
' "${release_tag_refs}"
	pass=$((pass + 1))
else
	printf '  FAIL only %s job(s) check out the released tag, expected at least 4
' "${release_tag_refs:-0}"
	fail=$((fail + 1))
fi

# Functional: the refusal text is extracted from the workflow, then executed in a
# checkout that has the guard and one that does not.
CONTRACT_DIR="${WORK}/guard-contract"
mkdir -p "${CONTRACT_DIR}/tag/scripts" "${CONTRACT_DIR}/legacy"
printf '#!/usr/bin/env bash
exit 0
' > "${CONTRACT_DIR}/tag/scripts/release-guard.sh"
sed -n '/if \[ ! -f scripts\/release-guard\.sh \]; then/,/^          fi$/p' "${RELEASE_YML}" | head -4 > "${CONTRACT_DIR}/require.sh"

if [ -s "${CONTRACT_DIR}/require.sh" ]; then
	if (cd "${CONTRACT_DIR}/tag" && bash "${CONTRACT_DIR}/require.sh") > /dev/null 2>&1; then
		printf '  ok   a release that ships the guard passes the requirement
'
		pass=$((pass + 1))
	else
		printf '  FAIL a release that ships the guard was rejected
'
		fail=$((fail + 1))
	fi
	legacy_status=0
	# set -e is active in this suite: a failure here is the expected outcome.
	legacy_out="$(cd "${CONTRACT_DIR}/legacy" && bash "${CONTRACT_DIR}/require.sh" 2>&1)" || legacy_status=$?
	if [ "${legacy_status}" -ne 0 ]; then
		printf '  ok   a legacy release without the guard fails closed
'
		pass=$((pass + 1))
	else
		printf '  FAIL a legacy release without the guard was accepted
'
		fail=$((fail + 1))
	fi
	case "${legacy_out}" in
		*"this legacy release does not contain the immutable release guard; automatic repair is refused"*)
			printf '  ok   the legacy failure says why it refused
'
			pass=$((pass + 1))
			;;
		*)
			printf '  FAIL the legacy failure does not explain the refusal
'
			fail=$((fail + 1))
			;;
	esac
else
	printf '  FAIL could not extract the legacy guard refusal from stable-release.yml
'
	fail=$((fail + 1))
fi

echo
echo "release guard self-test: ${pass} passed, ${fail} failed"
[ "$fail" -eq 0 ]
