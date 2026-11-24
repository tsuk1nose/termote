#!/bin/sh
# Offline local installer for Termote.
# Works either from an extracted `make dist` tarball (copied via scp to a target
# host) or directly from a built local git checkout. Never contacts the network.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/termote"
BIN_DIR="${TERMOTE_BIN_DIR:-$HOME/.local/bin}"

die() { echo "install-local: $*" >&2; exit 1; }

# Locate the binary: either inside an extracted dist bundle (./bin/termote)
# or in a local checkout (../server/termote).
SRC_BIN=""
if [ -f "$SCRIPT_DIR/bin/termote" ]; then
  SRC_BIN="$SCRIPT_DIR/bin/termote"
  SHA_FILE="$SCRIPT_DIR/termote.sha256"
  if [ -f "$SHA_FILE" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      (cd "$SCRIPT_DIR" && sha256sum -c termote.sha256)
    elif command -v shasum >/dev/null 2>&1; then
      (cd "$SCRIPT_DIR" && shasum -a 256 -c termote.sha256)
    fi
  fi
elif [ -f "$SCRIPT_DIR/../server/termote" ]; then
  SRC_BIN="$SCRIPT_DIR/../server/termote"
else
  die "termote binary not found (run 'make build' first, or run from an extracted 'make dist' bundle)"
fi

chmod +x "$SRC_BIN"

VERSION="${TERMOTE_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION="$("$SRC_BIN" version 2>/dev/null | sed -n 's/^Termote v\{0,1\}//p' | head -n 1 | tr -d '[:space:]')"
fi
[ -n "$VERSION" ] || VERSION="local"

TARGET_DIR="$DATA_DIR/versions/$VERSION"
mkdir -p "$TARGET_DIR/bin" "$BIN_DIR"

# Atomic replace so an already-running binary never fails with ETXTBSY.
TMP_BIN="$TARGET_DIR/bin/termote.new.$$"
cp "$SRC_BIN" "$TMP_BIN"
chmod 755 "$TMP_BIN"
mv -f "$TMP_BIN" "$TARGET_DIR/bin/termote"

ln -sfn "$TARGET_DIR" "$DATA_DIR/current"
ln -sfn "$DATA_DIR/current/bin/termote" "$BIN_DIR/termote"

echo "Installed Termote v$VERSION -> $TARGET_DIR/bin/termote"
echo "Symlinked: $BIN_DIR/termote -> $DATA_DIR/current/bin/termote"
echo "Next: termote start (or: termote restart)"
