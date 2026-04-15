#!/bin/sh
set -e

REMOVED=0

remove() {
  if [ -f "$1" ]; then
    if [ -w "$(dirname "$1")" ]; then
      rm -f "$1"
    else
      sudo rm -f "$1"
    fi
    echo "Removed: $1"
    REMOVED=1
  fi
}

remove "/usr/local/bin/crewalk"
remove "$HOME/.local/bin/crewalk"
remove "$(go env GOPATH 2>/dev/null)/bin/crewalk" 2>/dev/null || true
remove "$HOME/go/bin/crewalk"

if [ "$REMOVED" -eq 0 ]; then
  echo "crewalk not found in common locations."
  echo "If installed elsewhere, remove it manually: which crewalk"
else
  echo "Done."
fi
