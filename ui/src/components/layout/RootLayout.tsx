import { Outlet, useRouterState } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Sidebar } from './Sidebar'
import { ChatWidget } from '@/components/chat/ChatWidget'
import { useUIStore } from '@/lib/ui-store'
import { fetchNavigation } from '@/lib/api/system'
import { Button } from '@/components/ui/button'
import { Menu, X } from 'lucide-react'
import { ToastContainer } from '@/components/ui/Toast'
import { useEffect } from 'react'
import { Sheet, SheetContent } from '@/components/ui/sheet'

export function RootLayout() {
  const { sidebarOpen, toggleSidebar, setSidebarOpen, setActiveModule, setShellMode } = useUIStore()
  const routerState = useRouterState()
  const pathname = routerState.location.pathname

  const { data: navigation } = useQuery({
    queryKey: ['navigation'],
    queryFn: fetchNavigation,
    staleTime: 5 * 60_000,
  })

  useEffect(() => {
    if (!pathname.startsWith('/workspace')) return

    if (pathname.startsWith('/workspace/admin')) {
      setShellMode('advanced')
      setActiveModule(null)
      return
    }

    setShellMode('simple')

    const [, , doctype] = pathname.split('/')
    if (!doctype || !navigation?.modules) {
      setActiveModule(null)
      return
    }

    const decodedDoctype = decodeURIComponent(doctype)
    const module = navigation.modules.find((group) =>
      group.doctypes.some((item) => item.name === decodedDoctype),
    )
    setActiveModule(module?.module ?? null)
  }, [navigation?.modules, pathname, setActiveModule, setShellMode])

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <div className="hidden md:flex md:relative md:z-auto">
        <Sidebar />
      </div>

      <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
        <SheetContent side="left" className="w-[86vw] max-w-sm p-0 md:hidden">
          <Sidebar mobile />
        </SheetContent>
      </Sheet>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        {/* Mobile header bar */}
        <div className="flex h-12 items-center gap-3 border-b px-4 md:hidden">
          <Button variant="ghost" size="icon" onClick={toggleSidebar}>
            {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </Button>
          <span className="font-semibold text-sm">Kora</span>
        </div>
        <Outlet />
      </main>

      {/* AI Chat Widget — floating button + panel */}
      <ChatWidget />

      {/* Toast notifications */}
      <ToastContainer />
    </div>
  )
}
