const baseUrl = import.meta.env.VITE_SERVER_PUBLIC_URL || (import.meta.env.DEV ? 'http://localhost:8080' : '');
export const API_URL = `${baseUrl}/api/v1`;
export const AUTH_TOKEN_KEY = 'hornero_token';