import { test, expect } from '@playwright/test'
import { loginViaUI, loginViaAPI, authHeader, BASE, createUser } from './helpers'

test.describe('Projects management', () => {
  test.afterEach(async ({ request }) => {
    const token = await loginViaAPI(request)
    const listRes = await request.get(`${BASE}/api/v1/projects`, { headers: authHeader(token) })
    if (listRes.ok()) {
      const projects: Array<{ id: string; name: string }> = await listRes.json()
      for (const p of projects.filter(p => p.name.startsWith('e2e-'))) {
        await request.delete(`${BASE}/api/v1/projects/${p.id}`, { headers: authHeader(token) }).catch(() => {})
      }
    }
  })

  test('admin creates, views, deletes a project', async ({ page }) => {
    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^projects$/i }).click()
    await page.getByRole('button', { name: /create project/i }).click()
    await page.getByLabel(/name/i).fill('e2e-demo-project')
    await page.getByRole('button', { name: /^create$/i }).click()
    await expect(page.getByText('e2e-demo-project')).toBeVisible()

    // Drill in
    await page.getByText('e2e-demo-project').click()
    await expect(page.getByRole('heading', { name: 'e2e-demo-project' })).toBeVisible()
    await expect(page.getByRole('heading', { name: /assigned catalog entries/i })).toBeVisible()

    // Back + delete
    await page.getByRole('link', { name: /back to settings/i }).click()
    await page.getByRole('tab', { name: /^projects$/i }).click()
    page.once('dialog', d => d.accept())
    const row = page.getByRole('row', { name: /e2e-demo-project/i })
    await row.getByTitle('Delete').click()
    await expect(page.getByText('e2e-demo-project')).not.toBeVisible()
  })

  test('admin adds a person member with role project:developer', async ({ page }) => {
    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^projects$/i }).click()
    await page.getByRole('button', { name: /create project/i }).click()
    await page.getByLabel(/name/i).fill('e2e-member-project')
    await page.getByRole('button', { name: /^create$/i }).click()

    await page.getByText('e2e-member-project').click()
    await page.getByRole('button', { name: /add member/i }).click()
    await page.getByLabel('Member', { exact: true }).selectOption({ label: 'Administrator (Person)' })
    await page.getByLabel('Role', { exact: true }).selectOption('project:developer')
    await page.getByRole('button', { name: /^add$/i }).click()
    await expect(page.getByRole('cell', { name: 'Administrator' })).toBeVisible()
    await expect(page.getByText('project:developer')).toBeVisible()
  })

  test('admin changes a member role via edit dialog', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-role-project' },
    })
    const project = await projectRes.json()

    // Find admin's Person party.
    const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, { headers: authHeader(token) })
    const persons = await partiesRes.json() as Array<{ id: string; name: string }>
    const adminPerson = persons[0]
    await request.post(`${BASE}/api/v1/projects/${project.id}/members`, {
      headers: authHeader(token),
      data: { party_id: adminPerson.id, role: 'project:viewer' },
    })

    await loginViaUI(page)
    await page.goto(`/settings/projects/${project.id}`)
    // Click the role badge button to open the edit dialog.
    await page.locator('button', { hasText: 'project:viewer' }).click()
    await expect(page.getByRole('heading', { name: /edit member access/i })).toBeVisible()
    await page.getByLabel('Role').selectOption('project:owner')
    await page.getByRole('button', { name: /^save$/i }).click()
    await expect(page.getByText('project:owner')).toBeVisible()
  })

  test('admin assigns a catalog entry to a project', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-assign-project' },
    })
    const project = await projectRes.json()
    const entryRes = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'E2E Assigned Agent',
        description: 'assign test',
        protocol: 'a2a',
        endpoint: `http://e2e-assign-${Date.now()}.example.com`,
        version: '1.0.0',
      },
    })
    const entry = await entryRes.json()

    await loginViaUI(page)
    await page.goto(`/settings/projects/${project.id}`)
    await page.getByRole('button', { name: /assign entry/i }).click()
    await page.getByPlaceholder(/search catalog/i).fill('E2E Assigned')
    await page.getByText('E2E Assigned Agent').click()
    await page.getByRole('button', { name: /^assign$/i }).click()
    await expect(page.getByText('E2E Assigned Agent')).toBeVisible()

    await request.delete(`${BASE}/api/v1/catalog/${entry.id}`, { headers: authHeader(token) }).catch(() => {})
  })

  test('viewer sees list but no mutate buttons', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-read-only-project' },
    })
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    await createUser(request, token, {
      username: 'e2e_proj_viewer',
      password: 'Viewer@E2e99!',
      display_name: 'Project Viewer',
      role_id: viewer.id,
    })

    await loginViaUI(page, 'e2e_proj_viewer', 'Viewer@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^projects$/i }).click()
    await expect(page.getByText('e2e-read-only-project')).toBeVisible()
    await expect(page.getByRole('button', { name: /create project/i })).not.toBeVisible()

    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const vu = users.find(u => u.username === 'e2e_proj_viewer')
    if (vu) await request.delete(`${BASE}/api/v1/users/${vu.id}`, { headers: authHeader(token) }).catch(() => {})
  })
})
