import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { getAgent, deleteAgent } from '../api'
import type { Agent } from '../types'
import StatusBadge from './StatusBadge'
import ProtocolBadge from './ProtocolBadge'

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getAgent(id)
      .then(setAgent)
      .catch(e => setError(e instanceof Error ? e.message : 'Unknown error'))
      .finally(() => setLoading(false))
  }, [id])

  const handleDelete = async () => {
    if (!agent || !confirm(`Delete agent "${agent.name}"?`)) return
    setDeleting(true)
    try {
      await deleteAgent(agent.id)
      navigate('/')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
      setDeleting(false)
    }
  }

  if (loading) return <div className="text-center py-12 text-gray-400 text-sm">Loading…</div>
  if (error) return <div className="bg-red-50 border border-red-200 rounded-md p-4 text-red-700 text-sm">{error}</div>
  if (!agent) return null

  return (
    <div>
      <div className="mb-4">
        <Link to="/" className="text-sm text-indigo-600 hover:text-indigo-800">← Back to agents</Link>
      </div>

      <div className="bg-white rounded-lg border border-gray-200 p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">{agent.name}</h1>
            {agent.description && <p className="text-gray-500 mt-1">{agent.description}</p>}
          </div>
          <button
            onClick={handleDelete}
            disabled={deleting}
            className="shrink-0 px-3 py-1.5 text-sm text-red-600 border border-red-200 rounded hover:bg-red-50 disabled:opacity-50"
          >
            {deleting ? 'Deleting…' : 'Delete'}
          </button>
        </div>

        <div className="flex flex-wrap gap-2 mt-4">
          <ProtocolBadge protocol={agent.protocol} />
          <StatusBadge status={agent.status} />
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-gray-100 text-gray-600">{agent.source}</span>
          {agent.version && (
            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-gray-100 text-gray-600">v{agent.version}</span>
          )}
        </div>

        <dl className="mt-6 grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
          <Field label="Endpoint" value={agent.endpoint} mono />
          {agent.namespace && <Field label="Namespace" value={agent.namespace} />}
          {agent.team && <Field label="Team" value={agent.team} />}
          <Field label="Last Seen" value={new Date(agent.last_seen).toLocaleString()} />
          <Field label="Created" value={new Date(agent.created_at).toLocaleString()} />
        </dl>

        {agent.tags && agent.tags.length > 0 && (
          <div className="mt-4">
            <p className="text-xs text-gray-500 uppercase tracking-wide mb-2">Tags</p>
            <div className="flex flex-wrap gap-1">
              {agent.tags.map(tag => (
                <span key={tag} className="px-2 py-0.5 rounded bg-indigo-50 text-indigo-700 text-xs">{tag}</span>
              ))}
            </div>
          </div>
        )}

        {agent.skills && agent.skills.length > 0 && (
          <div className="mt-6">
            <p className="text-xs text-gray-500 uppercase tracking-wide mb-3">Skills ({agent.skills.length})</p>
            <div className="space-y-2">
              {agent.skills.map((skill, i) => (
                <div key={i} className="border border-gray-100 rounded p-3 bg-gray-50">
                  <p className="font-medium text-sm text-gray-800">{skill.name}</p>
                  {skill.description && <p className="text-xs text-gray-500 mt-0.5">{skill.description}</p>}
                  <div className="flex gap-4 mt-2 text-xs text-gray-400">
                    {skill.input_modes && skill.input_modes.length > 0 && (
                      <span>In: {skill.input_modes.join(', ')}</span>
                    )}
                    {skill.output_modes && skill.output_modes.length > 0 && (
                      <span>Out: {skill.output_modes.join(', ')}</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-xs text-gray-500 uppercase tracking-wide">{label}</dt>
      <dd className={`mt-0.5 text-gray-900 break-all ${mono ? 'font-mono text-xs' : ''}`}>{value}</dd>
    </div>
  )
}
