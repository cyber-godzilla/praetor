#!/usr/bin/env bash
# Render the Homebrew cask (GUI) + formula (TUI) from templates and push them to
# the tap repo. Run on the macOS release runner after the GitHub release assets
# exist. Self-contained replacement for GoReleaser's `brews:` block.
#
# Required env:
#   VERSION              release version WITHOUT the leading v (e.g. 0.1.5)
#   GUI_ZIP              path to Praetor_<ver>_darwin_universal.zip
#   TUI_AMD64_TGZ        path to praetor-tui_<ver>_darwin_amd64.tar.gz
#   TUI_ARM64_TGZ        path to praetor-tui_<ver>_darwin_arm64.tar.gz
#   HOMEBREW_TAP_TOKEN   PAT with push access to the tap repo
# Optional:
#   TAP_REPO             owner/name of the tap (default cyber-godzilla/homebrew-tap)
#   DRY_RUN              if set, render into ./dist-brew and skip the push
set -euo pipefail

: "${VERSION:?}" "${GUI_ZIP:?}" "${TUI_AMD64_TGZ:?}" "${TUI_ARM64_TGZ:?}"
TAP_REPO="${TAP_REPO:-cyber-godzilla/homebrew-tap}"

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

sha() { shasum -a 256 "$1" | awk '{print $1}'; }
GUI_SHA="$(sha "$GUI_ZIP")"
TUI_AMD_SHA="$(sha "$TUI_AMD64_TGZ")"
TUI_ARM_SHA="$(sha "$TUI_ARM64_TGZ")"

render() { # <template> <out>
  sed -e "s|@VERSION@|${VERSION}|g" \
      -e "s|@GUI_SHA@|${GUI_SHA}|g" \
      -e "s|@TUI_DARWIN_AMD64_SHA@|${TUI_AMD_SHA}|g" \
      -e "s|@TUI_DARWIN_ARM64_SHA@|${TUI_ARM_SHA}|g" \
      "$1" > "$2"
}

if [[ -n "${DRY_RUN:-}" ]]; then
  mkdir -p dist-brew/Casks dist-brew/Formula
  render "$here/praetor.rb.tmpl"     dist-brew/Casks/praetor.rb
  render "$here/praetor-tui.rb.tmpl" dist-brew/Formula/praetor-tui.rb
  echo "DRY_RUN: rendered into ./dist-brew"
  exit 0
fi

: "${HOMEBREW_TAP_TOKEN:?}"
work="$(mktemp -d)"
git clone --depth 1 "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" "$work"

mkdir -p "$work/Casks" "$work/Formula"
render "$here/praetor.rb.tmpl"     "$work/Casks/praetor.rb"
render "$here/praetor-tui.rb.tmpl" "$work/Formula/praetor-tui.rb"

# Remove the old terminal `praetor` FORMULA — the name is now a cask (GUI) and
# the TUI lives at praetor-tui. Leaving both would make `brew install praetor`
# ambiguous.
rm -f "$work/Formula/praetor.rb"

cd "$work"
git config user.name "cyber-godzilla-bot"
git config user.email "cg@cybergodzilla.com"
git add -A
if git diff --cached --quiet; then
  echo "Tap already up to date for ${VERSION}; nothing to push."
  exit 0
fi
git commit -m "praetor ${VERSION}: cask (GUI) + praetor-tui formula"
git push origin HEAD
echo "Pushed tap update for ${VERSION}."
