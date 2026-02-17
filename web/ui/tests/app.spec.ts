import { test, expect } from '@playwright/test';

test.describe('HorneroDB UI', () => {
  test('login page loads correctly', async ({ page }) => {
    await page.goto('/');
    
    // Check for logo
    await expect(page.getByText('HorneroDB').first()).toBeVisible();
    
    // Check for welcome text
    await expect(page.getByText('Bienvenido')).toBeVisible();
    
    // Check for login button
    await expect(page.getByRole('button', { name: /Iniciar sesión con PocketID/i })).toBeVisible();
  });

  test('login button is clickable', async ({ page }) => {
    await page.goto('/');
    
    // Check that the login button exists and is enabled
    const loginButton = page.getByRole('button', { name: /Iniciar sesión con PocketID/i });
    await expect(loginButton).toBeVisible();
    await expect(loginButton).toBeEnabled();
  });

  test('health endpoint responds', async ({ request }) => {
    const response = await request.get('http://localhost:8080/health');
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data.status).toBe('ok');
  });

  test('login page has correct layout', async ({ page }) => {
    await page.goto('/');
    
    // Check login container exists
    const loginContainer = page.locator('.login-container');
    await expect(loginContainer).toBeVisible();
    
    // Check left panel (logo area)
    const loginLeft = page.locator('.login-left');
    await expect(loginLeft).toBeVisible();
    
    // Check right panel (form area)
    const loginRight = page.locator('.login-right');
    await expect(loginRight).toBeVisible();
  });
});

test.describe('API Endpoints', () => {
  test('health check', async ({ request }) => {
    const response = await request.get('http://localhost:8080/health');
    expect(response.ok()).toBeTruthy();
  });

  test('workspaces endpoint requires auth', async ({ request }) => {
    const response = await request.get('http://localhost:8080/api/v1/workspaces');
    // Should return 401 without auth
    expect(response.status()).toBe(401);
  });
});
