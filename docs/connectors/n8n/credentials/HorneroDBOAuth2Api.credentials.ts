import { ICredentialType, INodeProperties } from "n8n-workflow";

export class HorneroDBOAuth2Api implements ICredentialType {
  name = "horneroDbOAuth2Api";
  extends = ["oAuth2Api"];
  displayName = "HorneroDB OAuth2 API";
  documentationUrl = "https://github.com/hornerodb/hornerodb";
  properties: INodeProperties[] = [
    {
      displayName: "HorneroDB Host URL",
      name: "host",
      type: "string",
      default: "https://api.hornerodb.com",
      required: true,
      description: "The base URL of your HorneroDB instance",
    },
    {
      displayName: "PocketID Base URL",
      name: "pocketIdHost",
      type: "string",
      default: "https://auth.hornerodb.com",
      required: true,
      description: "The base URL of your PocketID authentication server",
    },
    {
      displayName: "Authorization URL",
      name: "authUrl",
      type: "hidden",
      default: "={{$parameter.pocketIdHost}}/api/v1/auth/oidc/login",
    },
    {
      displayName: "Access Token URL",
      name: "accessTokenUrl",
      type: "hidden",
      default: "={{$parameter.pocketIdHost}}/api/v1/auth/oidc/token",
    },
    {
      displayName: "Scope",
      name: "scope",
      type: "hidden",
      default: "openid profile email offline_access",
    },
    {
      displayName: "Auth URI Query Parameters",
      name: "authQueryParameters",
      type: "hidden",
      default: "",
    },
    {
      displayName: "Authentication",
      name: "authentication",
      type: "hidden",
      default: "header",
    },
  ];
}
