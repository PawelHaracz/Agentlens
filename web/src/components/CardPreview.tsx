import type { ValidationPreview, TypedMeta } from '../types'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

interface CardPreviewProps {
  preview: ValidationPreview
  typedMeta?: TypedMeta[]
}

export default function CardPreview({ preview, typedMeta }: CardPreviewProps) {
  const extensions = typedMeta?.filter(m => m.kind === 'a2a.extension') ?? []
  const securitySchemes = typedMeta?.filter(m => m.kind === 'a2a.security_scheme') ?? []

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-lg font-semibold">{preview.display_name}</h3>
        {preview.description && (
          <p className="text-sm text-muted-foreground mt-1">{preview.description}</p>
        )}
      </div>

      <div className="flex flex-wrap gap-2">
        <Badge variant="secondary">{preview.protocol.toUpperCase()}</Badge>
        {preview.spec_version && (
          <Badge variant="outline">v{preview.spec_version}</Badge>
        )}
        <Badge variant="secondary">{preview.skills_count} skill{preview.skills_count !== 1 ? 's' : ''}</Badge>
      </div>

      {preview.interfaces.length > 0 && (
        <>
          <Separator />
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Interfaces</p>
            <div className="space-y-1">
              {preview.interfaces.map((url, i) => (
                <p key={i} className="text-sm font-mono break-all">{url}</p>
              ))}
            </div>
          </div>
        </>
      )}

      {extensions.length > 0 && (
        <>
          <Separator />
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">
              Extensions ({extensions.length})
            </p>
            <div className="space-y-1">
              {extensions.map((ext, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  <span className="font-mono break-all">{(ext as unknown as { uri: string }).uri}</span>
                  <Badge variant={(ext as unknown as { required: boolean }).required ? 'destructive' : 'secondary'} className="text-xs">
                    {(ext as unknown as { required: boolean }).required ? 'Required' : 'Optional'}
                  </Badge>
                </div>
              ))}
            </div>
          </div>
        </>
      )}

      {(securitySchemes.length > 0 || preview.security_schemes.length > 0) && (
        <>
          <Separator />
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Security</p>
            <div className="flex flex-wrap gap-1">
              {preview.security_schemes.map((scheme, i) => (
                <Badge key={i} variant="outline">{scheme}</Badge>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
