import {useEffect, useState} from 'react'
import {useParams} from '@tanstack/react-router'
import {Eraser, Plug, PlugZap, Search} from 'lucide-react'

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
import {useSessionStatus} from '@/lib/session-status-store'
import {
    clearTerminal,
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

    const [busy, setBusy] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [passwordHint, setPasswordHint] = useState<string | undefined>(undefined)
    const [askingPassword, setAskingPassword] = useState(false)
    const [finding, setFinding] = useState(false)

    // ⌘F lives here, not in the terminal module, because opening a panel is
    // React's business — a terminal has no idea a find bar exists. The key
    // reaches this listener untouched: xterm leaves ⌘-combinations alone on
    // macOS, which is also why ⌘C and ⌘V still work inside it.
    useEffect(() => {
        function handleKeyDown(event: KeyboardEvent) {
            if (event.metaKey && event.key === 'f') {
                event.preventDefault()
                setFinding(true)
            }
        }

        document.addEventListener('keydown', handleKeyDown)
        return () => document.removeEventListener('keydown', handleKeyDown)
    }, [])

    const workspace = workspaces.find((candidate) => candidate.id === workspaceId)
    const connection = (connections[workspaceId ?? ''] ?? []).find(
        (candidate) => candidate.id === serverId,
    )

    const state = connection ? stateOf(connection.id) : undefined
    const isConnected = state === 'connected'
    const isConnecting = state === 'connecting'

    async function connect(password: string) {
        if (!connection) return

        setBusy(true)
        setError(null)
        try {
            // A fresh attempt gets a fresh screen, but the same terminal: the
            // node stays attached, so there is nothing for React to re-mount.
            resetTerminal(connection.id)

            const size = terminalSize(connection.id)
            await api.connectSession(connection.id, password, size.cols, size.rows)

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
            setError(errorMessage(err))
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
        if (!connection) return

        setBusy(true)
        setError(null)
        try {
            await api.disconnectSession(connection.id)
        } catch (err) {
            setError(errorMessage(err))
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

    const statusMessage = statuses[connection.id]?.message

    return (
        <div className="flex flex-1 flex-col gap-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                    <div className="flex items-center gap-2">
                        <span className={cn('size-2 shrink-0 rounded-full', sessionDotClass(state))}/>
                        <h1 className="truncate text-2xl font-semibold tracking-tight">
                            {connection.name}
                        </h1>
                        <Badge variant="secondary">{kindMeta(connection.kind).label}</Badge>
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
                        onClick={() => clearTerminal(connection.id)}
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

            {error && (
                <p className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {error}
                </p>
            )}
            {!error && state === 'error' && statusMessage && (
                <p className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {statusMessage}
                </p>
            )}

            {finding && (
                <TerminalFindBar
                    connectionId={connection.id}
                    onClose={() => {
                        setFinding(false)
                        // Without this the keyboard is left on an input that no
                        // longer exists, and typing goes nowhere.
                        focusTerminal(connection.id)
                    }}
                />
            )}

            <div className="h-[75vh] min-h-[360px] overflow-hidden rounded-lg border bg-[var(--terminal-background)] p-3">
                <XtermView connectionId={connection.id}/>
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
