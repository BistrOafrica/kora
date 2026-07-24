import { useState, useRef, useEffect } from 'react'
import type { ViewComponentProps } from './registry'
import { Input } from '@/components/ui/input'
import { Search } from 'lucide-react'

/**
 * ScannerInput is an always-focused text input that fires on_scan actions
 * when the user types a barcode or product code and presses Enter.
 * Works with keyboard wedge barcode scanners (which send the code + Enter).
 */
export default function ScannerInput(props: ViewComponentProps) {
  const { config, onAction } = props
  const [value, setValue] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  // Auto-focus on mount and keep focused.
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && value.trim()) {
      onAction('on_scan', { barcode: value.trim(), raw: value.trim() })
      setValue('')
    }
  }

  const placeholder = config.bindings?.placeholder || 'Scan barcode or type to search...'
  const searchFields = config.bindings?.search_fields?.split(',').map(s => s.trim()) || ['barcode']

  return (
    <div className="relative">
      <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
      <Input
        ref={inputRef}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={() => inputRef.current?.focus()}
        placeholder={placeholder}
        className="pl-9 text-lg"
        autoFocus
      />
      <p className="mt-1 text-xs text-muted-foreground">
        Searches: {searchFields.join(', ')}
      </p>
    </div>
  )
}
