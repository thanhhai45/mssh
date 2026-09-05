import {createContext, useContext, useEffect, useMemo, useState} from 'react'

import {api, onSessionStatus, type SessionState, type SessionStatus} from '@/lib/api'
import {hasTerminal, writeToTerminal} from '@/lib/terminal-session'

type SessionStatusContextValue = {
    /** Latest status per connection. A missing entry means never connected. */
    statuses: Record<string, SessionStatus>
    stateOf: (connectionId: string) => SessionState | undefined
    isBusy: (connectionId: string) => boolean
}

const SessionStatusContext = createContext<SessionStatusContextValue | undefined>(undefined)

export function SessionStatusProvider({children}: {children: React.ReactNode}) {
    const [statuses, setStatuses] = useState<Record<string, SessionStatus>>({})

    useEffect(() => {
        const stopListening = onSessionStatus((status) => {
            setStatuses((previous) => ({...previous, [status.connectionId]: status}))

            // Say what happened inside the terminal itself, where the user is
            // already looking. The terminal is kept: its scrollback is often
            // the only clue about why the session ended.
            if (!hasTerminal(status.connectionId)) return

            if (status.state === 'disconnected') {
                writeToTerminal(status.connectionId, '\r\n\x1b[90m— session ended —\x1b[0m\r\n')
            } else if (status.state === 'error' && status.message) {
                writeToTerminal(status.connectionId, `\r\n\x1b[31m${status.message}\x1b[0m\r\n`)
            }
        })

        return stopListening
    }, [])

    // Sessions live in Go, not in the browser. After a page reload the frontend
    // has forgotten everything, so it has to ask what is still open.
    useEffect(() => {
        api.openSessionIds()
            .then((ids) => {
                setStatuses((previous) => {
                    const next = {...previous}
                    for (const connectionId of ids) {
                        next[connectionId] = {connectionId, state: 'connected', message: ''}
                    }
                    return next
                })
            })
            .catch(() => {
                // Not knowing is the same as "nothing open" for the UI.
            })
    }, [])

    const value = useMemo(
        () => ({
            statuses,
            stateOf: (connectionId: string) => statuses[connectionId]?.state,
            isBusy: (connectionId: string) => statuses[connectionId]?.state === 'connecting',
        }),
        [statuses],
    )

    return (
        <SessionStatusContext.Provider value={value}>{children}</SessionStatusContext.Provider>
    )
}

export function useSessionStatus() {
    const context = useContext(SessionStatusContext)
    if (!context) {
        throw new Error('useSessionStatus must be used within a SessionStatusProvider')
    }
    return context
}