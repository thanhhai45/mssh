import {createContext, useContext, useEffect, useMemo, useState} from 'react'

import {api, onSessionStatus, type SessionState, type SessionStatus} from '@/lib/api'
import type {SessionId} from '@/lib/ids'
import {writeToSessionTerminal} from '@/lib/terminal-session'

type SessionStatusContextValue = {
    /** Latest status per connection. A missing entry means never connected. */
    statuses: Record<string, SessionStatus>
    stateOf: (sessionsId: SessionId) => SessionState | undefined
    isBusy: (sessionId: SessionId) => boolean
}

const SessionStatusContext = createContext<SessionStatusContextValue | undefined>(undefined)

export function SessionStatusProvider({children}: {children: React.ReactNode}) {
    const [statuses, setStatuses] = useState<Record<string, SessionStatus>>({})

    useEffect(() => {
        const stopListening = onSessionStatus((status) => {
            setStatuses((previous) => ({...previous, [status.sessionId]: status}))

            // Mark the end of the session in the terminal, and nothing more.
            // The reason belongs in the banner above;: it is often several
            // paragraphs, and printing it here as well said everything twice.
            // 
            // This store knows about sessions but not about tabs, and it does
            // not need to - finding the terminal showing a session is the terminal module business.
            if (status.state === 'disconnected' || status.state === 'error') {
                writeToSessionTerminal(status.sessionId, '\r\n\x1b[90m— session ended —\x1b[0m\r\n')
            }
        })

        return stopListening
    }, [])

    // Sessions live in Go, not in the browser. After a page reload the frontend
    // has forgotten everything, so it has to ask what is still open.
    useEffect(() => {
        api.openSessions()
            .then((open) => {
                setStatuses((previous) => {
                    const next = {...previous}
                    for (const info of open) {
                        next[info.sessionId] = {
                            sessionId: info.sessionId,
                            connectionId: info.connectionId,
                            state: 'connected',
                            message: ''
                        }
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
            stateOf: (sessionId: SessionId) => statuses[sessionId]?.state,
            isBusy: (sessionId: SessionId) => statuses[sessionId]?.state === 'connecting',
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
