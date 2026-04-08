import { Boxes, Bot, Plug } from 'lucide-react'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { Protocol } from '../../../types'

interface Props {
  value: Protocol | undefined
  onChange: (protocol: Protocol | undefined) => void
}

const OPTIONS = [
  { value: undefined, label: 'All', icon: Boxes },
  { value: 'a2a' as Protocol, label: 'A2A', icon: Bot },
  { value: 'mcp' as Protocol, label: 'MCP', icon: Plug },
]

export function ProtocolFilter({ value, onChange }: Props) {
  return (
    <ToggleGroup
      type="single"
      value={value ?? 'all'}
      onValueChange={v => onChange(v === 'all' ? undefined : (v as Protocol))}
      aria-label="Filter by protocol"
    >
      {OPTIONS.map(opt => (
        <ToggleGroupItem
          key={opt.value ?? 'all'}
          value={opt.value ?? 'all'}
          aria-label={opt.label}
          className="flex items-center gap-1.5"
        >
          <opt.icon className="h-3.5 w-3.5" />
          {opt.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
