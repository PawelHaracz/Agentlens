import { test, expect } from '@playwright/test'
import { loginViaUI, loginViaAPI, authHeader, BASE, createUser } from './helpers'

test.describe('My projects', () => {
  test('new user sees empty My projects card', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    await createUser(request, token, {
      username: 'e2e_myproj_alice',
      password: 'Alice@E2e99!',
      display_name: 'E2E Alice',
      role_id: viewer.id,
    })

    await loginViaUI(page, 'e2e_myproj_alice', 'Alice@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /my account/i }).click()
    await expect(page.getByRole('heading', { name: /my projects/i })).toBeVisible()
    await expect(page.getByText(/don't belong to any projects yet/i)).toBeVisible()

    // Cleanup
    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const u = users.find(x => x.username === 'e2e_myproj_alice')
    if (u) await request.delete(`${BASE}/api/v1/users/${u.id}`, { headers: authHeader(token) }).catch(() => {})
  })

  test('user sees a project after admin assigns their Person as member', async ({ page, request }) => {
    const token = await loginViaAPI(request)

    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    const createdUser = await createUser(request, token, {
      username: 'e2e_myproj_bob',
      password: 'Bob@E2e99!',
      display_name: 'E2E Bob',
      role_id: viewer.id,
    })

    // Find Bob's auto-created Person party (matched by display_name used as party name).
    const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, { headers: authHeader(token) })
    const parties = await partiesRes.json() as Array<{ id: string; name: string }>
    // The person party name is set to display_name at creation time.
    const bobPerson = parties.find(p => p.name === createdUser.display_name || p.name === 'E2E Bob')!
    expect(bobPerson, `Person party for Bob not found; parties: ${JSON.stringify(parties)}`).toBeTruthy()

    // Create a project and add Bob as project:developer.
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-myproj-project' },
    })
    const project = await projectRes.json()
    await request.post(`${BASE}/api/v1/projects/${project.id}/members`, {
      headers: authHeader(token),
      data: { party_id: bobPerson.id, role: 'project:developer' },
    })

    await loginViaUI(page, 'e2e_myproj_bob', 'Bob@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /my account/i }).click()
    await expect(page.getByText('e2e-myproj-project')).toBeVisible()
    await expect(page.getByText('project:developer')).toBeVisible()

    // Row click drills to project detail.
    await page.getByText('e2e-myproj-project').click()
    await expect(page.getByRole('heading', { name: 'e2e-myproj-project' })).toBeVisible()

    // Cleanup
    await request.delete(`${BASE}/api/v1/projects/${project.id}`, { headers: authHeader(token) }).catch(() => {})
    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const bob = users.find(x => x.username === 'e2e_myproj_bob')
    if (bob) await request.delete(`${BASE}/api/v1/users/${bob.id}`, { headers: authHeader(token) }).catch(() => {})
  })
})
