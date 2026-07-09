import { useEffect, useMemo, useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { fetchDoctypeSchema } from '@/lib/api/system'
import { fetchList } from '@/lib/api/resources'
import { DataTable } from '@/components/tables/DataTable'
import { InsightsPanel } from '@/components/analytics/InsightsPanel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Breadcrumbs } from '@/components/layout/Breadcrumbs'
import { Filter, Plus, Search, List, BarChart3, X } from 'lucide-react'
import type { Document, DocType, Field } from '@/types/kora'
import { cn } from '@/lib/utils'

export default function ListPage() {
  const { doctype } = useParams({ from: '/workspace/$doctype' })
  const navigate = useNavigate()

  const [page, setPage] = useState(0)
  const [sorting, setSorting] = useState<{ field: string; order: string } | null>(null)
  const [activeTab, setActiveTab] = useState<string>('list')
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [filterValues, setFilterValues] = useState<Record<string, string>>({})
  const limit = 50

  const schemaQuery = useQuery({
    queryKey: ['doctype', doctype],
    queryFn: () => fetchDoctypeSchema(doctype),
    staleTime: 5 * 60_000,
  })

  const dt: DocType | undefined = schemaQuery.data?.doctype
  const perms = schemaQuery.data?.permissions
  const canCreate = perms?.create ?? false
  const canWrite = perms?.write ?? false

  const filterFields = useMemo(
    () => dt?.fields?.filter((f) => f.in_standard_filter && !isLayoutField(f.fieldtype) && !f.hidden) ?? [],
    [dt],
  )
  const listFields = dt?.fields?.filter((f) => f.in_list_view && !isLayoutField(f.fieldtype)) ?? []
  const activeFilters = useMemo(
    () => buildFilters(dt, debouncedSearch, filterValues, filterFields),
    [dt, debouncedSearch, filterValues, filterFields],
  )
  const filtersJson = useMemo(() => JSON.stringify(activeFilters), [activeFilters])
  const hasActiveFilters = debouncedSearch.trim().length > 0 || Object.values(filterValues).some((value) => value.trim() !== '')

  const listQuery = useQuery({
    queryKey: ['resource', doctype, page, sorting, filtersJson],
    queryFn: () =>
      fetchList(doctype, {
        limit,
        offset: page * limit,
        order_by: sorting ? `${sorting.field} ${sorting.order}` : undefined,
        filters: activeFilters.length > 0 ? filtersJson : undefined,
      }),
    staleTime: 15_000,
    placeholderData: (prev) => prev,
  })

  const total = listQuery.data?.meta?.total ?? 0
  const totalPages = Math.ceil(total / limit)

  useEffect(() => {
    const handle = window.setTimeout(() => setDebouncedSearch(search), 250)
    return () => window.clearTimeout(handle)
  }, [search])

  useEffect(() => {
    setPage(0)
  }, [doctype, debouncedSearch, filtersJson, sorting?.field, sorting?.order, activeTab])

  if (schemaQuery.isLoading || listQuery.isLoading) {
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
    <div className="p-4 md:p-8">
      <Breadcrumbs items={[{ label: dt.module }, { label: dt.name }]} className="mb-2" />

      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{dt.name}</h1>
          <p className="text-sm text-muted-foreground">
            {total} record{total !== 1 ? 's' : ''}
            {!canWrite && <span className="ml-2 text-amber-600 dark:text-amber-400">(read-only)</span>}
          </p>
        </div>
        <Button
          onClick={() => navigate({ to: '/workspace/$doctype/new', params: { doctype } })}
          disabled={!canCreate}
          title={!canCreate ? "You don't have permission to create" : undefined}
        >
          <Plus className="mr-2 h-4 w-4" />
          New {dt.name}
        </Button>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="mb-6">
          <TabsTrigger value="list" className="flex items-center gap-2">
            <List className="h-4 w-4" />
            List
          </TabsTrigger>
          <TabsTrigger value="insights" className="flex items-center gap-2">
            <BarChart3 className="h-4 w-4" />
            Insights
          </TabsTrigger>
        </TabsList>

        <TabsContent value="list">
          <div className="mb-4 rounded-xl border bg-card p-4 shadow-sm">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div className="grid flex-1 gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
                <div className="space-y-1.5">
                  <Label className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                    <Search className="h-3.5 w-3.5" />
                    Search
                  </Label>
                  <div className="relative">
                    <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      placeholder={`Search ${dt.name.toLowerCase()}...`}
                      className="pl-9 pr-9"
                    />
                    {search.trim() && (
                      <button
                        type="button"
                        className="absolute right-2 top-2 rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
                        onClick={() => setSearch('')}
                        aria-label="Clear search"
                      >
                        <X className="h-4 w-4" />
                      </button>
                    )}
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                    <Filter className="h-3.5 w-3.5" />
                    Quick filters
                  </Label>
                  <div className="flex flex-wrap gap-2">
                    {filterFields.length === 0 ? (
                      <span className="text-sm text-muted-foreground">No standard filters configured.</span>
                    ) : (
                      filterFields.slice(0, 4).map((field) => (
                        <FilterControl
                          key={field.fieldname}
                          field={field}
                          value={filterValues[field.fieldname] ?? ''}
                          onChange={(value) =>
                            setFilterValues((current) => ({
                              ...current,
                              [field.fieldname]: value,
                            }))
                          }
                        />
                      ))
                    )}
                  </div>
                </div>
              </div>

              {hasActiveFilters && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setSearch('')
                    setFilterValues({})
                  }}
                  className="shrink-0"
                >
                  Clear filters
                </Button>
              )}
            </div>
          </div>

          <DataTable
            columns={listFields}
            data={(listQuery.data?.data as Document[]) ?? []}
            titleField={dt.title_field}
            total={total}
            page={page}
            totalPages={totalPages}
            sorting={sorting}
            onSortingChange={setSorting}
            onPageChange={setPage}
            onRowClick={(doc) =>
              navigate({
                to: '/workspace/$doctype/$name',
                params: { doctype, name: doc.name },
              })
            }
            isEmpty={!listQuery.isFetching && total === 0}
            hasFilters={hasActiveFilters}
            isFetching={listQuery.isFetching}
            isError={listQuery.isError}
            onRetry={() => listQuery.refetch()}
            emptyTitle={`No ${dt.name.toLowerCase()} yet`}
            emptyDescription={`Create the first ${dt.name.toLowerCase()} to start working.`}
            filteredEmptyTitle="No matching records"
            filteredEmptyDescription="Try clearing a filter or broadening the search."
          />
        </TabsContent>

        <TabsContent value="insights">
          <InsightsPanel doctype={doctype} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function isLayoutField(fieldtype: string): boolean {
  return ['Section Break', 'Column Break', 'Heading', 'Table'].includes(fieldtype)
}

function buildFilters(
  dt: DocType | undefined,
  search: string,
  filterValues: Record<string, string>,
  filterFields: Field[],
): [string, string, string | number | boolean][] {
  const filters: [string, string, string | number | boolean][] = []
  const term = search.trim()

  if (term && dt?.title_field) {
    filters.push([dt.title_field, 'like', `%${term}%`])
  }

  for (const field of filterFields) {
    const raw = filterValues[field.fieldname]?.trim()
    if (!raw) continue

    switch (field.fieldtype) {
      case 'Check':
        if (raw === 'true' || raw === 'false') {
          filters.push([field.fieldname, '=', raw === 'true'])
        }
        break
      case 'Int':
      case 'Float':
      case 'Currency':
      case 'Percent': {
        const num = Number(raw)
        if (Number.isFinite(num)) filters.push([field.fieldname, '=', num])
        break
      }
      case 'Date':
      case 'Datetime':
      case 'Select':
      case 'Link':
      case 'Dynamic Link':
        filters.push([field.fieldname, '=', raw])
        break
      default:
        filters.push([field.fieldname, 'like', `%${raw}%`])
        break
    }
  }

  return filters
}

function FilterControl({
  field,
  value,
  onChange,
}: {
  field: Field
  value: string
  onChange: (value: string) => void
}) {
  const label = field.label || field.fieldname

  if (field.fieldtype === 'Check') {
    return (
      <div className="min-w-[140px] space-y-1">
        <Label className="text-[11px] font-medium text-muted-foreground">{label}</Label>
        <Select value={value || '__all__'} onValueChange={(next) => onChange(next === '__all__' ? '' : String(next))}>
          <SelectTrigger className="h-9">
            <SelectValue placeholder="Any" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Any</SelectItem>
            <SelectItem value="true">Yes</SelectItem>
            <SelectItem value="false">No</SelectItem>
          </SelectContent>
        </Select>
      </div>
    )
  }

  if (field.fieldtype === 'Select') {
    const options = field.fieldtype === 'Select'
      ? field.options.split('\n').map((opt) => opt.trim()).filter(Boolean)
      : []

    return (
      <div className="min-w-[180px] space-y-1">
        <Label className="text-[11px] font-medium text-muted-foreground">{label}</Label>
        <Select value={value || '__all__'} onValueChange={(next) => onChange(next === '__all__' ? '' : String(next))}>
          <SelectTrigger className="h-9">
            <SelectValue placeholder="Any" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Any</SelectItem>
            {options.length > 0 ? (
              options.map((opt) => (
                <SelectItem key={opt} value={opt}>
                  {opt}
                </SelectItem>
              ))
            ) : (
              <SelectItem value="__empty__" disabled>
                No predefined options
              </SelectItem>
            )}
          </SelectContent>
        </Select>
      </div>
    )
  }

  if (field.fieldtype === 'Link' || field.fieldtype === 'Dynamic Link') {
    return (
      <div className="min-w-[180px] space-y-1">
        <Label className="text-[11px] font-medium text-muted-foreground">{label}</Label>
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={`Filter by ${label.toLowerCase()}`}
          className="h-9"
        />
      </div>
    )
  }

  if (field.fieldtype === 'Date' || field.fieldtype === 'Datetime') {
    return (
      <div className="min-w-[180px] space-y-1">
        <Label className="text-[11px] font-medium text-muted-foreground">{label}</Label>
        <Input
          type="date"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="h-9"
        />
      </div>
    )
  }

  if (field.fieldtype === 'Int' || field.fieldtype === 'Float' || field.fieldtype === 'Currency' || field.fieldtype === 'Percent') {
    return (
      <div className="min-w-[160px] space-y-1">
        <Label className="text-[11px] font-medium text-muted-foreground">{label}</Label>
        <Input
          type="number"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="h-9"
        />
      </div>
    )
  }

  return (
    <div className={cn('min-w-[180px] space-y-1', field.fieldtype === 'Text Editor' && 'min-w-[220px]')}>
      <Label className="text-[11px] font-medium text-muted-foreground">{label}</Label>
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={`Filter by ${label.toLowerCase()}`}
        className="h-9"
      />
    </div>
  )
}
