#!/bin/sh
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/currenjin/crewalk/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": "\(.*\)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Could not determine latest release. Check https://github.com/currenjin/crewalk/releases"
  exit 1
fi

URL="https://github.com/currenjin/crewalk/releases/download/${LATEST}/crewalk-${OS}-${ARCH}"

echo "Installing crewalk ${LATEST} (${OS}/${ARCH})..."
curl -fsSL "$URL" -o /tmp/crewalk
chmod +x /tmp/crewalk

DEST="${CREWALK_INSTALL_DIR:-/usr/local/bin}/crewalk"
if [ -w "$(dirname "$DEST")" ]; then
  mv /tmp/crewalk "$DEST"
else
  sudo mv /tmp/crewalk "$DEST"
fi

echo "Installed: $DEST"
crewalk --help 2>/dev/null || true
