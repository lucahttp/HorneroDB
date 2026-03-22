# n8n-nodes-hornerodb

This is an n8n custom node package for **HorneroDB**. It allows you to:
- Authenticate via API Key or PocketID OAuth2.
- Perform standard Database Actions (Read, Write, Update, Delete).
- Subscribe to real-time Webhook Triggers when data changes in HorneroDB.

## Installation for Local n8n Installs
If you are running n8n via npm or locally:

```bash
npm install
npm run build
npm link
```
Then in your local n8n install directory:
```bash
npm link n8n-nodes-hornerodb
n8n start
```

## Security & Permissions
HorneroDB supports granular permissions for automated workflows. We recommend creating a dedicated role for n8n using the `__system__` namespace:

```json
{
  "__system__": {
    "webhooks": "manage",
    "tables": "view"
  },
  "*": {
    "read": "all",
    "create": "all"
  }
}
```
This ensures n8n can manage its own webhooks and access data while being restricted from sensitive actions like deleting roles or managing API keys.

#### Backward Compatibility
Roles named exactly `admin` will continue to have full system access for all actions.

## Docker Installs
If you are running n8n in Docker, you'll need to mount this compiled `dist/` directory into the `/home/node/.n8n/custom/` folder of the n8n container, or publish it to NPM and define `N8N_CUSTOM_EXTENSIONS="n8n-nodes-hornerodb"` in your docker-compose.



## Install n8n in docker

```bash
docker volume create n8n_data

docker run -it --rm \
 --name n8n \
 -p 5678:5678 \
 -e GENERIC_TIMEZONE="America/Argentina/Buenos_Aires" \
 -e TZ="America/Argentina/Buenos_Aires" \
 -e N8N_ENFORCE_SETTINGS_FILE_PERMISSIONS=true \
 -e N8N_RUNNERS_ENABLED=true \
 -v n8n_data:/home/node/.n8n \
 docker.n8n.io/n8nio/n8n
```