#!/usr/bin/env bash
# Build the Arch Linux package + a one-package pacman repo db, and publish
# both to the Buildkite *Files* registry (Buildkite has no pacman registry
# type). Run on the amd64 Linux release runner after the binaries exist.
# Self-contained, like packaging/homebrew/render.sh.
#
# Files-registry rules (verified 2026-09-10) that shape everything below:
#   * Filenames must match {BASENAME}-{SEMVER}.{EXT}. nfpm's default
#     praetor-<ver>-1-x86_64.pkg.tar.zst is REJECTED (the `_` lands in what
#     the registry parses as a prerelease), so the package is renamed to
#     praetor-<ver>-1.x86_64.pkg.tar.zst. That renamed file is what repo-add
#     records, so pacman fetches exactly the name the registry serves.
#   * Re-uploading an existing filename is HTTP 409. pacman fetches
#     <section>.db, a FIXED name, so the db is deleted (delete_packages
#     scope) and re-uploaded every release.
#   * The db name must itself carry a semver: praetor-1.0.0.db, which users
#     reference as [praetor-1.0.0]. 1.0.0 is the repo LAYOUT version, not the
#     app version; bump it only if the layout changes.
#
# Required env:
#   VERSION                release version WITHOUT the leading v (e.g. 0.4.4)
#   BUILDKITE_FILES_TOKEN  registry token with read_packages + write_packages
#                          + delete_packages (not needed with DRY_RUN)
# Optional:
#   BK_ORG        Buildkite org slug        (default cybergodzilla-2099)
#   BK_REGISTRY   Files registry slug       (default praetor-arch)
#   REPO_DB       repo db basename          (default praetor-1.0.0)
#   DIST          output directory          (default dist/arch)
#   DRY_RUN       if set, build into $DIST and skip the network entirely
#   PUBLISH_ONLY  if set, skip the build and publish what is already in $DIST
set -euo pipefail

: "${VERSION:?}"
BK_ORG="${BK_ORG:-cybergodzilla-2099}"
BK_REGISTRY="${BK_REGISTRY:-praetor-arch}"
REPO_DB="${REPO_DB:-praetor-1.0.0}"
DIST="${DIST:-dist/arch}"

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$here/../.."   # nfpm.yaml paths are relative to the repo root

PKG="praetor-${VERSION}-1.x86_64.pkg.tar.zst"

if [[ -z "${PUBLISH_ONLY:-}" ]]; then
  rm -rf "$DIST" && mkdir -p "$DIST"
  PKG_ARCH=amd64 VERSION="$VERSION" \
    nfpm pkg --config packaging/linux/nfpm.yaml --packager archlinux --target "$DIST/"
  mv "$DIST/praetor-${VERSION}-1-x86_64.pkg.tar.zst" "$DIST/$PKG"

  # Fresh one-package db. repo-add writes <db>.tar.gz plus a <db> symlink;
  # the upload needs a regular file named exactly <REPO_DB>.db.
  ( cd "$DIST" && repo-add -q "${REPO_DB}.db.tar.gz" "$PKG" )
  cp --remove-destination "$DIST/${REPO_DB}.db.tar.gz" "$DIST/${REPO_DB}.db"
  # .files is `pacman -F` data; nothing serves it.
  rm -f "$DIST/${REPO_DB}.files" "$DIST/${REPO_DB}.files.tar.gz"
  ls -la "$DIST"
fi

if [[ -n "${DRY_RUN:-}" ]]; then
  echo "DRY_RUN: built into $DIST; skipping upload."
  exit 0
fi

# ---- network half (Task 2) ------------------------------------------------
