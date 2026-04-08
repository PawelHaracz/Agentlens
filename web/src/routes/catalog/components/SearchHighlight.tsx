interface Props {
  snippet?: string
}

export function SearchHighlight({ snippet }: Props) {
  if (!snippet) return null
  return (
    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-1 italic">
      …{snippet}…
    </p>
  )
}
