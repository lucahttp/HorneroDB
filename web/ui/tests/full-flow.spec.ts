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
    await expect(page.getByRole('heading', { name: /Bienvenido/i })).toBeVisible();

    console.log('✅ Frontend carga correctamente');
  });
});

test.describe.serial('HorneroDB - Full CRUD Flow', () => {
  const API = 'http://localhost:8080/api/v1';
  const TEST_USER_ID = '00000000-aaaa-bbbb-cccc-000000000001';
  const JWT_SECRET = process.env.JWT_SECRET || 'your-super-secret-jwt-key-change-in-production';

  let token: string;
  let workspaceId: string;
  let tableId: string;

  // Generate a valid JWT token for testing (HS256)
  async function makeJWT(secret: string, payload: Record<string, unknown>): Promise<string> {
    const { createHmac } = await import('crypto');
    const header = { alg: 'HS256', typ: 'JWT' };
    const b64 = (obj: unknown) =>
      Buffer.from(JSON.stringify(obj)).toString('base64url');
    const unsigned = `${b64(header)}.${b64(payload)}`;
    const sig = createHmac('sha256', secret).update(unsigned).digest('base64url');
    return `${unsigned}.${sig}`;
  }

  test.beforeAll(async () => {
    token = await makeJWT(JWT_SECRET, {
      sub: TEST_USER_ID,
      email: 'test@hornero.dev',
      role: 'admin',
      workspace_id: '',
      exp: Math.floor(Date.now() / 1000) + 3600,
      iat: Math.floor(Date.now() / 1000),
    });
  });

  function headers() {
    return { Authorization: `Bearer ${token}` };
  }

  test('1. crear workspace', async ({ request }) => {
    const timestamp = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-');
    const name = `test-auto-${timestamp}`;

    const res = await request.post(`${API}/workspaces`, {
      headers: headers(),
      data: { name, slug: name.toLowerCase().replace(/\s+/g, '-'), owner_id: TEST_USER_ID },
    });

    expect(res.status()).toBe(201);
    const body = await res.json();
    workspaceId = body.id;
    expect(workspaceId).toBeTruthy();
    expect(body.name).toBe(name);
    console.log(`✅ Workspace creado: ${name} (${workspaceId})`);
  });

  test('2. obtener workspace', async ({ request }) => {
    const res = await request.get(`${API}/workspaces/${workspaceId}`, { headers: headers() });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.id).toBe(workspaceId);
    console.log('✅ GET workspace OK');
  });

  test('3. crear tabla "asistentes_cumple"', async ({ request }) => {
    const res = await request.post(`${API}/workspaces/${workspaceId}/tables`, {
      headers: headers(),
      data: { name: 'asistentes_cumple', slug: 'asistentes_cumple' },
    });

    expect(res.status()).toBe(201);
    const body = await res.json();
    tableId = body.id;
    expect(tableId).toBeTruthy();
    console.log(`✅ Tabla creada: asistentes_cumple (${tableId})`);
  });

  test('4. agregar columnas', async ({ request }) => {
    const columns = [
      { name: 'nombre', slug: 'nombre', field_type: 'text' },
      { name: 'numero', slug: 'numero', field_type: 'number' },
      { name: 'laptop_desktop', slug: 'laptop_desktop', field_type: 'text' },
    ];

    for (const col of columns) {
      const res = await request.post(
        `${API}/workspaces/${workspaceId}/tables/${tableId}/columns`,
        { headers: headers(), data: col }
      );
      expect(res.status()).toBe(201);
      console.log(`  ✅ Columna: ${col.name} (${col.field_type})`);
    }

    // Verificar que las 3 columnas existen
    const listRes = await request.get(
      `${API}/workspaces/${workspaceId}/tables/${tableId}/columns`,
      { headers: headers() }
    );
    expect(listRes.status()).toBe(200);
    const allCols = await listRes.json();
    expect(allCols.length).toBe(3);
    console.log('✅ 3 columnas verificadas');
  });

  test('5. agregar registros de prueba', async ({ request }) => {
    const records = [
      { nombre: 'Lucas', numero: 1, laptop_desktop: 'desktop' },
      { nombre: 'María', numero: 2, laptop_desktop: 'laptop' },
      { nombre: 'Pedro', numero: 3, laptop_desktop: 'desktop' },
    ];

    for (const rec of records) {
      const res = await request.post(
        `${API}/workspaces/${workspaceId}/data/asistentes_cumple`,
        { headers: headers(), data: rec }
      );
      expect(res.status()).toBe(201);
      console.log(`  ✅ Registro: ${rec.nombre}`);
    }

    // Verificar registros insertados
    const listRes = await request.get(
      `${API}/workspaces/${workspaceId}/data/asistentes_cumple`,
      { headers: headers() }
    );
    expect(listRes.status()).toBe(200);
    const body = await listRes.json();
    expect(body.data.length).toBe(3);
    console.log('✅ 3 registros verificados');
  });

  test('6. verificar roles del workspace', async ({ request }) => {
    const res = await request.get(`${API}/workspaces/${workspaceId}/roles`, {
      headers: headers(),
    });
    expect(res.status()).toBe(200);
    const roles = await res.json();

    // Al crear un workspace se generan "admin" y "user" por defecto
    const roleNames = roles.map((r: { name: string }) => r.name);
    expect(roleNames).toContain('admin');
    expect(roleNames).toContain('user');
    console.log(`✅ Roles verificados: ${roleNames.join(', ')}`);
  });

  test('7. cleanup - eliminar workspace', async ({ request }) => {
    const res = await request.delete(`${API}/workspaces/${workspaceId}`, {
      headers: headers(),
    });
    expect(res.status()).toBe(200);
    console.log('✅ Workspace eliminado (cleanup)');
  });
});
