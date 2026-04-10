import { Link } from 'react-router-dom'
import ProtocolBadge from '@/components/ProtocolBadge'
import StatusBadge from '@/components/StatusBadge'
import type { CapabilityAgentDTO, Protocol, LifecycleState } from '@/types'

interface AgentForCapabilityRowProps {
  agent: CapabilityAgentDTO
}

export function AgentForCapabilityRow({ agent }: AgentForCapabilityRowProps) {
  const snippetDescription =
    (agent.capability_snippet?.description as string) || ''
  const truncated =
    snippetDescription.length > 100
      ? snippetDescription.slice(0, 100) + '...'
      : snippetDescription

  return (
    <tr className="hover:bg-muted/50">
      <td className="p-3">
        <ProtocolBadge protocol={agent.protocol as Protocol} />
      </td>
      <td className="p-3">
        <Link to={`/catalog/${agent.id}`} className="font-medium hover:underline">
          {agent.display_name}
        </Link>
      </td>
      <td className="p-3 text-sm text-muted-foreground">
        {agent.provider?.organization || '-'}
      </td>
      <td className="p-3">
        <StatusBadge status={agent.status as LifecycleState} />
      </td>
      <td className="p-3 text-sm text-muted-foreground">
        {agent.spec_version || '-'}
      </td>
      <td className="p-3 text-sm text-muted-foreground max-w-xs" title={snippetDescription}>
        {truncated}
      </td>
      <td className="p-3 text-sm text-muted-foreground">
        {agent.health.latencyMs}ms
      </td>
    </tr>
  )
}
