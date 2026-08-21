import type { Field } from '@/types/kora'

const IMAGE_EXTS = new Set(['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp', '.svg'])
const AUDIO_EXTS = new Set(['.mp3', '.wav', '.ogg', '.oga', '.m4a', '.aac', '.flac', '.opus', '.webm'])

function extFromPath(path: string): string {
  const name = path.split('/').pop() || path
  const i = name.lastIndexOf('.')
  return i >= 0 ? name.slice(i).toLowerCase() : ''
}

/**
 * Validates an attachment client-side before upload. When `accept` is provided it
 * takes precedence over the field-type defaults. Returns a human-readable error
 * string, or null when the file is allowed.
 */
export function validateAttachmentFile(
  file: File,
  fieldtype: Field['fieldtype'],
  accept?: string,
): string | null {
  const ext = extFromPath(file.name)
  const mime = (file.type || '').toLowerCase()

  if (accept) {
    return validateAgainstAccept(accept, ext, mime)
  }

  if (fieldtype === 'Attach Image') {
    if (mime.startsWith('image/')) return null
    if (IMAGE_EXTS.has(ext)) return null
    return 'Please choose an image file.'
  }

  if (fieldtype === 'Attach Audio') {
    if (mime.startsWith('audio/')) return null
    if (AUDIO_EXTS.has(ext)) return null
    return 'Please choose an audio file.'
  }

  return null
}

function validateAgainstAccept(accept: string, ext: string, mime: string): string | null {
  const entries = accept
    .replace(/\n/g, ',')
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean)

  for (const entry of entries) {
    if (entry.startsWith('.')) {
      if (ext === entry.toLowerCase()) return null
      continue
    }
    if (entry.includes('/')) {
      // MIME match: "image/*" matches any image; "application/pdf" is exact.
      if (entry === mime || (entry.endsWith('/*') && mime.startsWith(entry.slice(0, -1)))) {
        return null
      }
    }
  }

  return `Please choose a file in one of these formats: ${entries.join(', ')}.`
}
