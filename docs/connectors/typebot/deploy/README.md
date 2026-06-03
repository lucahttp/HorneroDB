# Typebot + HorneroDB Forge block — deployment

Two build paths are shipped here. **Start with `build-thin.sh`**; only fall
back to the full source build if the thin path doesn't pick up your block.

| Script | Build time | RAM | What it does |
|---|---|---|---|
| `build-thin.sh` + `Dockerfile.hornerodb-thin` | ~30 s | ~50 MB | Layer the HorneroDB block on top of the OFFICIAL `baptistearno/typebot-builder:latest`. Tracks upstream releases by changing one ARGs. |
| `build.sh` + `Dockerfile.hornerodb-builder` | 5-10 min | ~2 GB | Rebuild the whole Typebot monorepo from source. Last-resort option. |

## Why a custom image is the only option

Typebot ships its builder as a Next.js **standalone bundle** (`output: "standalone"` in `apps/builder/next.config.mjs`) — forge blocks are tree-shaken into the image at build time. There is no runtime plugin loader, so the only way to add a new block in self-hosted mode is to layer a custom image on top. The maintainer [confirms this in #2490](https://github.com/baptisteArno/typebot.io/issues/2490).

## Option A — Thin image (recommended)

```bash
# 1. Build on your Mac (or any machine with docker)
cd docs/connectors/typebot/deploy
./build-thin.sh myorg/typebot-hornerodb:latest

# 2. Ship to Oracle
scp myorg_typebot-hornerodb_latest.tar ubuntu@<ip>:~/
ssh ubuntu@<ip> 'docker load -i ~/myorg_typebot-hornerodb_latest.tar'
```

The thin Dockerfile is two `COPY` instructions over the official image, so:
- **Tracking upstream**: change `TYPEBOT_IMAGE` to the newer tag, rerun `build-thin.sh`. Done.
- **Updating the block**: edit `../src/...`, rerun `build-thin.sh`. Done.
- **No 5-10 min rebuild, no 2 GB RAM, no cloning the 1.5 GB Typebot monorepo.**

### Caveat: when the thin path doesn't work

Next.js standalone traces imports at build time. If the trace didn't follow a path that reaches the block, the runtime won't see it. This is rare for forge blocks (they're always imported by the block registry that lives in `apps/builder`), but if it happens you need Option B.

## Option B — Full source build (fallback)

```bash
# One-time: clone Typebot
git clone --depth 1 https://github.com/baptisteArno/typebot.io.git
cd typebot.io   # workdir

# Build (5-10 min, ~2 GB RAM)
../hornerodb/docs/connectors/typebot/deploy/build.sh
```

This clones the monorepo, drops the block into `packages/forge/blocks/hornerodb/`, runs `bun install` + `bunx nx build`, and produces a tarball. Use this when the thin path fails.

## Deploy on the server

```bash
ssh ubuntu@<ip>

# Env file
mkdir -p ~/typebot-deploy && cd ~/typebot-deploy
cat > .env <<'EOF'
ADMIN_EMAIL=you@example.com
NEXTAUTH_SECRET=$(openssl rand -base64 32)
NEXTAUTH_URL=http://<ip>:8080
ENCRYPTION_SECRET=$(openssl rand -base64 32)
# v3.17+ SSRF: allow internal service names if HorneroDB is on the same network
SSRF_ALLOWED_HOSTS=hornerodb
EOF

# Compose file (or curl from raw.githubusercontent)
curl -O https://raw.githubusercontent.com/hornerodb/hornerodb/main/docs/connectors/typebot/deploy/docker-compose.yml
docker compose up -d
```

Open `http://<ip>:8080`. The **HorneroDB** card with the orange "H" logo should appear in the integrations panel.

## Updating the block

```bash
# On your Mac
cd docs/connectors/typebot/deploy
./build-thin.sh myorg/typebot-hornerodb:latest

# On the server
scp myorg_typebot-hornerodb_latest.tar ubuntu@<ip>:~/
ssh ubuntu@<ip> 'cd ~/typebot-deploy && \
  docker load -i ~/myorg_typebot-hornerodb_latest.tar && \
  docker compose up -d --force-recreate typebot-builder typebot-viewer'
```

## Tracking upstream Typebot

```bash
cd typebot.io   # or wherever you have the clone for Option B
git pull origin main
cd ../hornerodb/docs/connectors/typebot/deploy
./build-thin.sh myorg/typebot-hornerodb:latest baptistearno/typebot-builder:NEW_TAG
```

## Troubleshooting

- **Card doesn't appear after thin build**: jump to Option B (full source build). The block must compile in context of the real nx graph to be traceable.
- **`Cannot find module '@typebot.io/forge'`**: the block's `package.json` doesn't match the workspace name. Make sure it reads `"name": "@typebot.io/hornerodb-block"`.
- **Typebot can't reach HorneroDB**: v3.17+ blocks private IP ranges by default. Set `SSRF_ALLOWED_HOSTS=hornerodb` (or the service name) on the builder and viewer.
- **OOM during build**: only applies to Option B. Bump swap to 4 GB or build on your Mac.
- **Nx `failed to parse turbo json` (issue #1201)**: the block's `tsconfig.json` was malformed. The shipped config is clean.
