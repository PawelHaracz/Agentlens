import { useState, useEffect, type FormEvent } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useTheme } from '../contexts/ThemeContext'
import * as api from '@/api'
import type { User, Role } from '@/api'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { Sun, Moon, Monitor, Plus, Pencil, Trash2, Lock, Unlock, Shield } from 'lucide-react'
import GroupsTab from '../routes/parties/GroupsTab'

const ALL_PERMISSIONS = [
  'catalog:read', 'catalog:write', 'catalog:delete',
  'users:read', 'users:write', 'users:delete',
  'roles:read', 'roles:write',
  'settings:read', 'settings:write',
]

/* ─── General Tab ─── */
function GeneralTab() {
  const { theme, setTheme } = useTheme()
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    api.getSettings()
      .then(list => {
        const map: Record<string, string> = {}
        for (const s of list) map[s.key] = s.value
        setSettings(map)
        if (map['ui.theme']) {
          setTheme(map['ui.theme'] as 'light' | 'dark' | 'system')
        }
      })
      .catch(() => { /* settings may not exist yet */ })
  }, [setTheme])

  const handleSave = async () => {
    setSaving(true)
    try {
      await api.updateSettings(settings)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch { /* ignore */ }
    setSaving(false)
  }

  const update = (key: string, value: string) => {
    setSettings(prev => ({ ...prev, [key]: value }))
    if (key === 'ui.theme') setTheme(value as 'light' | 'dark' | 'system')
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Appearance</CardTitle>
          <CardDescription>Customize the look and feel of the application.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Theme</label>
            <div className="flex gap-2">
              {([
                { value: 'light', label: 'Light', icon: Sun },
                { value: 'dark', label: 'Dark', icon: Moon },
                { value: 'system', label: 'System', icon: Monitor },
              ] as const).map(({ value, label, icon: Icon }) => (
                <Button
                  key={value}
                  variant={theme === value ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => update('ui.theme', value)}
                  className="gap-2"
                >
                  <Icon className="h-4 w-4" />
                  {label}
                </Button>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Display</CardTitle>
          <CardDescription>Configure display and polling settings.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-2">
              <label className="text-sm font-medium">Items per page</label>
              <Input
                type="number"
                min={5}
                max={100}
                value={settings['ui.items_per_page'] || '25'}
                onChange={e => update('ui.items_per_page', e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Poll interval (seconds)</label>
              <Input
                type="number"
                min={5}
                max={3600}
                value={settings['ui.poll_interval'] || '30'}
                onChange={e => update('ui.poll_interval', e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Health check interval (seconds)</label>
              <Input
                type="number"
                min={10}
                max={3600}
                value={settings['health.check_interval'] || '60'}
                onChange={e => update('health.check_interval', e.target.value)}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? 'Saving…' : 'Save settings'}
        </Button>
        {saved && <span className="text-sm text-muted-foreground">Settings saved.</span>}
      </div>
    </div>
  )
}

/* ─── Users Tab ─── */
function UsersTab() {
  const { hasPermission } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editUser, setEditUser] = useState<User | null>(null)
  const [form, setForm] = useState({ username: '', email: '', display_name: '', password: '', role_id: '' })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const canWrite = hasPermission('users:write')
  const canDelete = hasPermission('users:delete')

  const load = () => {
    api.listUsers().then(setUsers).catch(() => {})
    api.listRoles().then(setRoles).catch(() => {})
  }

  useEffect(load, [])

  const openAdd = () => {
    setEditUser(null)
    setForm({ username: '', email: '', display_name: '', password: '', role_id: roles[0]?.id ?? '' })
    setError('')
    setDialogOpen(true)
  }

  const openEdit = (u: User) => {
    setEditUser(u)
    setForm({ username: u.username, email: u.email || '', display_name: u.display_name || '', password: '', role_id: u.role_id })
    setError('')
    setDialogOpen(true)
  }

  const handleSave = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      if (editUser) {
        await api.updateUser(editUser.id, {
          username: form.username,
          email: form.email,
          display_name: form.display_name,
          role_id: form.role_id,
        })
      } else {
        await api.createUser({
          username: form.username,
          email: form.email || undefined,
          display_name: form.display_name || undefined,
          password: form.password,
          role_id: form.role_id,
        })
      }
      setDialogOpen(false)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed')
    }
    setSaving(false)
  }

  const handleDelete = async (u: User) => {
    if (!confirm(`Delete user "${u.username}"?`)) return
    try {
      await api.deleteUser(u.id)
      load()
    } catch { /* ignore */ }
  }

  const handleToggleActive = async (u: User) => {
    try {
      await api.updateUser(u.id, { is_active: !u.is_active })
      load()
    } catch { /* ignore */ }
  }

  const roleName = (id: string) => roles.find(r => r.id === id)?.name ?? id

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Users</h3>
        {canWrite && (
          <Button size="sm" onClick={openAdd} className="gap-1">
            <Plus className="h-4 w-4" /> Add user
          </Button>
        )}
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              {(canWrite || canDelete) && <TableHead className="w-24">Actions</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map(u => (
              <TableRow key={u.id}>
                <TableCell className="font-medium">{u.username}</TableCell>
                <TableCell>{u.email}</TableCell>
                <TableCell><Badge variant="secondary">{roleName(u.role_id)}</Badge></TableCell>
                <TableCell>
                  <Badge variant={u.is_active ? 'default' : 'destructive'}>
                    {u.is_active ? 'Active' : 'Locked'}
                  </Badge>
                </TableCell>
                {(canWrite || canDelete) && (
                  <TableCell>
                    <div className="flex gap-1">
                      {canWrite && (
                        <>
                          <Button variant="ghost" size="icon" onClick={() => openEdit(u)} title="Edit">
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => handleToggleActive(u)} title={u.is_active ? 'Lock' : 'Unlock'}>
                            {u.is_active ? <Lock className="h-3.5 w-3.5" /> : <Unlock className="h-3.5 w-3.5" />}
                          </Button>
                        </>
                      )}
                      {canDelete && (
                        <Button variant="ghost" size="icon" onClick={() => handleDelete(u)} title="Delete">
                          <Trash2 className="h-3.5 w-3.5 text-destructive" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {users.length === 0 && (
              <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground py-8">No users found.</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editUser ? 'Edit user' : 'Add user'}</DialogTitle>
            <DialogDescription>{editUser ? 'Update user details.' : 'Create a new user account.'}</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSave} className="space-y-4">
            {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
            <div className="space-y-2">
              <label className="text-sm font-medium">Username</label>
              <Input value={form.username} onChange={e => setForm(f => ({ ...f, username: e.target.value }))} required />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Email</label>
              <Input type="email" value={form.email} onChange={e => setForm(f => ({ ...f, email: e.target.value }))} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Display name</label>
              <Input value={form.display_name} onChange={e => setForm(f => ({ ...f, display_name: e.target.value }))} />
            </div>
            {!editUser && (
              <div className="space-y-2">
                <label className="text-sm font-medium">Password</label>
                <Input type="password" value={form.password} onChange={e => setForm(f => ({ ...f, password: e.target.value }))} required />
              </div>
            )}
            <div className="space-y-2">
              <label className="text-sm font-medium">Role</label>
              <Select value={form.role_id} onValueChange={v => setForm(f => ({ ...f, role_id: v }))}>
                <SelectTrigger><SelectValue placeholder="Select role" /></SelectTrigger>
                <SelectContent>
                  {roles.map(r => <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <DialogFooter>
              <Button type="submit" disabled={saving}>{saving ? 'Saving…' : editUser ? 'Update' : 'Create'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

/* ─── Roles Tab ─── */
function RolesTab() {
  const { hasPermission } = useAuth()
  const [roles, setRoles] = useState<Role[]>([])
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editRole, setEditRole] = useState<Role | null>(null)
  const [form, setForm] = useState({ name: '', description: '', permissions: [] as string[] })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const canWrite = hasPermission('roles:write')

  const load = () => { api.listRoles().then(setRoles).catch(() => {}) }
  useEffect(load, [])

  const openAdd = () => {
    setEditRole(null)
    setForm({ name: '', description: '', permissions: [] })
    setError('')
    setDialogOpen(true)
  }

  const openEdit = (r: Role) => {
    setEditRole(r)
    setForm({ name: r.name, description: r.description || '', permissions: [...r.permissions] })
    setError('')
    setDialogOpen(true)
  }

  const togglePerm = (perm: string) => {
    setForm(f => ({
      ...f,
      permissions: f.permissions.includes(perm)
        ? f.permissions.filter(p => p !== perm)
        : [...f.permissions, perm],
    }))
  }

  const handleSave = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      if (editRole) {
        await api.updateRole(editRole.id, { name: form.name, description: form.description, permissions: form.permissions })
      } else {
        await api.createRole({ name: form.name, description: form.description || undefined, permissions: form.permissions })
      }
      setDialogOpen(false)
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed')
    }
    setSaving(false)
  }

  const handleDelete = async (r: Role) => {
    if (!confirm(`Delete role "${r.name}"?`)) return
    try {
      await api.deleteRole(r.id)
      load()
    } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Roles</h3>
        {canWrite && (
          <Button size="sm" onClick={openAdd} className="gap-1">
            <Plus className="h-4 w-4" /> Add role
          </Button>
        )}
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Description</TableHead>
              <TableHead>Permissions</TableHead>
              {canWrite && <TableHead className="w-24">Actions</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {roles.map(r => (
              <TableRow key={r.id}>
                <TableCell className="font-medium">
                  <div className="flex items-center gap-2">
                    {r.name}
                    {r.is_system && <span title="System role"><Shield className="h-3.5 w-3.5 text-muted-foreground" /></span>}
                  </div>
                </TableCell>
                <TableCell>{r.description}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {r.permissions.map(p => <Badge key={p} variant="outline" className="text-xs">{p}</Badge>)}
                  </div>
                </TableCell>
                {canWrite && (
                  <TableCell>
                    {!r.is_system && (
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => openEdit(r)} title="Edit">
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => handleDelete(r)} title="Delete">
                          <Trash2 className="h-3.5 w-3.5 text-destructive" />
                        </Button>
                      </div>
                    )}
                  </TableCell>
                )}
              </TableRow>
            ))}
            {roles.length === 0 && (
              <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground py-8">No roles found.</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editRole ? 'Edit role' : 'Add role'}</DialogTitle>
            <DialogDescription>{editRole ? 'Update role settings.' : 'Create a new role.'}</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSave} className="space-y-4">
            {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
            <div className="space-y-2">
              <label className="text-sm font-medium">Name</label>
              <Input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} required />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Description</label>
              <Input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Permissions</label>
              <div className="grid grid-cols-2 gap-2">
                {ALL_PERMISSIONS.map(perm => (
                  <label key={perm} className="flex items-center gap-2 text-sm cursor-pointer">
                    <input
                      type="checkbox"
                      checked={form.permissions.includes(perm)}
                      onChange={() => togglePerm(perm)}
                      className="rounded border-input"
                    />
                    {perm}
                  </label>
                ))}
              </div>
            </div>
            <DialogFooter>
              <Button type="submit" disabled={saving}>{saving ? 'Saving…' : editRole ? 'Update' : 'Create'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

/* ─── My Account Tab ─── */
function AccountTab() {
  const { user, refreshUser } = useAuth()
  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileMsg, setProfileMsg] = useState('')

  const [currentPw, setCurrentPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [pwSaving, setPwSaving] = useState(false)
  const [pwMsg, setPwMsg] = useState('')
  const [pwError, setPwError] = useState('')

  useEffect(() => {
    setDisplayName(user?.display_name ?? '')
    setEmail(user?.email ?? '')
  }, [user])

  const handleProfileSave = async (e: FormEvent) => {
    e.preventDefault()
    if (!user) return
    setProfileSaving(true)
    setProfileMsg('')
    try {
      await api.updateUser(user.id, { display_name: displayName, email })
      await refreshUser()
      setProfileMsg('Profile updated.')
      setTimeout(() => setProfileMsg(''), 3000)
    } catch (err) {
      setProfileMsg(err instanceof Error ? err.message : 'Failed')
    }
    setProfileSaving(false)
  }

  const handlePasswordChange = async (e: FormEvent) => {
    e.preventDefault()
    setPwError('')
    setPwMsg('')
    if (newPw !== confirmPw) { setPwError('Passwords do not match.'); return }
    if (newPw.length < 8) { setPwError('Password must be at least 8 characters.'); return }
    setPwSaving(true)
    try {
      await api.changePassword(currentPw, newPw)
      setCurrentPw('')
      setNewPw('')
      setConfirmPw('')
      setPwMsg('Password changed.')
      setTimeout(() => setPwMsg(''), 3000)
    } catch (err) {
      setPwError(err instanceof Error ? err.message : 'Failed')
    }
    setPwSaving(false)
  }

  if (!user) return null

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
          <CardDescription>Update your personal information.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleProfileSave} className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Username</label>
              <Input value={user.username} disabled />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Display name</label>
              <Input value={displayName} onChange={e => setDisplayName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Email</label>
              <Input type="email" value={email} onChange={e => setEmail(e.target.value)} />
            </div>
            {user.last_login && (
              <p className="text-sm text-muted-foreground">
                Last login: {new Date(user.last_login).toLocaleString()}
              </p>
            )}
            <div className="flex items-center gap-3">
              <Button type="submit" disabled={profileSaving}>{profileSaving ? 'Saving…' : 'Update profile'}</Button>
              {profileMsg && <span className="text-sm text-muted-foreground">{profileMsg}</span>}
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Change password</CardTitle>
          <CardDescription>Update your password.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handlePasswordChange} className="space-y-4 max-w-sm">
            {pwError && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{pwError}</div>}
            <div className="space-y-2">
              <label className="text-sm font-medium">Current password</label>
              <Input type="password" value={currentPw} onChange={e => setCurrentPw(e.target.value)} autoComplete="current-password" required />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">New password</label>
              <Input type="password" value={newPw} onChange={e => setNewPw(e.target.value)} autoComplete="new-password" required />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Confirm new password</label>
              <Input type="password" value={confirmPw} onChange={e => setConfirmPw(e.target.value)} autoComplete="new-password" required />
            </div>
            <div className="flex items-center gap-3">
              <Button type="submit" disabled={pwSaving}>{pwSaving ? 'Changing…' : 'Change password'}</Button>
              {pwMsg && <span className="text-sm text-muted-foreground">{pwMsg}</span>}
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

/* ─── Settings Page ─── */
export default function SettingsPage() {
  const { hasPermission } = useAuth()
  const showUsers = hasPermission('users:read')
  const showRoles = hasPermission('roles:read')

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Settings</h2>
        <p className="text-muted-foreground">Manage your application preferences and administration.</p>
      </div>
      <Separator />
      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">General</TabsTrigger>
          {showUsers && <TabsTrigger value="users">Users</TabsTrigger>}
          {showRoles && <TabsTrigger value="roles">Roles</TabsTrigger>}
          <TabsTrigger value="groups">Groups</TabsTrigger>
          <TabsTrigger value="account">My Account</TabsTrigger>
        </TabsList>
        <TabsContent value="general" className="mt-6"><GeneralTab /></TabsContent>
        {showUsers && <TabsContent value="users" className="mt-6"><UsersTab /></TabsContent>}
        {showRoles && <TabsContent value="roles" className="mt-6"><RolesTab /></TabsContent>}
        <TabsContent value="groups" className="mt-6"><GroupsTab /></TabsContent>
        <TabsContent value="account" className="mt-6"><AccountTab /></TabsContent>
      </Tabs>
    </div>
  )
}
