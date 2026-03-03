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

## Docker Installs
If you are running n8n in Docker, you'll need to mount this compiled `dist/` directory into the `/home/node/.n8n/custom/` folder of the n8n container, or publish it to NPM and define `N8N_CUSTOM_EXTENSIONS="n8n-nodes-hornerodb"` in your docker-compose.
