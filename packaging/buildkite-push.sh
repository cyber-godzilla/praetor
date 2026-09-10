#!/usr/bin/env bash
# Push package files to a Buildkite Package Registry, idempotently.
#
#   BUILDKITE_TOKEN=<token> packaging/buildkite-push.sh <registry-slug> <file>...
#
# A 2xx is a successful publish. A 409 means this exact package version is
# already in the registry — which is what a re-run of a release job for an
# existing tag sees — and is reported and skipped rather than failing the
# job, so the steps after it still run. Anything else fails with the API's
# message. The token only ever travels in the header and is never printed.
#
# Used by the deb and rpm steps in .github/workflows/release.yaml. (The Arch
# step has its own client in packaging/arch/publish.sh because it also has
# to list and delete; the Chocolatey step goes through `choco push`.)
set -euo pipefail

: "${BUILDKITE_TOKEN:?}"
BK_ORG="${BK_ORG:-cybergodzilla-2099}"
registry="${1:?usage: buildkite-push.sh <registry-slug> <file>...}"; shift
[[ $# -gt 0 ]] || { echo "buildkite-push.sh: no files given" >&2; exit 1; }

API="${BUILDKITE_API:-https://api.buildkite.com}/v2/packages/organizations/${BK_ORG}/registries/${registry}/packages"
body="$(mktemp)"; trap 'rm -f "$body"' EXIT

rc=0
for f in "$@"; do
  status="$(curl -sS -o "$body" -w '%{http_code}' -X POST "$API" \
    -H "Authorization: Bearer ${BUILDKITE_TOKEN}" -F "file=@${f}")"
  case "$status" in
    2??) echo "pushed ${f} to ${registry} (HTTP ${status})" ;;
    409) echo "already in ${registry}, skipping: ${f} (HTTP 409)" ;;
    *)   echo "push of ${f} to ${registry} failed: HTTP ${status}: $(cat "$body")" >&2; rc=1 ;;
  esac
done
exit "$rc"
