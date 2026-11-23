const baseUrl = import.meta.env.VITE_SERVER_PUBLIC_URL || (import.meta.env.DEV ? 'http://localhost:8080' : '');
export const API_URL = `${baseUrl}/api/v1`;
export const AUTH_TOKEN_KEY = 'hornero_token';

/**
 * Event dispatched on `window` when the API answers 401 for a protected
 * endpoint. `AuthContext` listens to it and performs the actual logout, so
 * the Axios interceptor stays decoupled from the auth state.
 */
export const AUTH_EXPIRED_EVENT = 'hornero:session-expired';
