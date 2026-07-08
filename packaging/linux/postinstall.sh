#!/bin/sh
# Best-effort: refresh the desktop database so the Praetor GUI appears in the
# applications menu right after install/upgrade (rather than only after the
# next login). Everything here is optional and must never fail the package
# install — hence no `set -e` and `|| true` on every command.
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi
exit 0
