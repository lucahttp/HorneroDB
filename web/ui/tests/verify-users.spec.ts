import { test, expect } from '@playwright/test';
import { createHmac } from 'crypto';

const TEST_USER_ID = '00000000-aaaa-bbbb-cccc-000000000001';
const JWT_SECRET = process.env.JWT_SECRET || 'your-super-secret-jwt-key-change-in-production';

async function makeJWT(secret, payload) {
    const header = { alg: 'HS256', typ: 'JWT' };
    const b64 = (obj) => Buffer.from(JSON.stringify(obj)).toString('base64url');
    const unsigned = `${b64(header)}.${b64(payload)}`;
    const sig = createHmac('sha256', secret).update(unsigned).digest('base64url');
    return `${unsigned}.${sig}`;
}

test.describe('User Management UI', () => {
    let token;

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

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await page.evaluate((t) => localStorage.setItem('hornero_token', t), token);
    });

    test('Users tab should be visible and functional', async ({ page }) => {
        // Mock API responses
        await page.route('**/api/v1/workspaces', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    success: true,
                    data: [{ id: 'ws-123', name: 'Test Workspace', owner_id: TEST_USER_ID, slug: 'test-ws' }]
                })
            });
        });

        await page.route('**/api/v1/workspaces/ws-123', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    success: true,
                    data: { id: 'ws-123', name: 'Test Workspace', owner_id: TEST_USER_ID, slug: 'test-ws' }
                })
            });
        });

        await page.route('**/api/v1/workspaces/ws-123/tables', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    success: true,
                    data: []
                })
            });
        });

        await page.route('**/api/v1/workspaces/ws-123/roles', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    success: true,
                    data: [{ id: 'role-1', name: 'Editor' }]
                })
            });
        });

        await page.route('**/api/v1/workspaces/ws-123/users', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    success: true,
                    data: [
                        { id: TEST_USER_ID, name: 'Admin User', email: 'admin@hornero.dev', role_name: 'admin', picture: '' },
                        { id: 'user-2', name: 'Guest User', email: 'guest@hornero.dev', role_name: 'Editor', picture: '' }
                    ]
                })
            });
        });

        await page.route('**/api/v1/auth/me', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    success: true,
                    data: { id: TEST_USER_ID, email: 'test@hornero.dev', role: 'admin' }
                })
            });
        });

        // Keys fetch might happen too if we are lazy, but usually not unless tab clicked.
        // Roles mock is enough for initial load (activeSection='roles')

        await page.goto('/dashboard');
        await page.click('.card');

        // Go to Settings
        // Use a more generic selector if translation fails, but usually icon or position.
        // Click element with text containing "Configuraci" OR "Settings"
        await page.locator('button', { hasText: /Configuraci|Settings/ }).click();

        // Check Users tab
        const usersTab = page.locator('button.tab').filter({ hasText: /Usuarios|Users/ }).first();
        await expect(usersTab).toBeVisible();

        // Click and Wait for Request
        const responsePromise = page.waitForResponse('**/api/v1/workspaces/ws-123/users');
        await usersTab.click();
        const response = await responsePromise;
        expect(response.status()).toBe(200);

        // Check Users list content
        await expect(page.locator('text=Admin User')).toBeVisible();
        await expect(page.locator('text=guest@hornero.dev')).toBeVisible();

        // Check "Add User" button
        const addButton = page.locator('button', { hasText: /Agregar Usuario|Add User/ }).first();
        await expect(addButton).toBeVisible();
        await addButton.click();

        // Check Modal
        await expect(page.getByRole('heading', { name: /Importar Usuario|Import User/i })).toBeVisible();
        await expect(page.locator('input[type="email"]')).toBeVisible();

        // Close modal
        await page.locator('button', { hasText: /Cancelar|Cancel/ }).click();
    });
});
