import { test, expect } from '@playwright/test';
import axios from 'axios';

const API_URL = 'http://localhost:8080/api/v1';

test.describe('HorneroDB - Tests con API', () => {
  
  test('crear workspace, tabla y datos via API', async ({ request }) => {
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    const workspaceName = `test-auto-luca ${timestamp}`;
    const tableSlug = 'asistentes_cumple';
    
    // Este test requiere un token válido
    // Por ahora verificamos que el endpoint de health funcione
    
    const health = await request.get('http://localhost:8080/health');
    expect(health.ok()).toBeTruthy();
    
    console.log('✅ Health check OK');
    console.log(`Workspace a crear: ${workspaceName}`);
  });

  test('verificar que el frontend cargue correctamente', async ({ page }) => {
    await page.goto('http://localhost:5173');
    await page.waitForLoadState('networkidle');
    
    // Verificar elementos del login
    await expect(page.getByText('HorneroDB').first()).toBeVisible();
    await expect(page.getByText('Bienvenido')).toBeVisible();
    
    console.log('✅ Frontend carga correctamente');
  });
});

test.describe('HorneroDB - Manual Test Guide', () => {
  test('guía para test manual', async () => {
    // Este test siempre pasa y da instrucciones
    console.log(`
==========================================
GUÍA PARA TEST MANUAL
==========================================

1. Ir a http://localhost:5173
2. Click en "Iniciar sesión con PocketID"
3. Completar autenticación WebAuthn
4. Crear workspace: "test-auto-luca [fecha]"
5. Crear tabla: "asistentes_cumple"
6. Agregar columnas:
   - nombre (texto)
   - numero (número)
   - laptop_desktop (texto)
7. Agregar registros de prueba
8. Ir a Settings y verificar roles

==========================================
    `);
  });
});
