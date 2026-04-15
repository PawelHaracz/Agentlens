import { test, expect } from '@playwright/test'
import { loginViaUI, loginViaAPI, authHeader, BASE, createUser } from './helpers'

test.describe('Groups management', () => {
  test.afterEach(async ({ request }) => {
    // Best-effort cleanup: delete any groups whose name starts with "e2e-"
    const token = await loginViaAPI(request)
    const listRes = await request.get(`${BASE}/api/v1/groups`, { headers: authHeader(token) })
    if (listRes.ok()) {
      const groups: Array<{ id: string; name: string }> = await listRes.json()
      for (const g of groups.filter(g => g.name.startsWith('e2e-'))) {
        await request.delete(`${BASE}/api/v1/groups/${g.id}`, { headers: authHeader(token) }).catch(() => {})
      }
    }
  })

  test('admin creates, views, deletes a group', async ({ page }) => {
    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /groups/i }).click()
    await page.getByRole('button', { name: /create group/i }).click()
    await page.getByLabel(/name/i).fill('e2e-demo-group')
    await page.getByRole('button', { name: /^create$/i }).click()
    await expect(page.getByText('e2e-demo-group')).toBeVisible()

    // Drill in
    await page.getByText('e2e-demo-group').click()
    await expect(page.getByRole('heading', { name: 'e2e-demo-group' })).toBeVisible()
    await expect(page.getByText(/no members yet/i)).toBeVisible()

    // Back + delete
    await page.getByRole('link', { name: /back to settings/i }).click()
    page.once('dialog', d => d.accept())
    await page.getByTitle('Delete').first().click()
    await expect(page.getByText('e2e-demo-group')).not.toBeVisible()
  })

  test('admin adds a person member', async ({ page, request }) => {
    // Seed a person via API — create a user which auto-creates a Person party.
    const token = await loginViaAPI(request)
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    await createUser(request, token, {
      username: 'e2e_alice', password: 'Alice@E2e99!', display_name: 'E2E Alice', role_id: viewer.id,
    })

    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /groups/i }).click()
    await page.getByRole('button', { name: /create group/i }).click()
    await page.getByLabel(/name/i).fill('e2e-member-group')
    await page.getByRole('button', { name: /^create$/i }).click()

    await page.getByText('e2e-member-group').click()
    await page.getByRole('button', { name: /add member/i }).click()
    await page.getByRole('combobox').selectOption({ label: /E2E Alice/ })
    await page.getByRole('button', { name: /^add$/i }).click()
    await expect(page.getByText(/E2E Alice/)).toBeVisible()

    // Cleanup user
    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const alice = users.find(u => u.username === 'e2e_alice')
    if (alice) await request.delete(`${BASE}/api/v1/users/${alice.id}`, { headers: authHeader(token) }).catch(() => {})
  })

  test('admin nests a group inside another group', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const mkGroup = async (name: string) => {
      const r = await request.post(`${BASE}/api/v1/groups`, {
        headers: authHeader(token),
        data: { name },
      })
      return (await r.json()).id as string
    }
    await mkGroup('e2e-parent')
    await mkGroup('e2e-child')

    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /groups/i }).click()
    await page.getByText('e2e-parent').click()
    await page.getByRole('button', { name: /add member/i }).click()
    await page.getByRole('combobox').selectOption({ label: /e2e-child/ })
    await page.getByRole('button', { name: /^add$/i }).click()
    await expect(page.getByText('e2e-child')).toBeVisible()
    await expect(page.getByText(/^Group$/)).toBeVisible()
  })

  test('cycle attempt shows inline error', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const mkGroup = async (name: string) => {
      const r = await request.post(`${BASE}/api/v1/groups`, {
        headers: authHeader(token),
        data: { name },
      })
      return (await r.json()).id as string
    }
    const parentId = await mkGroup('e2e-cycle-parent')
    const childId = await mkGroup('e2e-cycle-child')
    // Add child as a member of parent via API (direct)
    await request.post(`${BASE}/api/v1/groups/${parentId}/members`, {
      headers: authHeader(token),
      data: { party_id: childId, role: 'member' },
    })

    await loginViaUI(page)
    await page.goto(`/settings/groups/${childId}`)
    await page.getByRole('button', { name: /add member/i }).click()
    // Try to add parent as a member of child → cycle
    await page.getByRole('combobox').selectOption({ label: /e2e-cycle-parent/ })
    await page.getByRole('button', { name: /^add$/i }).click()
    await expect(page.getByText(/already in the group's ancestry/i)).toBeVisible()
  })

  test('viewer sees list but no mutate buttons', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    await request.post(`${BASE}/api/v1/groups`, {
      headers: authHeader(token),
      data: { name: 'e2e-read-only' },
    })

    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    await createUser(request, token, {
      username: 'e2e_viewer', password: 'Viewer@E2e99!', display_name: 'Viewer', role_id: viewer.id,
    })

    await loginViaUI(page, 'e2e_viewer', 'Viewer@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /groups/i }).click()
    await expect(page.getByText('e2e-read-only')).toBeVisible()
    await expect(page.getByRole('button', { name: /create group/i })).not.toBeVisible()

    // Cleanup user
    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const vu = users.find(u => u.username === 'e2e_viewer')
    if (vu) await request.delete(`${BASE}/api/v1/users/${vu.id}`, { headers: authHeader(token) }).catch(() => {})
  })
})
