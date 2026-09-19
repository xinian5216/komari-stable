# Stable branch checks

Rulesets must target `stable`, the default and release branch.

Required contexts: `required-gate`, `admin-sw-gate`, `security-gate`,
`secret-gate`.
Each aggregation job runs with `always()` and accepts only `success` from every
dependency. Failure, cancellation, or skipped dependencies must block merging.
Security scanning runs on every PR to stable and every push to stable, including
documentation changes. The push run is required because squash merging creates a
new commit SHA; release preflight verifies checks for that exact stable SHA.
Use GitHub Actions as the required check source.

These checks cover the existing CI, the anonymous installation rehearsal, the
official-1.4.3 migration rehearsal, the bundled-theme fresh-install/failure/upgrade
matrix, the real-browser Next/admin Service Worker regression, dependency/CodeQL
analysis and secret scans. The three long rehearsals are reusable workflows called
by `stable-ci.yml`, so their failure reaches the single `required-gate` context.
`stable-release.yml`
also reads `scripts/release-preflight.sh` from protected `stable` and refuses to
build a release unless its immutable tag commit is in `stable` history and the
four contexts above have succeeded for that exact commit. Additional release
rehearsals are tracked separately in xinian5216/komari-stable#3.

Configure an active branch ruleset with no bypass actors: require PRs (zero
approving reviews for the solo maintainer), resolve conversations, require the
checks above with an up-to-date branch, linear history, and block deletion and
force pushes. Configure immutable release tags separately.

Changing these check names requires updating the ruleset before removing a name.
Keep release tags and existing assets immutable.
