import {useEffect, useRef} from 'react'
import type {TabId} from '@/lib/ids'
import {attachTerminal, detachTerminal, resizeTerminal} from '@/lib/terminal-session'

/**
 * Borrows a terminal from the module-level store for as long as it is mounted.
 *
 * It creates nothing and destroys nothing. The terminal outlives this
 * component, which is the only way scrollback survives changing tabs.
 */
export function XtermView({tabId}: {tabId: TabId}) {
    const containerRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        const container = containerRef.current
        if (!container) return

        attachTerminal(tabId, container)

        const observer = new ResizeObserver(() => resizeTerminal(tabId))
        observer.observe(container)

        return () => {
            observer.disconnect()
            // Detach, never dispose: the session is still running.
            detachTerminal(tabId)
        }
    }, [tabId])

    return <div ref={containerRef} className="h-full w-full"/>
}
