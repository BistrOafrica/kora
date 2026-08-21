export type PageManifestSection = {
  id: string
  kind: 'hero' | 'search' | 'metrics' | 'list' | 'tabs' | 'empty'
  title?: string
  description?: string
}

export type PageManifest = {
  name: string
  route: string
  label: string
  module: string
  status: ManifestLifecycleStatus
  version?: string
  runtime?: string
  capabilities?: string[]
  permissions?: string[]
  offline?: 'unsupported' | 'read_only' | 'queue_writes' | 'full_slice'
  sections: PageManifestSection[]
}

export type PageManifestError = { message: string }
export type ManifestLifecycleStatus = 'draft' | 'preview' | 'active' | 'retired'

export const MANIFEST_LIFECYCLE_TRANSITIONS: Record<ManifestLifecycleStatus, ManifestLifecycleStatus[]> = {
  draft: ['preview', 'active', 'retired'],
  preview: ['draft', 'active', 'retired'],
  active: ['retired', 'preview'],
  retired: [],
}

export const WORKSPACE_DASHBOARD_MANIFEST: PageManifest = {
  name: 'workspace-dashboard',
  route: '/workspace',
  label: 'Workspace',
  module: 'Workspace',
  status: 'active',
  sections: [
    { id: 'hero', kind: 'hero', title: 'Start with the next useful task' },
    { id: 'search', kind: 'search', title: 'Workspace search' },
    { id: 'resume', kind: 'metrics', title: 'Resume work' },
    { id: 'modules', kind: 'list', title: 'Modules' },
  ],
}

export const DOCTYPE_WORKSPACE_MANIFEST: PageManifest = {
  name: 'doctype-workspace',
  route: '/workspace/$doctype',
  label: 'Doctype workspace',
  module: 'Workspace',
  status: 'active',
  sections: [
    { id: 'header', kind: 'hero', title: 'Document workspace' },
    { id: 'actions', kind: 'metrics', title: 'Primary actions' },
    { id: 'tabs', kind: 'tabs', title: 'List and insights' },
  ],
}

export function validatePageManifest(manifest: PageManifest): PageManifestError[] {
  const errors: PageManifestError[] = []

  if (!manifest.name) errors.push({ message: 'manifest.name is required' })
  if (!manifest.route || !manifest.route.startsWith('/')) errors.push({ message: 'manifest.route must start with /' })
  if (!manifest.label) errors.push({ message: 'manifest.label is required' })
  if (!manifest.module) errors.push({ message: 'manifest.module is required' })
  if (!manifest.status) errors.push({ message: 'manifest.status is required' })
  else if (!Object.prototype.hasOwnProperty.call(MANIFEST_LIFECYCLE_TRANSITIONS, manifest.status)) {
    errors.push({ message: 'manifest.status must be draft, preview, active, or retired' })
  }
  if (!Array.isArray(manifest.sections) || manifest.sections.length === 0) {
    errors.push({ message: 'manifest.sections must contain at least one section' })
  }
  if (manifest.capabilities && !Array.isArray(manifest.capabilities)) {
    errors.push({ message: 'manifest.capabilities must be an array' })
  }
  if (manifest.permissions && !Array.isArray(manifest.permissions)) {
    errors.push({ message: 'manifest.permissions must be an array' })
  }
  if (manifest.offline && !['unsupported', 'read_only', 'queue_writes', 'full_slice'].includes(manifest.offline)) {
    errors.push({ message: 'manifest.offline must be unsupported, read_only, queue_writes, or full_slice' })
  }

  const seen = new Set<string>()
  for (const section of manifest.sections ?? []) {
    if (!section.id) errors.push({ message: 'section.id is required' })
    if (section.id && seen.has(section.id)) errors.push({ message: `duplicate section id: ${section.id}` })
    if (section.id) seen.add(section.id)
    if (!section.kind) errors.push({ message: `section ${section.id || '<unknown>'} is missing kind` })
  }

  return errors
}

export function canTransitionManifestStatus(
  from: ManifestLifecycleStatus,
  to: ManifestLifecycleStatus,
): boolean {
  return MANIFEST_LIFECYCLE_TRANSITIONS[from]?.includes(to) ?? false
}

export function transitionManifestStatus(
  manifest: PageManifest,
  nextStatus: ManifestLifecycleStatus,
): PageManifest {
  if (!canTransitionManifestStatus(manifest.status, nextStatus)) {
    throw new Error(`invalid manifest lifecycle transition: ${manifest.status} -> ${nextStatus}`)
  }
  return {
    ...manifest,
    status: nextStatus,
  }
}

export function isDraftManifest(manifest: Pick<PageManifest, 'status'>): boolean {
  return manifest.status === 'draft'
}

export function isPreviewManifest(manifest: Pick<PageManifest, 'status'>): boolean {
  return manifest.status === 'preview'
}

export function isActiveManifest(manifest: Pick<PageManifest, 'status'>): boolean {
  return manifest.status === 'active'
}

export function isRetiredManifest(manifest: Pick<PageManifest, 'status'>): boolean {
  return manifest.status === 'retired'
}

export async function computePageManifestDigest(manifest: PageManifest): Promise<string> {
  const json = serializePageManifest(manifest)
  const bytes = new TextEncoder().encode(json)
  const buffer = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength)
  const digest = await crypto.subtle.digest('SHA-256', buffer)
  return `sha256:${toBase64Url(new Uint8Array(digest))}`
}

export async function buildPageManifestETag(manifest: PageManifest): Promise<string> {
  return `"${await computePageManifestDigest(manifest)}"`
}

export async function verifyPageManifestSignature(
  manifest: PageManifest,
  signature: string | undefined,
  publicKey: JsonWebKey,
): Promise<boolean> {
  if (!signature) return false
  const encoded = new TextEncoder().encode(serializePageManifest(manifest))
  const data = Uint8Array.from(encoded)
  const sig = base64UrlToBytes(signature)
  const signatureBytes = Uint8Array.from(sig)
  const key = await crypto.subtle.importKey('jwk', publicKey, { name: 'Ed25519' }, false, ['verify'])
  return crypto.subtle.verify('Ed25519', key, signatureBytes, data)
}

export function serializePageManifest(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map((entry) => serializePageManifest(entry)).join(',')}]`
  }
  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>).sort(([a], [b]) => a.localeCompare(b))
    return `{${entries.map(([key, entry]) => `${JSON.stringify(key)}:${serializePageManifest(entry)}`).join(',')}}`
  }
  return JSON.stringify(value)
}

function toBase64Url(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function base64UrlToBytes(input: string): Uint8Array {
  const normalized = input.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}
