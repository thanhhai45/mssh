import {useEffect, useState} from 'react'
import {useParams} from '@tanstack/react-router'
import {Eraser, Plug, PlugZap, Plus, Search, X} from 'lucide-react'

import {PasswordDialog} from '@/components/password-dialog'
import {TerminalFindBar} from '@/components/terminal-find-bar'
import {XtermView} from '@/components/xterm-view'
import {Badge} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {
    api,
    describeConnection,
    errorMessage,
    kindMeta,
    needsPassword,
    sessionDotClass,
    type SessionState,
} from '@/lib/api'
import {accentTextClass} from '@/lib/colors'
import {KindIcon} from '@/components/kind-icon'
import {useSessionStatus} from '@/lib/session-status-store'
import {useTabs} from '@/lib/tabs-store'
import {
    attachSession,
    clearTerminal,
    detachSession,
    focusTerminal,
    resetTerminal,
    terminalSize,
} from '@/lib/terminal-session'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

const STATE_LABEL: Record<SessionState, string> = {
    connecting: 'Connecting…',
    connected: 'Connected',
    disconnected: 'Disconnected',
    error: 'Error',
}

export function ServerTerminalPage() {
    const {workspaceId, serverId} = useParams({strict: false})
    const {workspaces, connections, loading} = useWorkspaces()
    const {stateOf, statuses} = useSessionStatus()
    const {tabsFor, activeTabOf, focusTab, openTab, closeTab, setSession} = useTabs()

    const [busy, setBusy] = useState(false)
    const [localError, setLocalError] = useState<string | null>(null)
    /** The message the user dismissed, so the same one does not come back. */
    const [dismissed, setDismissed] = useState<string | null>(null)
    const [passwordHint, setPasswordHint] = useState<string | undefined>(undefined)
    const [askingPassword, setAskingPassword] = useState(false)
    const [finding, setFinding] = useState(false)

    const workspace = workspaces.find((candidate) => candidate.id === workspaceId)
    const connection = (connections[workspaceId ?? ''] ?? []).find(
        (candidate) => candidate.id === serverId,
    )

    const tabs = connection ? tabsFor(connection.id) : []
    const activeTabId = connection ? activeTabOf(connection.id) : undefined
    // Falling back to the first tab keeps the page usable for the frame between
    // opening a tab and the active id catching up.
    const activeTab = tabs.find((tab) => tab.id === activeTabId) ?? tabs[0]

    // The header follows whichever tab you are looking at. A machine has no one
    // state any more; a tab does.
    const state = activeTab?.sessionId ? stateOf(activeTab.sessionId) : undefined
    const isConnected = state === 'connected'
    const isConnecting = state === 'connecting'

    // Visiting a machine gives it a terminal, the way it always has. The tab is
    // a side effect of navigating here, which is why it belongs in an effect.
    const connectionId = connection?.id
    const tabCount = tabs.length
    useEffect(() => {
        if (!connectionId || tabCount > 0) return
        openTab(connectionId)
    }, [connectionId, tabCount, openTab])

    // ⌘F, ⌘T and ⌘W live here, not in the terminal module, because opening a
    // panel or a tab is React's business — a terminal has no idea either
    // exists. The keys reach this listener untouched: xterm leaves
    // ⌘-combinations alone on macOS, which is also why ⌘C and ⌘V still work
    // inside it.
    //
    // Declared after the values it depends on: a dependency array is evaluated
    // during render, so naming a const above its own declaration throws.
    useEffect(() => {
        function handleKeyDown(event: KeyboardEvent) {
            if (event.metaKey && event.key === 'f') {
                event.preventDefault()
                setFinding(true)
                return
            }
            if (!connectionId) return

            if (event.metaKey && event.key === 't') {
                event.preventDefault()
                openTab(connectionId)
                return
            }
            if (event.metaKey && event.key === 'w') {
                event.preventDefault()
                // The last tab stays. Closing it would only make the effect
                // above open an empty one straight back.
                if (tabCount > 1 && activeTabId) closeTab(activeTabId)
            }
        }

        document.addEventListener('keydown', handleKeyDown)
        return () => document.removeEventListener('keydown', handleKeyDown)
    }, [connectionId, tabCount, activeTabId, openTab, closeTab])

    async function connect(password: string) {
        if (!connection || !activeTab) return

        setBusy(true)
        setLocalError(null)
        try {
            // A fresh attempt gets a fresh screen, but the same terminal: the
            // node stays attached, so there is nothing for React to re-mount.
            resetTerminal(activeTab.id)

            const size = terminalSize(activeTab.id)
            const sessionId = await api.connectSession(
                connection.id, password, size.cols, size.rows)

            // Two places have to learn about the new session: React, so the
            // header and the sidebar follow it, and the terminal module, so the
            // output events land in this tab.
            setSession(activeTab.id, sessionId)
            attachSession(activeTab.id, sessionId)

            setAskingPassword(false)
            setPasswordHint(undefined)
        } catch (err) {
            if (needsPassword(err)) {
                // Not a failure: Go is telling us to go and ask.
                setPasswordHint(undefined)
                setAskingPassword(true)
                return
            }
            if (askingPassword) {
                // The password we just supplied did not work; stay open and say so.
                setPasswordHint(errorMessage(err))
                return
            }
            setLocalError(errorMessage(err))
        } finally {
            setBusy(false)
        }
    }

    async function handlePasswordSubmit(password: string, remember: boolean) {
        if (!connection) return

        if (remember) {
            // Save before connecting: if the connection works, the password was
            // right, and if it does not the user can correct it in the form.
            await api.setConnectionPassword(connection.id, password).catch(() => {})
        }
        await connect(password)
    }

    async function disconnect() {
        if (!activeTab?.sessionId) return

        setBusy(true)
        setLocalError(null)
        try {
            await api.disconnectSession(activeTab.sessionId)
            detachSession(activeTab.id)
            setSession(activeTab.id, null)
        } catch (err) {
            setLocalError(errorMessage(err))
        } finally {
            setBusy(false)
        }
    }

    if (loading) {
        return <p className="text-sm text-muted-foreground">Loading…</p>
    }
    if (!workspace || !connection) {
        return <p className="text-sm text-muted-foreground">Connection not found.</p>
    }

    // One notice, two sources: the connect call and the status event. They used
    // to be two banners, the second having to exclude the first with !error.
    const statusMessage =
        activeTab?.sessionId && state === 'error'
            ? statuses[activeTab.sessionId]?.message
            : undefined
    const notice = localError ?? statusMessage ?? null
    const showNotice = notice !== null && notice !== dismissed

    return (
        <div className="flex flex-1 flex-col gap-4 duration-300 animate-in fade-in">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                    <div className="flex items-center gap-2">
                        <span
                            className={cn(
                                'size-2 shrink-0 rounded-full transition-colors duration-200',
                                sessionDotClass(state),
                            )}
                        />
                        <h1 className={cn('truncate text-2xl font-semibold tracking-tight', accentTextClass(workspace.color))}>
                            {connection.name}
                        </h1>
                        <Badge variant="secondary" className="shrink-0 gap-1">
                            <KindIcon kind={connection.kind} className="size-3"/>
                            {kindMeta(connection.kind).label}
                        </Badge>
                    </div>
                    <p className="truncate text-sm text-muted-foreground">
                        {describeConnection(connection)}
                        {state ? ` · ${STATE_LABEL[state]}` : ''}
                    </p>
                </div>
                {/* One child, not three: the parent uses justify-between, which
                    spaces its direct children apart. */}
                <div className="flex items-center gap-2">
                    <Button
                        variant="ghost"
                        onClick={() => setFinding(true)}
                        title="Find in terminal (⌘F)"
                    >
                        <Search/>
                        Find
                    </Button>
                    <Button
                        variant="ghost"
                        onClick={() => activeTab && clearTerminal(activeTab.id)}
                        title="Clear the terminal (⌘K)"
                    >
                        <Eraser/>
                        Clear
                    </Button>
                    {isConnected ? (
                        <Button variant="outline" onClick={disconnect} disabled={busy}>
                            <PlugZap/>
                            {busy ? 'Disconnecting…' : 'Disconnect'}
                        </Button>
                    ) : (
                        <Button onClick={() => connect('')} disabled={busy || isConnecting}>
                            <Plug/>
                            {busy || isConnecting ? 'Connecting…' : 'Connect'}
                        </Button>
                    )}
                </div>
            </div>

            {showNotice && (
                <div className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {/* pre-wrap because explainHandshakeFailure answers in
                        paragraphs, and without it they run into one block. */}
                    <p className="min-w-0 flex-1 whitespace-pre-wrap">{notice}</p>
                    <button
                        type="button"
                        onClick={() => {
                            setDismissed(notice)
                            setLocalError(null)
                        }}
                        className="shrink-0 rounded p-0.5 hover:bg-destructive/10"
                        title="Dismiss"
                    >
                        <X className="size-4"/>
                    </button>
                </div>
            )}

            {finding && activeTab && (
                <TerminalFindBar
                    tabId={activeTab.id}
                    onClose={() => {
                        setFinding(false)
                        // Without this the keyboard is left on an input that no
                        // longer exists, and typing goes nowhere.
                        focusTerminal(activeTab.id)
                    }}
                />
            )}

            <div className="flex items-center gap-1 overflow-x-auto border-b pb-1">
                {tabs.map((tab, index) => {
                    const tabState = tab.sessionId ? stateOf(tab.sessionId) : undefined
                    return (
                        <button
                            key={tab.id}
                            type="button"
                            onClick={() => focusTab(connection.id, tab.id)}
                            className={cn(
                                'group flex shrink-0 items-center gap-2 rounded-t-md px-3 py-1.5 text-sm',
                                tab.id === activeTab?.id
                                    ? 'bg-accent text-accent-foreground'
                                    : 'text-muted-foreground hover:bg-accent/50',
                            )}
                        >
                            <span className={cn('size-1.5 rounded-full', sessionDotClass(tabState))}/>
                            <span>Terminal {index + 1}</span>
                            {/* The last tab has no close button: closing it would
                                only make the effect above open an empty one
                                straight back. A span, not a button — a button
                                inside a button is invalid HTML. */}
                            {tabs.length > 1 && (
                                <span
                                    role="button"
                                    tabIndex={-1}
                                    onClick={(event) => {
                                        event.stopPropagation()
                                        closeTab(tab.id)
                                    }}
                                    className="rounded p-0.5 opacity-0 hover:bg-background group-hover:opacity-100"
                                >
                                    <X className="size-3"/>
                                </span>
                            )}
                        </button>
                    )
                })}

                <button
                    type="button"
                    onClick={() => openTab(connection.id)}
                    className="shrink-0 rounded-md p-1.5 text-muted-foreground hover:bg-accent"
                    title="New terminal (⌘T)"
                >
                    <Plus className="size-4"/>
                </button>
            </div>

            <div className="h-[75vh] min-h-[360px] overflow-hidden rounded-lg border bg-[var(--terminal-background)] p-3">
                {/* Only the active tab is mounted. Switching detaches one
                    terminal and attaches another; both stay alive in the module,
                    so each tab keeps its own scrollback. */}
                {activeTab && <XtermView key={activeTab.id} tabId={activeTab.id}/>}
            </div>

            {askingPassword && (
                <PasswordDialog
                    connectionName={connection.name}
                    hint={passwordHint}
                    onSubmit={handlePasswordSubmit}
                    onCancel={() => {
                        setAskingPassword(false)
                        setPasswordHint(undefined)
                    }}
                />
            )}
        </div>
    )
}
