// Configuración del sitio de demo
const CONFIG = {
  // URL de HorneroDB
  API_BASE: 'http://localhost:2452/api/v1',
  
  // Workspace blest
  WORKSPACE_ID: 'blest',
  
  // API key para acceso público (se crea en HorneroDB)
  API_KEY: 'YOUR_API_KEY_HERE',
  
  // Rate limit recomendado para público: 10 req/min
  RATE_LIMIT: 10,
  
  // Dominios permitidos
  ALLOWED_ORIGINS: ['http://localhost:2452', 'http://localhost:2452/admin'],
  
  // Tablas usadas
  TABLES: {
    TURNOS: 'turnos',
    SERVICIOS: 'servicios',
    HORARIOS: 'horarios'
  }
};
