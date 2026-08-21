import { useRef, useState } from 'react'
import type { Field } from '@/types/kora'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { uploadFile, fileUrl } from '@/lib/api/files'
import { cn } from '@/lib/utils'
import { Loader2, Upload, Trash2, FileText, ExternalLink } from 'lucide-react'
import { validateAttachmentFile } from './attachment-utils'

interface AttachmentFieldProps {
  field: Field
  value: any
  onChange: (fieldname: string, value: any) => void
  disabled: boolean
  error?: string
  compact?: boolean
  label: string
  id: string
  labelClass?: string
  gapClass?: string
}

// Human-readable hints for each attachment type (shown in the empty state).
const TYPE_HELP: Record<string, string> = {
  'Attach Image': 'JPG, PNG, GIF, or WebP',
  'Attach Audio': 'MP3, WAV, OGG, M4A, or FLAC',
  Attach: 'PDF, Word, Excel, or any file',
}

const TYPE_VERB: Record<string, string> = {
  'Attach Image': 'image',
  'Attach Audio': 'audio',
  Attach: 'file',
}

function filenameFromPath(path: string): string {
  return path.split('/').pop() || path
}

function extFromPath(path: string): string {
  const name = filenameFromPath(path)
  const i = name.lastIndexOf('.')
  return i >= 0 ? name.slice(i).toLowerCase() : ''
}

export function AttachmentField({
  field,
  value,
  onChange,
  disabled,
  error,
  compact,
  label,
  id,
  labelClass,
  gapClass,
}: AttachmentFieldProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  const currentPath = typeof value === 'string' ? value : ''
  const hasValue = currentPath.length > 0
  const filename = hasValue ? filenameFromPath(currentPath) : ''
  const ext = hasValue ? extFromPath(currentPath) : ''

  const isImage = field.fieldtype === 'Attach Image'
  const isAudio = field.fieldtype === 'Attach Audio'
  const isPDF = ext === '.pdf'
  const kind = TYPE_VERB[field.fieldtype] || 'file'
  const help = field.accept
    ? field.accept.replace(/\n/g, ', ')
    : TYPE_HELP[field.fieldtype] || TYPE_HELP.Attach

  const accept = field.accept
    ? field.accept.replace(/\n/g, ',').split(',').map((s) => s.trim()).filter(Boolean).join(',')
    : isImage ? 'image/*' : isAudio ? 'audio/*' : undefined

  const openPicker = () => {
    if (disabled || uploading) return
    const input = inputRef.current
    if (!input) return
    if (typeof input.showPicker === 'function') {
      input.showPicker()
      return
    }
    input.click()
  }

  const handleSelect = async (file: File) => {
    if (!file) return
    setLocalError(null)
    const validationError = validateAttachmentFile(file, field.fieldtype, field.accept)
    if (validationError) {
      setLocalError(validationError)
      if (inputRef.current) inputRef.current.value = ''
      return
    }
    setUploading(true)
    try {
      const uploaded = await uploadFile(file, field.fieldtype, field.accept)
      onChange(field.fieldname, uploaded.path)
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'Upload failed. Please try again.')
    } finally {
      setUploading(false)
      if (inputRef.current) inputRef.current.value = ''
    }
  }

  const hint = localError || error

  return (
    <div className={gapClass}>
      <Label htmlFor={id} className={labelClass}>{label}</Label>

      {hasValue ? (
        <div className="space-y-2">
          {isImage && (
            <a href={fileUrl(currentPath)} target="_blank" rel="noreferrer" className="block w-fit">
              <img
                src={fileUrl(currentPath)}
                alt={filename}
                className={cn('rounded-md border object-contain', compact ? 'max-h-24' : 'max-h-40')}
              />
            </a>
          )}

          {isAudio && (
            <audio controls src={fileUrl(currentPath)} className="w-full" />
          )}

          {!isImage && !isAudio && isPDF && (
            <iframe
              src={fileUrl(currentPath)}
              title={filename}
              className="h-64 w-full rounded-md border"
            />
          )}

          <div className="flex flex-wrap items-center justify-between gap-2">
            <a
              href={fileUrl(currentPath)}
              target="_blank"
              rel="noreferrer"
              className="inline-flex min-w-0 items-center gap-1.5 text-sm text-muted-foreground hover:text-primary"
              title={filename}
            >
              <FileText className="h-4 w-4 shrink-0" />
              <span className="truncate">{filename}</span>
              <ExternalLink className="h-3 w-3 shrink-0" />
            </a>

            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={disabled || uploading}
                onClick={openPicker}
              >
                {uploading ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Upload className="mr-1.5 h-3.5 w-3.5" />}
                Replace
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={disabled || uploading}
                onClick={() => onChange(field.fieldname, '')}
              >
                <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                Remove
              </Button>
            </div>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={openPicker}
          disabled={disabled || uploading}
          className={cn(
            'flex w-full flex-col items-center justify-center gap-1 rounded-xl border border-dashed bg-muted/20 text-center transition-colors hover:bg-muted/40 disabled:cursor-not-allowed disabled:opacity-50',
            compact ? 'px-3 py-3' : 'px-4 py-6',
          )}
        >
          {uploading ? (
            <>
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              <span className="text-sm font-medium">Uploading…</span>
            </>
          ) : (
            <>
              <Upload className="h-5 w-5 text-muted-foreground" />
              <span className="text-sm font-medium">Upload {kind}</span>
              <span className="text-xs text-muted-foreground">{help}</span>
            </>
          )}
        </button>
      )}

      <input
        ref={inputRef}
        type="file"
        id={id}
        accept={accept}
        className="sr-only"
        disabled={disabled || uploading}
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) handleSelect(file)
        }}
      />

      {hint && <p className="mt-1 text-sm text-destructive">{hint}</p>}
    </div>
  )
}
