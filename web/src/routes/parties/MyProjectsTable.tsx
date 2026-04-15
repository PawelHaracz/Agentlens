import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import * as api from '@/api'
import type { UserProjectMembership } from '@/api'

export default function MyProjectsTable() {
  const navigate = useNavigate()
  const [memberships, setMemberships] = useState<UserProjectMembership[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const load = () => {
    setLoading(true)
    setError(false)
    api.getMyProjects()
      .then(m => { setMemberships(m); setLoading(false) })
      .catch(() => { setError(true); setLoading(false) })
  }

  useEffect(load, [])

  if (loading) return <div className="text-sm text-muted-foreground">Loading…</div>
  if (error) {
    return (
      <div className="space-y-2">
        <p className="text-sm text-destructive">Failed to load projects.</p>
        <Button size="sm" variant="outline" onClick={load}>Retry</Button>
      </div>
    )
  }
  if (memberships.length === 0) {
    return <p className="text-sm text-muted-foreground">You don't belong to any projects yet.</p>
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Role</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {memberships.map(m => (
            <TableRow
              key={m.project.id}
              className="cursor-pointer hover:bg-muted/50"
              onClick={() => navigate(`/settings/projects/${m.project.id}`)}
            >
              <TableCell className="font-medium">{m.project.name}</TableCell>
              <TableCell><Badge variant="outline">{m.role}</Badge></TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
