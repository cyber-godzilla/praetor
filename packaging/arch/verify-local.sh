#!/usr/bin/env bash
# Prove that pacman can sync + install from a DRY_RUN build of publish.sh.
# Mounts $DIST (default dist/arch) into an archlinux container as a file://
# repo using the same section name users will put in pacman.conf.
# Usage: DRY_RUN=1 VERSION=<ver> packaging/arch/publish.sh && packaging/arch/verify-local.sh
set -euo pipefail
DIST="${DIST:-dist/arch}"
REPO_DB="${REPO_DB:-praetor-1.0.0}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$here/../.."
[[ -f "$DIST/${REPO_DB}.db" ]] || { echo "no $DIST/${REPO_DB}.db — run DRY_RUN=1 publish.sh first" >&2; exit 1; }

docker run --rm -v "$(pwd)/$DIST:/repo:ro" archlinux:latest bash -euo pipefail -c "
  printf '[${REPO_DB}]\nSigLevel = Never\nServer = file:///repo\n' >> /etc/pacman.conf
  pacman -Sy --noconfirm >/dev/null
  pacman -Si praetor | grep -E 'Repository|Version|Architecture|Depends'
  pacman -S --noconfirm praetor >/dev/null
  pacman -Ql praetor | grep -E 'bin/praetor|applications/praetor.desktop|pixmaps/praetor.png'
  praetor-tui --version
"
