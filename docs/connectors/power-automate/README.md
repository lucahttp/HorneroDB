# HorneroDB Power Automate Connection

This directory contains the custom connector definitions for **Microsoft Power Automate**, **Power Apps**, and **Logic Apps**.

It uses OAuth 2.0 (PocketID) by default but allows an API Key override for backend-service tasks using policy templates.

## Deployment Instructions

1. Install the Power Automate CLI:
   ```bash
   pip install paconn
   ```

2. Login to your Power Automate Environment:
   ```bash
   paconn login
   ```

3. Create the Custom Connector (Run this from within this folder):
   ```bash
   paconn create --api-prop apiProperties.json --api-def apiDefinition.swagger.json
   ```

4. Go to [make.powerautomate.com](https://make.powerautomate.com/), open **Custom Connectors**, edit your new connection, go to the **Security** tab, and enter your PocketID `Client ID` and `Client Secret`.

## Security Recommendations
For Power Automate flows, we recommend using a dedicated API Key with a role that leverages the `__system__` permission namespace. For example, to allow a flow to manage its own webhooks:

```json
{
  "__system__": {
    "webhooks": "manage"
  },
  "*": {
    "read": "all",
    "create": "all"
  }
}
```

This keeps your Power Automate integrations secure and limited to only the necessary tables and system actions.
