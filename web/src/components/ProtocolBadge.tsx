import type { Protocol } from '../types'

const styles: Record<Protocol, string> = {
  a2a: 'bg-blue-100 text-blue-800',
  mcp: 'bg-green-100 text-green-800',
  a2ui: 'bg-purple-100 text-purple-800',
}

export default function ProtocolBadge({ protocol }: { protocol: Protocol }) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold uppercase ${styles[protocol]}`}>
      {protocol}
    </span>
  )
}
