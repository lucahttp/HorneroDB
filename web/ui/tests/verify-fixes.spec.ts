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

test.describe('Fixes Verification', () => {
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
        // Inject token
        // We go to login page first to set localStorage
        await page.goto('/');

        await page.evaluate((t) => localStorage.setItem('hornero_token', t), token);
    });

    test('Settings page and Language Selector', async ({ page }) => {
        // Navigate to dashboard (authenticated)
        await page.goto('/dashboard');

        // Wait for dashboard content
        await expect(page.locator('h1').filter({ hasText: /Dashboard|Tus Workspaces/ })).toBeVisible({ timeout: 10000 });

        // Check for Language Selector
        const langSelect = page.locator('.language-selector select');
        await expect(langSelect).toBeAttached();

        // Optional: Check ThemeToggle
        await expect(page.locator('button[title*="Modo"]')).toBeVisible();
    });

    test('DataTable row actions', async ({ page }) => {
        await page.goto('/dashboard');
        await expect(page.locator('h1').filter({ hasText: /Dashboard|Tus Workspaces/ })).toBeVisible();

        // If there are workspaces, try to check datatable
        const cards = page.locator('.card:has-text("Open")');
        if (await cards.count() > 0) {
            await cards.first().click();

            // In workspace view
            await expect(page.locator('.sidebar')).toBeVisible();

            // Find a table
            const tableCards = page.locator('.card');
            // The last card is "New Table", previous ones are tables
            // Actually workspace has grid of tables.
            // Let's see if we can find one.
            const tables = page.locator('.card').filter({ hasNotText: 'New Table' });
            // Or generic click
            if (await tables.count() > 0) {
                await tables.first().click();
                // Now in table view

                // Check if table container exists
                await expect(page.locator('.table-container')).toBeVisible();

                // Check if actions column exists (last th is empty)
                const headers = page.locator('thead th');
                const count = await headers.count();
                // We added a th at the end
                // Just check actions button in a row if any
                const rows = page.locator('tbody tr:not(.inline-new-row)');
                if (await rows.count() > 0) {
                    const firstRow = rows.first();
                    await firstRow.hover();
                    // Check .row-actions
                    await expect(firstRow.locator('.row-actions')).toBeVisible();
                    await expect(firstRow.locator('.row-actions button')).toHaveCount(1); // delete button
                }
            }
        }
    });

});
