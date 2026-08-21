import { useMemo } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { fetchDoctypeSchema } from '@/lib/api/system'
import { Breadcrumbs } from '@/components/layout/Breadcrumbs'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Plus } from 'lucide-react'
import type { DocType } from '@/types/kora'
import { ManifestRenderer } from '@/manifest/runtime/ManifestRenderer'
import { createStandardPageManifest } from '@/manifest/runtime/standard-pages'
import { validatePageManifestContract } from '@/manifest/schema/page'

export default function ListPage() {
  const { doctype } = useParams({ from: '/workspace/$doctype' })
  const navigate = useNavigate()

  const schemaQuery = useQuery({
    queryKey: ['doctype', doctype],
    queryFn: () => fetchDoctypeSchema(doctype),
    staleTime: 5 * 60_000,
  })

  const dt: DocType | undefined = schemaQuery.data?.doctype
  const manifest = useMemo(() => (dt ? createStandardPageManifest(dt, 'overview') : null), [dt])
  const issues = useMemo(() => (manifest ? validatePageManifestContract(manifest) : []), [manifest])

  if (schemaQuery.isLoading || !manifest) {
    return (
      <div className="p-8 space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (schemaQuery.isError || !dt) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-muted-foreground">DocType "{doctype}" not found.</p>
      </div>
    )
  }

  return (
    <div className="p-4 md:p-8 space-y-6">
      <div>
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
          Overview / {dt.name}
        </p>
        <Breadcrumbs items={[{ label: dt.module }, { label: dt.name }]} className="mb-2" />

        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">{dt.name}</h1>
            <p className="text-sm text-muted-foreground">
              Manifest-driven workspace with list and insights sections.
            </p>
          </div>
          <Button
            onClick={() => navigate({ to: '/workspace/$doctype/new', params: { doctype } })}
          >
            <Plus className="mr-2 h-4 w-4" />
            New {dt.name}
          </Button>
        </div>
      </div>

      {issues.length > 0 ? (
        <div className="rounded-xl border border-dashed bg-card p-6">
          <h2 className="text-lg font-semibold">Manifest needs attention</h2>
          <ul className="mt-4 space-y-2 text-sm">
            {issues.map((issue) => (
              <li key={`${issue.path}:${issue.message}`} className="rounded-lg bg-muted/40 p-2">
                <span className="font-mono text-xs text-destructive">{issue.path}</span>
                <span className="block text-muted-foreground">{issue.message}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <ManifestRenderer manifest={manifest} mode="runtime" />
      )}
    </div>
  )
}
