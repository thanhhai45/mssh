import {createContext, useCallback, useContext, useEffect, useMemo, useState} from 'react'

import {api, type SessionState} from '@/lib/api'
import {asSessionId, newTabId, type SessionId, type TabId} from '@/lib/ids'
import {useSessionStatus} from '@/lib/session-status-store'
import {disposeTerminal} from '@/lib/terminal-session'

/**
 * One terminal on screen
 *
 * sessionId is null while the tab has nothing running - before the first
 * connect, and again after the session ends. The tab outlives its sessions on
 * purpose: the scrollback is usually the only record of why one of the died
 * */
export type Tab = {
    id: TabId
    connectionId: string
    sessionId: SessionId | null
}

/**
* How notable each state is, lowest first
*
* A machine with several tabs gets one dot, and it takes the colour of the most
* notable state among them, error wins because an error nobody sees is worse
* than one slightly overstated: the dot says "come and look", and which tab is broken is
* obviouse once you do
*/

const STATE_PRIORITY: Record<SessionState, number> = {
    error: 0,
    connecting: 1,
    connected: 2,
    disconnected: 3,
}

export type ConnectionSummary = {
    count: number
    state: SessionState | undefined
}

type TabsContextValue = {
    tabsFor: (connectionId: string) => Tab[]
    activeTabOf: (connectionId: string) => TabId | undefined
    focusTab: (connectionId: string, tabId: TabId) => void
    openTab: (connectionId: string) => TabId
    closeTab: (tabId: TabId) => void
    setSession: (tabId: TabId, sessionId: SessionId | null) => void
    summaryOf: (connectionId: string) => ConnectionSummary
}

const TabsContext = createContext<TabsContextValue | undefined>(undefined)

export function TabsProvider({children}: {children: React.ReactNode}) {
    const [tabs, setTabs] = useState<Tab[]>([])
    const [active, setActive] = useState<Record<string, TabId>>({})
    const {stateOf} = useSessionStatus()

    useEffect(() => {
        api.openSessions()
            .then((open) => {
                const recovered = open.map((info) => ({
                  id: newTabId(),
                  connectionId: info.connectionId,
                  sessionId: asSessionId(info.sessionId),
                }))
                if (recovered.length === 0) return

                setTabs(recovered)
                setActive(() => {
                    const next: Record<string, TabId> = {}
                    for (const tab of recovered) {
                        next[tab.connectionId] ??= tab.id
                    }
                    return next
                })
            })
            .catch(() => {
                // Nothing recoverd is the same as nothing open
            })
    }, [])

    const openTab = useCallback((connectionId: string) => {
        const tab: Tab = {id: newTabId(), connectionId, sessionId: null}
        setTabs((previous) => [...previous, tab])
        setActive((previous) => ({...previous, [connectionId]: tab.id}))
        return tab.id
    }, [])

    const closeTab = useCallback((tabId: TabId) => {
        setTabs((previous) => {
            const closing = previous.find((tab) => tab.id === tabId)
            if (!closing) return previous

            // Closing the window ends the conversation. Leaving the session
            // running would make it invisible - nothing on screen could reach
            // it, and it would reappear only after a reload
            if (closing.sessionId) {
                void api.disconnectSession(closing.sessionId).catch(() => {})
            }
            // Really destroy this one: the tab is gone, so the scrollback hsa nowhere left to be shown
            disposeTerminal(tabId)

            const remainning = previous.filter((tab) => tab.id !== tabId)

            setActive((previousActive) => {
                if (previousActive[closing.connectionId] !== tabId) return previousActive

                const next = {...previousActive}
                const sibling = remainning.find(
                    (tab) => tab.connectionId === closing.connectionId
                )
                if (sibling) {
                    next[closing.connectionId] = sibling.id
                } else {
                    delete next[closing.connectionId]
                }
                return next
            })

            return remainning
        })
    }, [])

    const setSession = useCallback((tabId: TabId, sessionId: SessionId | null) => {
        setTabs((previous) => previous.map((tab) => (tab.id === tabId ? {...tab, sessionId} : tab)))
    }, [])

    const focusTab = useCallback((connectionId: string, tabId: TabId) => {
        setActive((previous) => ({...previous, [connectionId]: tabId}))
    }, [])

    const value = useMemo<TabsContextValue>(() => {
        const tabsFor = (connectionId: string) => tabs.filter((tab) => tab.connectionId === connectionId)
    
        return {
            tabsFor,
            activeTabOf: (connectionId: string) => active[connectionId],
            focusTab,
            openTab,
            closeTab,
            setSession,
            summaryOf: (connectionId: string): ConnectionSummary => {
                const mine = tabsFor(connectionId)
                if (mine.length === 0) return {count: 0, state: undefined}

                let worst: SessionState = 'disconnected'
                for (const tab of mine) {
                    // A tab with no session is simply not connected
                    const state = tab.sessionId ? stateOf(tab.sessionId) : undefined
                    const resolved = state ?? 'disconnected'
                    if (STATE_PRIORITY[resolved] < STATE_PRIORITY[worst]) {
                        worst = resolved
                    }
                }
                return {count: mine.length, state: worst}
            },
        }
    }, [tabs, active, stateOf, focusTab, openTab, closeTab, setSession])

    return <TabsContext.Provider value={value}>{children}</TabsContext.Provider>
}

export function useTabs() {
    const context = useContext(TabsContext)
    if (!context) {
        throw new Error('useTabs must be used within a TabsProvider')
    }
    return context
}
