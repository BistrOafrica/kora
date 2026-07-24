import type { ViewComponentProps } from './registry'

/**
 * PrintLayout wraps children in a print-optimized container.
 * On screen, renders normally. On print (@media print), hides
 * navigation and applies print-friendly styles.
 */
export default function PrintLayout(props: ViewComponentProps) {
  const { config, children } = props

  return (
    <div className="print-layout">
      <style>{`
        @media print {
          body * { visibility: hidden; }
          .print-layout, .print-layout * { visibility: visible; }
          .print-layout { position: absolute; left: 0; top: 0; width: 100%; }
          .no-print { display: none !important; }
        }
      `}</style>
      <div className="max-w-2xl mx-auto">
        {config.label && (
          <div className="text-center mb-6 no-print">
            <h2 className="text-lg font-bold">{config.label}</h2>
          </div>
        )}
        {children}
      </div>
    </div>
  )
}
