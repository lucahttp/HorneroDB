# HorneroDB Typebot Integration

This directory contains the integration logic for **Typebot**. It allows you to use HorneroDB as a backend data store for your chat bots.

## Features
- **Fetch Records**: Read data from any table to personalize chat experiences.
- **Create/Update Records**: Save leads, support tickets, or user preferences directly from Typebot.
- **Dynamic Relations**: Support for relation columns to link bot data across tables.

## Security Configuration

For Typebot integrations, we recommend using a dedicated **API Key**. 

### Recommended Permissions
Create a specific role for your bot with the following permission structure:

```json
{
  "__system__": {
    "tables": "view"
  },
  "leads": {
    "create": "all",
    "read": "none"
  },
  "knowledge_base": {
    "read": "all"
  }
}
```

### Setup Instructions
1. Create an API Key in HorneroDB and assign it the specialized role.
2. In Typebot, use the **HTTP Request** block.
3. Set the `Authorization` header to `Bearer YOUR_API_KEY`.
4. Use the HorneroDB REST API endpoints (e.g., `POST /api/v1/workspaces/:id/data/:table`).
