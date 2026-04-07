import type { Stats } from '../types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function StatsBar({ stats }: { stats: Stats }) {
  const active = stats.by_status['active'] ?? 0
  const degraded = stats.by_status['degraded'] ?? 0
  const offline = stats.by_status['offline'] ?? 0
  const registered = stats.by_status['registered'] ?? 0

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
      <StatCard label="Total" value={stats.total} className="text-foreground" />
      <StatCard label="Active" value={active} className="text-green-600" />
      <StatCard label="Degraded" value={degraded} className="text-yellow-600" />
      <StatCard label="Offline" value={offline + registered} className="text-destructive" />
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
