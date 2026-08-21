export interface UploadedFile {
  path: string
  filename: string
  key: string
  mime_type: string
  size: number
  checksum?: string
}

export async function uploadFile(file: File, fieldtype: string, accept?: string): Promise<UploadedFile> {
  const formData = new FormData()
  formData.append('file', file)
  if (fieldtype) formData.append('fieldtype', fieldtype)
  if (accept) formData.append('accept', accept)

  // Use fetch directly since the api client always sends JSON.
  const url = new URL('/api/v1/upload', window.location.origin)
  const csrfMatch = document.cookie.match(/(?:^|;\s*)kora_csrf=([^;]*)/)
  const csrf = csrfMatch ? decodeURIComponent(csrfMatch[1]) : ''

  const response = await fetch(url.toString(), {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrf ? { 'X-Kora-CSRF-Token': csrf } : {},
    body: formData,
  })

  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: { message: 'Upload failed' } }))
    const message = typeof err.error === 'string' ? err.error : err.error?.message || 'Upload failed'
    throw new Error(message)
  }

  const json = await response.json()
  return json.data as UploadedFile
}

// Builds a same-origin URL for serving a stored attachment. The site is resolved
// via the kora_site cookie (path-based) or Host header (host-based) on the request.
export function fileUrl(path: string): string {
  const encoded = path.split('/').map((segment) => encodeURIComponent(segment)).join('/')
  return `/api/files/${encoded}`
}
