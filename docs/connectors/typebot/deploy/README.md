# Typebot + HorneroDB Forge block (custom image)

Build a Docker image of [Typebot](https://github.com/baptisteArno/typebot.io) that ships the **HorneroDB** forge block (defined in `../src/`) baked in. No `docker cp`, no entrypoint hacks — the block is part of the image and survives `docker compose up`.

## Files in this folder

| File | What it is |
|---|---|
| `Dockerfile.hornerodb-builder` | Multi-stage build (bun + nx) based on the official Typebot Dockerfile. Adds the HorneroDB block before `bun install`. |
| `build.sh` | Helper that clones the Typebot monorepo (or reuses a local one), copies the HorneroDB block into `packages/forge/blocks/hornerodb`, and runs `docker build`. |
| `docker-compose.yml` | Postgres + Redis + Typebot builder/viewer using the custom image. |
| `README.md` | This file. |

## One-time setup on the Oracle Cloud server

```bash
ssh ubuntu@<your-server>

# 1) Create a working dir
mkdir -p ~/typebot-deploy && cd ~/typebot-deploy

# 2) Clone Typebot (the build needs the full monorepo)
git clone --depth 1 https://github.com/baptisteArno/typebot.io.git
# (or use your fork / a pinned tag like: git clone --branch v3.17.1 ...)

# 3) Clone this repo (or just copy the deploy/ + the block folder)
git clone --depth 1 https://github.com/hornerodb/hornerodb.git
# now you have:
#   ~/typebot-deploy/typebot.io/
#   ~/typebot-deploy/hornerodb/docs/connectors/typebot/

# 4) Build the custom image
cd hornerodb/docs/connectors/typebot/deploy
./build.sh
# → builds `typebot-builder-hornerodb:latest`
```

## Bring up the stack

```bash
cd ~/typebot-deploy/hornerodb/docs/connectors/typebot/deploy
cp docker-compose.yml ~/typebot-deploy/typebot-compose.yml
cd ~/typebot-deploy
# Create the .env file Typebot expects
cat > .env <<'EOF'
ADMIN_EMAIL=you@example.com
NEXTAUTH_SECRET=run-openssl-rand-base64-32
NEXTAUTH_URL=http://<server-ip>:8080
ENCRYPTION_SECRET=run-openssl-rand-base64-32
EOF
docker compose -f typebot-compose.yml up -d
```

Then open `http://<server-ip>:8080`. In the editor sidebar you should see the **HorneroDB** card with the orange "H" logo.

## Updating the block

After editing files in `../src/`:

```bash
cd ~/typebot-deploy/hornerodb/docs/connectors/typebot/deploy
./build.sh
cd ~/typebot-deploy
docker compose -f typebot-compose.yml up -d --force-recreate typebot-builder
```

## How the build works

The official Typebot Dockerfile copies `.next/standalone` from the nx build output into the release image. The forge blocks are statically imported by the builder app, so once they're part of the monorepo at `packages/forge/blocks/hornerodb/`, `bun install` registers the workspace package and `nx build` bundles it into the standalone output. No runtime patches, no `docker cp`, no `pnpm install` at boot.

## Troubleshooting

- **Card doesn't appear**: the most common cause is a TypeScript error in the block failing the nx build. Check the build output (`build.sh` runs in the terminal) and re-run after fixing.
- **`Cannot find module '@typebot.io/forge'`**: the block was copied but nx didn't pick it up. Make sure the destination is `packages/forge/blocks/hornerodb` and that the folder contains a `package.json` with `"name": "@typebot.io/hornerodb-block"`.
- **Typebot can't reach HorneroDB**: if both run in the same `docker compose`, use the service name (`http://hornerodb-server:8090` or whatever you named it). If HorneroDB is on the host, use `http://host.docker.internal:8090` on Docker Desktop, or the host's LAN IP on Linux.
