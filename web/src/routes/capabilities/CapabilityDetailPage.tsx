import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { getCapabilityAgents } from '@/api'
import { AgentForCapabilityRow } from './components/AgentForCapabilityRow'

function kindToLabel(kind: string): string {
  const map: Record<string, string> = {
    'a2a.skill': 'A2A Skill',
    'mcp.tool': 'MCP Tool',
    'mcp.resource': 'MCP Resource',
    'mcp.prompt': 'MCP Prompt',
  }
  return map[kind] || kind
}

export default function CapabilityDetailPage() {
  const { key } = useParams<{ key: string }>()

  if (!key) {
    return <div className="text-center py-12">Invalid capability key</div>
  }

  const decoded = decodeURIComponent(key)
  const [kind, name] = decoded.split('::', 2)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['capability-agents', kind, name],
    queryFn: () => getCapabilityAgents(kind, name),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">Failed to load capability details</p>
        <p className="text-sm text-muted-foreground mt-1">
          {error instanceof Error ? error.message : 'Unknown error'}
        </p>
      </div>
    )
  }

  if (!data || data.agents.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">No agents offer this capability</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/" className="hover:underline">
          Catalog
        </Link>
        <span>/</span>
        <Link to="/catalog/capabilities" className="hover:underline">
          Capabilities
        </Link>
        <span>/</span>
        <span className="text-foreground">{name}</span>
      </div>

      {/* Back button */}
      <Button variant="ghost" size="sm" asChild>
        <Link to="/catalog/capabilities">
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Capabilities
        </Link>
      </Button>

      {/* Header */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Badge variant="outline">{kindToLabel(kind)}</Badge>
        </div>
        <h1 className="text-3xl font-bold">{name}</h1>
        <p className="text-muted-foreground mt-1">
          {data.agents.length} {data.agents.length === 1 ? 'agent' : 'agents'}
        </p>
      </div>

      {/* Agents table */}
      <div className="border rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-muted/50 border-b">
            <tr>
              <th className="p-3 text-left text-sm font-medium">Protocol</th>
              <th className="p-3 text-left text-sm font-medium">Agent</th>
              <th className="p-3 text-left text-sm font-medium">Provider</th>
              <th className="p-3 text-left text-sm font-medium">Status</th>
              <th className="p-3 text-left text-sm font-medium">Version</th>
              <th className="p-3 text-left text-sm font-medium">Description</th>
              <th className="p-3 text-left text-sm font-medium">Latency</th>
            </tr>
          </thead>
          <tbody>
            {data.agents.map((agent) => (
              <AgentForCapabilityRow key={agent.id} agent={agent} />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
