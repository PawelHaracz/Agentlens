import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import * as api from '@/api'
import type { Party } from '@/api'

interface Props {
  entryId: string
}

export function EntryProjectsSection({ entryId }: Props) {
  const [projects, setProjects] = useState<Party[]>([])

  useEffect(() => {
    api.listEntryProjects(entryId).then(setProjects).catch(() => setProjects([]))
  }, [entryId])

  if (projects.length === 0) return null

  return (
    <div>
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">Projects</p>
      <div className="flex flex-wrap gap-2">
        {projects.map(p => (
          <Link key={p.id} to={`/settings/projects/${p.id}`}>
            <Badge variant="outline" className="cursor-pointer hover:bg-muted">{p.name}</Badge>
          </Link>
        ))}
      </div>
    </div>
  )
}
