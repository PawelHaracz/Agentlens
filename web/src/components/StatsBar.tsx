import type { Stats } from '../types'

export default function StatsBar({ stats }: { stats: Stats }) {
  const healthy = stats.by_status['healthy'] ?? 0
  const degraded = stats.by_status['degraded'] ?? 0
  const down = stats.by_status['down'] ?? 0
  const unknown = stats.by_status['unknown'] ?? 0

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
      <Stat label="Total" value={stats.total} color="text-gray-900" />
      <Stat label="Healthy" value={healthy} color="text-green-600" />
      <Stat label="Degraded" value={degraded} color="text-yellow-600" />
      <Stat label="Down" value={down + unknown} color="text-red-600" />
    </div>
  )
}

function Stat({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
      <p className={`text-2xl font-bold mt-1 ${color}`}>{value}</p>
    </div>
  )
}
