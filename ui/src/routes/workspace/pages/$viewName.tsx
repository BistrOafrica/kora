import { ViewRenderer } from '@/components/views/ViewRenderer'

/**
 * Dynamic view route: /workspace/pages/$viewName
 * Renders any view registered in the Kora view system.
 */
export default function PageView() {
  return <ViewRenderer />
}
