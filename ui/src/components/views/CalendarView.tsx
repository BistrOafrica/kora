import { useState, useMemo } from 'react'
import type { ViewComponentProps } from './registry'
import { Button } from '@/components/ui/button'
import { Loader2, ChevronLeft, ChevronRight } from 'lucide-react'
import { format, startOfMonth, endOfMonth, startOfWeek, endOfWeek, addDays, isSameMonth, isSameDay, parseISO } from 'date-fns'

/**
 * CalendarView renders a month grid with events from any doctype.
 * Events are positioned based on the configured date field binding.
 */
export default function CalendarView(props: ViewComponentProps) {
  const { data, isLoading, config, onAction } = props
  const [currentDate, setCurrentDate] = useState(new Date())

  const dateField = config.bindings?.date || 'date'
  const titleField = config.bindings?.title || 'name'

  const events = useMemo(() => {
    const records: any[] = data?.data || []
    const eventMap: Record<string, any[]> = {}
    for (const r of records) {
      const d = r[dateField]
      if (!d) continue
      try {
        const key = format(new Date(d), 'yyyy-MM-dd')
        if (!eventMap[key]) eventMap[key] = []
        eventMap[key].push(r)
      } catch {}
    }
    return eventMap
  }, [data, dateField])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const monthStart = startOfMonth(currentDate)
  const monthEnd = endOfMonth(monthStart)
  const startDate = startOfWeek(monthStart)
  const endDate = endOfWeek(monthEnd)
  const days: Date[] = []
  let day = startDate
  while (day <= endDate) { days.push(day); day = addDays(day, 1) }

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between px-4 py-3 border-b bg-muted/20">
        <Button variant="ghost" size="icon" onClick={() => setCurrentDate(d => addDays(startOfMonth(d), -1))}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <h3 className="text-sm font-semibold">{format(currentDate, 'MMMM yyyy')}</h3>
        <Button variant="ghost" size="icon" onClick={() => setCurrentDate(d => addDays(endOfMonth(d), 1))}>
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
      <div className="grid grid-cols-7 text-xs">
        {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map(d => (
          <div key={d} className="px-2 py-1 text-center font-medium text-muted-foreground border-b">{d}</div>
        ))}
        {days.map((d) => {
          const key = format(d, 'yyyy-MM-dd')
          const dayEvents = events[key] || []
          const isToday = isSameDay(d, new Date())
          const isCurrent = isSameMonth(d, currentDate)
          return (
            <div
              key={key}
              className={`min-h-[80px] border-b border-r p-1 ${!isCurrent && 'bg-muted/20 text-muted-foreground'}`}
            >
              <div className={`text-xs mb-1 ${isToday ? 'bg-primary text-primary-foreground rounded-full w-5 h-5 flex items-center justify-center' : ''}`}>
                {format(d, 'd')}
              </div>
              {dayEvents.slice(0, 3).map((ev: any) => (
                <div
                  key={ev.name}
                  className="text-xs truncate rounded bg-primary/10 px-1 py-0.5 mb-0.5 cursor-pointer hover:bg-primary/20"
                  onClick={() => onAction('select', { name: ev.name, row: ev })}
                >
                  {ev[titleField] || ev.name}
                </div>
              ))}
              {dayEvents.length > 3 && (
                <div className="text-xs text-muted-foreground">+{dayEvents.length - 3} more</div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
