import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

interface KindFilterProps {
  value: string
  onChange: (value: string) => void
}

const kinds = [
  { value: 'all', label: 'All' },
  { value: 'a2a.skill', label: 'A2A Skill' },
  { value: 'mcp.tool', label: 'MCP Tool' },
  { value: 'mcp.resource', label: 'MCP Resource' },
  { value: 'mcp.prompt', label: 'MCP Prompt' },
]

export function KindFilter({ value, onChange }: KindFilterProps) {
  return (
    <ToggleGroup
      type="single"
      value={value || 'all'}
      onValueChange={(val) => onChange(val === 'all' ? '' : val || '')}
      aria-label="Filter by capability kind"
      className="justify-start"
    >
      {kinds.map((kind) => (
        <ToggleGroupItem key={kind.value} value={kind.value}>
          {kind.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
