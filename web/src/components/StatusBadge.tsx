import type { Status } from '../types'

const styles: Record<Status, string> = {
  healthy: 'bg-green-100 text-green-800',
  degraded: 'bg-yellow-100 text-yellow-800',
  down: 'bg-red-100 text-red-800',
  unknown: 'bg-gray-100 text-gray-600',
}

export default function StatusBadge({ status }: { status: Status }) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${styles[status]}`}>
      {status}
    </span>
  )
}
