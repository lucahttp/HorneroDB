#!/usr/bin/env bash
# Build the THIN image (~30s, ~50 MB extra) by layering the HorneroDB block
# on top of the official baptistearno/typebot-builder:latest.
#
# Usage:
#   ./build-thin.sh [output_image] [typebot_base_image]
#
# Examples:
#   ./build-thin.sh                                              # default base
#   ./build-thin.sh my/typebot-builder:v3.18 baptistearno/typebot-builder:v3.18.0
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUTPUT="${1:-typebot-builder-hornerodb:latest}"
TYPEBOT_BASE="${2:-baptistearno/typebot-builder:latest}"

# 1) Stage the block source at a stable name so the Dockerfile can COPY it.
#    We exclude the `deploy/` folder and any node_modules.
echo "▶ Staging HorneroDB block at $HERE/hornerodb-block"
rm -rf "$HERE/hornerodb-block"
mkdir -p "$HERE/hornerodb-block"
cp -R "$HERE/../src" "$HERE/hornerodb-block/src"
cp "$HERE/../package.json" "$HERE/hornerodb-block/package.json"
cp "$HERE/../tsconfig.json" "$HERE/hornerodb-block/tsconfig.json" 2>/dev/null || true

# 2) Pull the base image so the next build is incremental
echo "▶ Pulling $TYPEBOT_BASE"
docker pull --quiet "$TYPEBOT_BASE" || true

# 3) Build with the thin Dockerfile
echo "▶ Building $OUTPUT (FROM $TYPEBOT_BASE)"
docker build \
  -f "$HERE/Dockerfile.hornerodb-thin" \
  --build-arg TYPEBOT_IMAGE="$TYPEBOT_BASE" \
  -t "$OUTPUT" \
  "$HERE"

# 4) Save to tar (optional, for scp to the server)
OUTPUT_TAR="${OUTPUT//[:\/]/_}.tar"
echo "▶ Saving image to $OUTPUT_TAR"
docker save "$OUTPUT" -o "$OUTPUT_TAR"

echo
echo "✅ Listo. Imagen: $OUTPUT  ($(du -h "$OUTPUT_TAR" | cut -f1))"
echo "   Para deployar: scp $OUTPUT_TAR ubuntu@<ip>:~/ && ssh ubuntu@<ip> 'docker load -i ~/$OUTPUT_TAR'"

# 5) Limpiar el staging
rm -rf "$HERE/hornerodb-block"
