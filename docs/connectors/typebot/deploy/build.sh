#!/usr/bin/env bash
# Build the custom typebot-builder image that ships the HorneroDB forge block,
# and save it to a .tar file you can scp to the server.
#
# Layout expected in the current working directory:
#   ./typebot.io/                 ← clone of github.com/baptisteArno/typebot.io
#   ./hornerodb/docs/connectors/typebot/  ← this repo's connector
#
# Override locations with TYPEBOT_SRC / HORNERODB_SRC env vars.
#
# Usage: ./build.sh [output-tar]
# Default output-tar: typebot-builder-hornerodb.tar (in cwd)
set -euo pipefail

SCOPE="${SCOPE:-builder}"
TAG="${TAG:-typebot-builder-hornerodb:latest}"
OUTPUT_TAR="${1:-typebot-builder-hornerodb.tar}"

TYPEBOT_SRC="${TYPEBOT_SRC:-$PWD/typebot.io}"
HORNERODB_SRC="${HORNERODB_SRC:-$PWD/hornerodb}"
DOCKERFILE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ ! -d "$TYPEBOT_SRC" ]; then
  echo "❌ Typebot source not found at $TYPEBOT_SRC"
  echo "   Clona con: git clone --depth 1 https://github.com/baptisteArno/typebot.io.git"
  exit 1
fi

if [ ! -d "$HORNERODB_SRC/docs/connectors/typebot/src" ]; then
  echo "❌ HorneroDB connector not found at $HORNERODB_SRC/docs/connectors/typebot"
  exit 1
fi

echo "▶ Injecting HorneroDB block into the monorepo"
DEST="$TYPEBOT_SRC/packages/forge/blocks/hornerodb"
rm -rf "$DEST"
cp -R "$HORNERODB_SRC/docs/connectors/typebot" "$DEST"

echo
echo "▶ Building $TAG (this takes 5-10 minutes and ~2 GB RAM)"
echo "  typebot src:  $TYPEBOT_SRC"
echo "  scope:         $SCOPE"
echo

docker build \
  -f "$DOCKERFILE_DIR/Dockerfile.hornerodb-builder" \
  --build-arg SCOPE="$SCOPE" \
  -t "$TAG" \
  "$TYPEBOT_SRC"

echo
echo "▶ Saving image to $OUTPUT_TAR (~1.5 GB)"
docker save "$TAG" -o "$OUTPUT_TAR"

echo
echo "✅ Listo."
echo "   Para deployar en el server:"
echo "     scp $OUTPUT_TAR ubuntu@<ip>:~/"
echo "     ssh ubuntu@<ip> 'docker load -i ~/$OUTPUT_TAR'"
