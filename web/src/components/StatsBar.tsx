import type { Stats } from '../types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function StatsBar({ stats }: { stats: Stats }) {
  const healthy = stats.by_status['healthy'] ?? 0
  const degraded = stats.by_status['degraded'] ?? 0
  const down = stats.by_status['down'] ?? 0
  const unknown = stats.by_status['unknown'] ?? 0

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
      <StatCard label="Total" value={stats.total} className="text-foreground" />
      <StatCard label="Healthy" value={healthy} className="text-green-600" />
      <StatCard label="Degraded" value={degraded} className="text-yellow-600" />
      <StatCard label="Down" value={down + unknown} className="text-destructive" />
    </div>
  )
}

function StatCard({ label, value, className }: { label: string; value: number; className: string }) {
  return (
    <Card>
      <CardHeader className="pb-2 pt-4 px-4">
        <CardTitle className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent className="px-4 pb-4 pt-0">
        <p className={`text-2xl font-bold ${className}`}>{value}</p>
      </CardContent>
    </Card>
  )
}
