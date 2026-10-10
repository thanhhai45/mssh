import {createContext, useCallback, useContext, useEffect, useMemo, useState} from 'react'

import {api, onTransferChanged, type Transfer} from '@/lib/api'
import type {SessionId} from '@/lib/ids'

type TransfersContextValue = {
    /** Every transfer this run has seen and the user has not cleared, newest first. */
    transfers: Transfer[]
    transfersOf: (sessionId: SessionId) => Transfer[]
    cancel: (id: string) => void
    /** Hides finished transfers of one session. Running ones stay. */
    clearFinished: (sessionId: SessionId) => void
}

const TransfersContext = createContext<TransfersContextValue | undefined>(undefined)

/**
 * Transfers run in Go and outlive any page, so the list of them lives up here,
 * above the router, for the same reason: leaving the server page must not lose
 * track of a download that is still going.
 */
export function TransfersProvider({children}: {children: React.ReactNode}) {
    const [byId, setById] = useState<Record<string, Transfer>>({})
    /** Cleared from view. Go still has them, so a reload would bring them back without this. */
    const [hidden, setHidden] = useState<Record<string, true>>({})

    useEffect(() => {
        return onTransferChanged((transfer) => {
            setById((previous) => ({...previous, [transfer.id]: transfer}))
        })
    }, [])

    // After a reload the page has forgotten everything; Go has not.
    useEffect(() => {
        api.listTransfers()
            .then((listed) => {
                setById((previous) => {
                    const next = {...previous}
                    for (const transfer of listed) {
                        // An event that arrived while this call was on its way
                        // is newer than what the call read: keep the event's.
                        next[transfer.id] ??= transfer
                    }
                    return next
                })
            })
            .catch(() => {
                // Nothing listed is the same as nothing running.
            })
    }, [])

    const cancel = useCallback((id: string) => {
        void api.cancelTransfer(id).catch(() => {})
    }, [])

    const value = useMemo<TransfersContextValue>(() => {
        const transfers = Object.values(byId)
            .filter((transfer) => !hidden[transfer.id])
            .sort((first, second) => second.startedAt - first.startedAt || first.id.localeCompare(second.id))
        return {
            transfers,
            transfersOf: (sessionId: SessionId) => transfers.filter((transfer) => transfer.sessionId === sessionId),
            cancel,
            clearFinished: (sessionId: SessionId) => {
                setHidden((previous) => {
                    const next = {...previous}
                    for (const transfer of Object.values(byId)) {
                        if (transfer.sessionId === sessionId && transfer.state !== 'running') {
                            next[transfer.id] = true
                        }
                    }
                    return next
                })
            },
        }
    }, [byId, hidden, cancel])

    return <TransfersContext.Provider value={value}>{children}</TransfersContext.Provider>
}

export function useTransfers() {
    const context = useContext(TransfersContext)
    if (!context) {
        throw new Error('useTransfers must be used within a TransfersProvider')
    }
    return context
}
