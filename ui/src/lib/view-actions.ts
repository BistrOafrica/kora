import { executeViewAction } from '@/lib/api/views'
import { router } from '@/router'
import { useCartStore } from './cart-store'

export interface ActionContext {
  viewName: string
  doctype?: string
  name?: string
  data: Record<string, any>
}

/**
 * Executes a view action. Server-side actions (create_transaction,
 * workflow_transition, create_record, update_record) are dispatched
 * to the server. Local actions (local_cart_add, navigate, local_state_set)
 * are handled client-side.
 */
export async function handleAction(
  actionId: string,
  viewName: string,
  componentId: string,
  actionType: string,
  actionConfig: Record<string, any> | undefined,
  context: ActionContext,
): Promise<any> {
  // Server-side actions: send to view action endpoint.
  // The server resolves the action type from stored config.
  if (
    actionType === 'create_record' ||
    actionType === 'update_record' ||
    actionType === 'delete_record' ||
    actionType === 'workflow_transition' ||
    actionType === 'create_transaction' ||
    actionType === 'call_script' ||
    actionType === 'call_webhook'
  ) {
    return executeViewAction(actionId, viewName, componentId, context.data)
  }

  // Local actions: handle client-side.
  switch (actionType) {
    case 'local_cart_add':
      useCartStore.getState().addItem(context.data)
      return { success: true }

    case 'local_cart_remove':
      useCartStore.getState().removeItem(context.data?.name)
      return { success: true }

    case 'navigate':
      if (actionConfig?.to) {
        router.navigate({ to: resolvePath(actionConfig.to, context) })
      }
      return { success: true }

    case 'local_state_set':
      // Handled by the component via the onAction callback.
      return { success: true }

    default:
      console.warn(`Unknown action type: ${actionType}`)
      return { success: false, error: `Unknown action type: ${actionType}` }
  }
}

function resolvePath(to: string, context: ActionContext): string {
  return to
    .replace(/\{doctype\}/g, context.doctype || '')
    .replace(/\{name\}/g, context.name || '')
}
