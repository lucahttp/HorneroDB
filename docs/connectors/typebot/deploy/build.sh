#!/usr/bin/env bash
# Build the custom typebot-builder image that ships the HorneroDB forge block.
#
# Layout expected in the current working directory:
#   ./typebot.io/                 ← clone of github.com/baptisteArno/typebot.io
#   ./hornerodb/docs/connectors/typebot/  ← THIS folder's sibling
#
# Or use TYPEBOT_SRC / HORNERODB_SRC env vars to point elsewhere.
set -euo pipefail

SCOPE="${SCOPE:-builder}"
TAG="${TAG:-typebot-builder-hornerodb:latest}"

TYPEBOT_SRC="${TYPEBOT_SRC:-$PWD/typebot.io}"
HORNERODB_SRC="${HORNERODB_SRC:-$PWD/hornerodb}"

if [ ! -d "$TYPEBOT_SRC" ]; then
  echo "❌ Typebot source not found at $TYPEBOT_SRC"
  echo "   Clona con: git clone --depth 1 https://github.com/baptisteArno/typebot.io.git"
  exit 1
fi

if [ ! -d "$HORNERODB_SRC/docs/connectors/typebot/src" ]; then
  echo "❌ HorneroDB connector not found at $HORNERODB_SRC/docs/connectors/typebot"
  exit 1
fi

DOCKERFILE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "▶ Building $TAG"
echo "  typebot src:  $TYPEBOT_SRC"
echo "  hornerodb src: $HORNERODB_SRC"
echo "  scope:         $SCOPE"
echo

# Copy the HorneroDB forge block into the Typebot monorepo so the COPY
# instruction in the Dockerfile can find it at a deterministic path.
DEST="$TYPEBOT_SRC/packages/forge/blocks/hornerodb"
mkdir -p "$DEST"
rm -rf "$DEST"
cp -R "$HORNERODB_SRC/docs/connectors/typebot" "$DEST"

# We build from the typebot.io root so the Dockerfile can COPY . over the
# monorepo. The Dockerfile references docs/connectors/typebot/ at its own
# context — we resolve that by symlinking the block from inside the build
# context.
ln -sfn "$DEST" "$TYPEBOT_SRC/docs-connectors-typebot-symlink" 2>/dev/null || true

docker build \
  -f "$DOCKERFILE_DIR/Dockerfile.hornerodb-builder" \
  --build-arg SCOPE="$SCOPE" \
  -t "$TAG" \
  "$TYPEBOT_SRC"

echo
echo "✅ Built image: $TAG"
echo "   Run: docker run --rm $TAG"
