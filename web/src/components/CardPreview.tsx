import type { ValidationPreview } from '../types'
import { Badge } from '@/components/ui/badge'

interface CardPreviewProps {
  preview: ValidationPreview
}

export default function CardPreview({ preview }: CardPreviewProps) {
  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-lg font-semibold">{preview.display_name}</h3>
        {preview.description && (
          <p className="text-sm text-muted-foreground mt-1">{String(preview.description)}</p>
        )}
      </div>

      <div className="flex flex-wrap gap-2">
        <Badge variant="secondary">{preview.protocol.toUpperCase()}</Badge>
        {preview.spec_version && (
          <Badge variant="outline">v{String(preview.spec_version)}</Badge>
        )}
      </div>
    </div>
  )
}
