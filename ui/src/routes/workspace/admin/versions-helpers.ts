export function getVersionConfirmTitle(type: 'activate' | 'discard' | 'rollback' | 'activateAll' | null): string {
  switch (type) {
    case 'activate': return 'Activate Version'
    case 'discard': return 'Discard Draft'
    case 'rollback': return 'Rollback Version'
    case 'activateAll': return 'Activate All Drafts'
    default: return ''
  }
}

export function getVersionConfirmDescription(
  type: 'activate' | 'discard' | 'rollback' | 'activateAll' | null,
  dialogError: string | null,
): string {
  if (dialogError) return dialogError

  switch (type) {
    case 'activate':
      return 'This will promote the selected draft to the live configuration.'
    case 'discard':
      return 'This draft will be marked as Superseded and removed from the pending queue.'
    case 'rollback':
      return 'This will replace the current config with the selected historical version.'
    case 'activateAll':
      return 'This will activate the latest draft and fold in all earlier draft changes.'
    default:
      return ''
  }
}

export function getVersionConfirmLabel(type: 'activate' | 'discard' | 'rollback' | 'activateAll' | null): string {
  switch (type) {
    case 'discard': return 'Discard'
    case 'rollback': return 'Rollback'
    default: return 'Activate'
  }
}
