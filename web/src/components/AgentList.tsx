import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { listAgents, getStats } from '../api'
import type { Agent, Stats, Protocol, Status } from '../types'
import StatusBadge from './StatusBadge'
import ProtocolBadge from './ProtocolBadge'
import StatsBar from './StatsBar'
import SearchBar from './SearchBar'

export default function AgentList() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [stats, setStats] = useState<Stats | null>(null)
  const [search, setSearch] = useState('')
  const [protocol, setProtocol] = useState<Protocol | ''>('')
  const [status, setStatus] = useState<Status | ''>('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [a, s] = await Promise.all([
        listAgents({
          q: search || undefined,
          protocol: protocol || undefined,
          status: status || undefined,
        }),
        getStats(),
      ])
      setAgents(a)
      setStats(s)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [search, protocol, status])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div>
      {stats && <StatsBar stats={stats} />}

      <div className="flex flex-col sm:flex-row gap-2 mb-4">
        <div className="flex-1">
          <SearchBar value={search} onChange={setSearch} />
        </div>
        <select
          className="border border-gray-300 rounded-md px-3 py-2 text-sm bg-white focus:outline-none focus:ring-1 focus:ring-indigo-500"
          value={protocol}
          onChange={e => setProtocol(e.target.value as Protocol | '')}
        >
          <option value="">All protocols</option>
          <option value="a2a">A2A</option>
          <option value="mcp">MCP</option>
          <option value="a2ui">A2UI</option>
        </select>
        <select
          className="border border-gray-300 rounded-md px-3 py-2 text-sm bg-white focus:outline-none focus:ring-1 focus:ring-indigo-500"
          value={status}
          onChange={e => setStatus(e.target.value as Status | '')}
        >
          <option value="">All statuses</option>
          <option value="healthy">Healthy</option>
          <option value="degraded">Degraded</option>
          <option value="down">Down</option>
          <option value="unknown">Unknown</option>
        </select>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-4 text-red-700 text-sm">{error}</div>
      )}

      {loading ? (
        <div className="text-center py-12 text-gray-400 text-sm">Loading…</div>
      ) : agents.length === 0 ? (
        <div className="text-center py-12 text-gray-400 text-sm">No agents found.</div>
      ) : (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Protocol</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider hidden sm:table-cell">Source</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider hidden md:table-cell">Endpoint</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {agents.map(agent => (
                <AgentRow key={agent.id} agent={agent} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function AgentRow({ agent }: { agent: Agent }) {
  return (
    <tr className="hover:bg-gray-50 transition-colors">
      <td className="px-4 py-3">
        <Link to={`/agents/${agent.id}`} className="font-medium text-indigo-600 hover:text-indigo-800">
          {agent.name}
        </Link>
        {agent.description && (
          <p className="text-xs text-gray-500 mt-0.5 truncate max-w-xs">{agent.description}</p>
        )}
      </td>
      <td className="px-4 py-3">
        <ProtocolBadge protocol={agent.protocol} />
      </td>
      <td className="px-4 py-3">
        <StatusBadge status={agent.status} />
      </td>
      <td className="px-4 py-3 text-sm text-gray-500 hidden sm:table-cell">{agent.source}</td>
      <td className="px-4 py-3 text-sm text-gray-400 hidden md:table-cell font-mono truncate max-w-xs">{agent.endpoint}</td>
    </tr>
  )
}
