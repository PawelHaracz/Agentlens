import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { CheckCircle, XCircle } from 'lucide-react'
import * as api from '@/api'

export default function PendingIdentitiesPage() {
  const qc = useQueryClient()

  const { data: identities = [], isLoading } = useQuery({
    queryKey: ['pending-identities'],
    queryFn: api.listPendingIdentities,
  })

  const approveMut = useMutation({
    mutationFn: (id: string) => api.approveIdentity(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pending-identities'] }),
  })

  const rejectMut = useMutation({
    mutationFn: (id: string) => api.rejectIdentity(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pending-identities'] }),
  })

  return (
    <div className="space-y-4 p-6">
      <div>
        <h1 className="text-2xl font-semibold">Pending Identities</h1>
        <p className="text-sm text-muted-foreground">Federated users awaiting operator approval</p>
      </div>

      <div className="rounded-md border">
        <Table data-testid="pending-identities-table">
          <TableHeader>
            <TableRow>
              <TableHead>Provider</TableHead>
              <TableHead>Subject</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow><TableCell colSpan={5} className="text-center">Loading…</TableCell></TableRow>
            )}
            {!isLoading && identities.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No pending identities
                </TableCell>
              </TableRow>
            )}
            {identities.map(identity => (
              <TableRow key={identity.id} data-testid={`identity-row-${identity.id}`}>
                <TableCell>{identity.provider_name}</TableCell>
                <TableCell className="font-mono text-xs">{identity.sub}</TableCell>
                <TableCell>{identity.email || '—'}</TableCell>
                <TableCell><Badge variant="outline">{identity.status}</Badge></TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-1">
                    <Button variant="ghost" size="icon" title="Approve"
                      data-testid={`approve-btn-${identity.id}`}
                      onClick={() => approveMut.mutate(identity.id)}>
                      <CheckCircle className="h-4 w-4 text-green-600" />
                    </Button>
                    <Button variant="ghost" size="icon" title="Reject"
                      data-testid={`reject-btn-${identity.id}`}
                      onClick={() => rejectMut.mutate(identity.id)}>
                      <XCircle className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
