#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if [ "${1:-}" = "--tty-read-probe" ]; then
    # shellcheck source=/dev/null
    source "$ROOT/install-komari.sh"
    REPLY=""
    read_user_input
    printf 'tty-input=%s\n' "$REPLY"
    exit 0
fi

stdin_help_output="$(bash -s -- --help < "$ROOT/install-komari.sh" 2>&1)"
printf '%s\n' "$stdin_help_output" | grep -Fq 'Komari Stable installer'
if printf '%s\n' "$stdin_help_output" | grep -Fq 'return: can only'; then
    echo 'FAIL: stdin execution was mistaken for a sourced installer' >&2
    exit 1
fi

tty_probe_output="$(
    printf '2\n' | script -qefc \
        "bash '$ROOT/scripts/tests/test-installer-migration.sh' --tty-read-probe" \
        /dev/null
)"
if ! printf '%s\n' "$tty_probe_output" | grep -Fq 'tty-input=2'; then
    echo 'FAIL: piped installer did not read interactive input from /dev/tty' >&2
    exit 1
fi

# shellcheck source=/dev/null
source "$ROOT/install-komari.sh"

INSTALL_DIR="$WORK/install"
DATA_DIR="$INSTALL_DIR"
BINARY_PATH="$INSTALL_DIR/komari"
BACKUP_DIR="$INSTALL_DIR/backup"
DATA_BACKUP_DIR="$INSTALL_DIR/data/backup"
DATA_MIGRATION_BACKUP_DIR="$BACKUP_DIR"
STANDARD_REPO="xinian5216/komari-stable"
REPO="$STANDARD_REPO"

mkdir -p "$INSTALL_DIR/data/plugin/demo" "$INSTALL_DIR/data/backup" "$BACKUP_DIR"
printf 'db-before' > "$INSTALL_DIR/data/komari.db"
printf 'plugin-before' > "$INSTALL_DIR/data/plugin/demo/index.js"
printf 'old-backup' > "$INSTALL_DIR/data/backup/old.zip"
printf 'old-binary' > "$BINARY_PATH"
printf 'staged' > "$INSTALL_DIR/komari.new.fixture"

create_data_backup fixture
[ -s "$DATA_BACKUP_ARCHIVE" ]
tar -tzf "$DATA_BACKUP_ARCHIVE" | grep -Fxq './data/komari.db'
tar -tzf "$DATA_BACKUP_ARCHIVE" | grep -Fxq './data/plugin/demo/index.js'
if tar -tzf "$DATA_BACKUP_ARCHIVE" | grep -qE '^\./data/backup/|^\./komari($|\.)'; then
    echo 'FAIL: offline archive contains excluded binaries or nested backups' >&2
    exit 1
fi

printf 'db-after' > "$INSTALL_DIR/data/komari.db"
restore_data_backup "$DATA_BACKUP_ARCHIVE" fixture
[ "$(cat "$INSTALL_DIR/data/komari.db")" = 'db-before' ]
[ "$(cat "$FAILED_DATA_DIR/komari.db")" = 'db-after' ]

release_dir="$WORK/release"
mkdir -p "$release_dir"
printf 'verified-binary' > "$release_dir/komari-linux-amd64"
(cd "$release_dir" && sha256sum komari-linux-amd64 > SHA256SUMS)
verify_download "file://$release_dir/komari-linux-amd64" \
    "$release_dir/komari-linux-amd64" "komari-linux-amd64"
rm "$release_dir/SHA256SUMS"
if verify_download "file://$release_dir/komari-linux-amd64" \
    "$release_dir/komari-linux-amd64" "komari-linux-amd64"; then
    echo 'FAIL: Komari Stable asset without a checksum was accepted' >&2
    exit 1
fi

FAKE_AVAILABLE_KB=100000
du() {
    case "${*: -1}" in
        "$BINARY_PATH") printf '2\t%s\n' "$BINARY_PATH" ;;
        *) printf '10\t%s\n' "$INSTALL_DIR/data" ;;
    esac
}
df() {
    printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
    printf 'fixture 200000 1 %s 1%% %s\n' "$FAKE_AVAILABLE_KB" "$INSTALL_DIR"
}
check_migration_disk_space
[ "$MIGRATION_REQUIRED_KB" -eq 65568 ]
FAKE_AVAILABLE_KB=65567
if check_migration_disk_space >/dev/null 2>&1; then
    echo 'FAIL: insufficient rollback workspace was accepted' >&2
    exit 1
fi

systemctl() { return 0; }
curl() {
    case "${*: -1}" in
        */ping) printf 'pong' ;;
        */api/version) printf '{"data":{"version":"1.5.0-stable.test"}}' ;;
        *) return 1 ;;
    esac
}
HEALTHCHECK_TIMEOUT=2
wait_for_target_health 'v1.5.0-stable.test'
[ "$HEALTHY_VERSION" = '1.5.0-stable.test' ]

echo 'OK: installer migration safety tests passed'
