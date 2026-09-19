import {useEffect, useMemo, useRef, useState} from 'react'
import {useNavigate} from '@tanstack/react-router'
import {CornerDownLeft, Search} from 'lucide-react'

import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogTitle,
} from '@/components/ui/dialog'
import {describeConnection, sessionDotClass, type Connection, type Workspace} from '@/lib/api'
import {accentTextClass} from '@/lib/colors'
import {kindIcon} from '@/lib/kind-icons'
import {useSessionStatus} from '@/lib/session-status-store'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

type Entry = {workspace: Workspace; connection: Connection; haystack: string}

/**
 * Jump to any connection by typing part of its name.
 *
 * ⌘P, not ⌘K: ⌘K already clears the terminal, the way it does in iTerm2. A
 * shortcut that means two things means neither.
 */
export function CommandPalette() {
    const [open, setOpen] = useState(false)
    const [query, setQuery] = useState('')
    const [highlighted, setHighlighted] = useState(0)

    const navigate = useNavigate()
    const {workspaces, connections} = useWorkspaces()
    const {stateOf} = useSessionStatus()
    const inputRef = useRef<HTMLInputElement>(null)
    const listRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        function handleKeyDown(event: KeyboardEvent) {
            if (event.metaKey && event.key === 'p') {
                event.preventDefault()
                setOpen((wasOpen) => !wasOpen)
            }
        }

        document.addEventListener('keydown', handleKeyDown)
        return () => document.removeEventListener('keydown', handleKeyDown)
    }, [])

    // Every searchable field is folded into one lowercase string per entry, so
    // matching is a substring test rather than a scan of five fields.
    const entries = useMemo<Entry[]>(
        () =>
            workspaces.flatMap((workspace) =>
                (connections[workspace.id] ?? []).map((connection) => ({
                    workspace,
                    connection,
                    haystack: [
                        connection.name,
                        connection.target,
                        connection.username,
                        connection.kind,
                        workspace.name,
                    ]
                        .join(' ')
                        .toLowerCase(),
                })),
            ),
        [workspaces, connections],
    )

    const matches = useMemo(() => {
        const needle = query.trim().toLowerCase()
        const found = needle === ''
            ? entries
            : entries.filter((entry) => entry.haystack.includes(needle))

        // Most recently used first, so an empty box is already a useful list.
        return [...found]
            .sort((a, b) => b.connection.lastUsedAt - a.connection.lastUsedAt)
            .slice(0, 50)
    }, [entries, query])

    // A new query means a new list, and the old highlight index may point past
    // the end of it.
    useEffect(() => {
        setHighlighted(0)
    }, [query])

    // Reopening should not resume the last search.
    useEffect(() => {
        if (open) setQuery('')
    }, [open])

    useEffect(() => {
        listRef.current
            ?.querySelector('[data-highlighted="true"]')
            ?.scrollIntoView({block: 'nearest'})
    }, [highlighted, matches])

    function choose(entry: Entry) {
        setOpen(false)
        void navigate({
            to: '/workspaces/$workspaceId/servers/$serverId',
            params: {
                workspaceId: entry.connection.workspaceId,
                serverId: entry.connection.id,
            },
        })
    }

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent
                showCloseButton={false}
                // sm:max-w-xl too: the base class sets sm:max-w-sm, and a bare
                // max-w-xl would lose to it on any screen wider than 640px.
                className="top-[15%] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-xl"
                onOpenAutoFocus={(event) => {
                    event.preventDefault()
                    inputRef.current?.focus()
                }}
            >
                {/* Radix requires both for screen readers; neither belongs on screen. */}
                <DialogTitle className="sr-only">Go to connection</DialogTitle>
                <DialogDescription className="sr-only">
                    Search your connections by name, host or workspace.
                </DialogDescription>

                <div className="flex items-center gap-2 border-b px-3">
                    <Search className="size-4 shrink-0 text-muted-foreground"/>
                    <input
                        ref={inputRef}
                        value={query}
                        placeholder="Go to connection…"
                        className="h-11 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                        onChange={(event) => setQuery(event.target.value)}
                        onKeyDown={(event) => {
                            if (event.key === 'ArrowDown') {
                                event.preventDefault()
                                // Wrap around: the list is short and a dead end
                                // at the bottom is just an extra keystroke.
                                setHighlighted((index) => (index + 1) % Math.max(matches.length, 1))
                                return
                            }
                            if (event.key === 'ArrowUp') {
                                event.preventDefault()
                                setHighlighted((index) =>
                                    (index - 1 + matches.length) % Math.max(matches.length, 1),
                                )
                                return
                            }
                            if (event.key === 'Enter') {
                                event.preventDefault()
                                const entry = matches[highlighted]
                                if (entry) choose(entry)
                            }
                        }}
                    />
                </div>

                <div ref={listRef} className="max-h-80 overflow-y-auto p-1">
                    {matches.length === 0 ? (
                        <p className="px-3 py-6 text-center text-sm text-muted-foreground">
                            {entries.length === 0
                                ? 'No connections to jump to yet.'
                                : `Nothing matches “${query}”.`}
                        </p>
                    ) : (
                        matches.map((entry, index) => {
                            const Icon = kindIcon(entry.connection.kind)
                            const isHighlighted = index === highlighted

                            return (
                                <button
                                    key={entry.connection.id}
                                    type="button"
                                    data-highlighted={isHighlighted}
                                    // Mouse and keyboard drive the same single
                                    // highlight, so they can never disagree.
                                    onMouseMove={() => setHighlighted(index)}
                                    onClick={() => choose(entry)}
                                    className={cn(
                                        'flex w-full items-center gap-3 rounded-md px-3 py-2 text-left',
                                        isHighlighted && 'bg-accent',
                                    )}
                                >
                                    <Icon
                                        className={cn(
                                            'size-4 shrink-0',
                                            accentTextClass(entry.workspace.color),
                                        )}
                                    />
                                    <div className="flex min-w-0 flex-1 flex-col leading-tight">
                                        <span className="truncate text-sm">
                                            {entry.connection.name}
                                        </span>
                                        <span className="truncate text-xs text-muted-foreground">
                                            {entry.workspace.name} ·{' '}
                                            {describeConnection(entry.connection)}
                                        </span>
                                    </div>
                                    <span
                                        className={cn(
                                            'size-2 shrink-0 rounded-full',
                                            sessionDotClass(stateOf(entry.connection.id)),
                                        )}
                                    />
                                </button>
                            )
                        })
                    )}
                </div>

                <div className="flex items-center gap-3 border-t px-3 py-2 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1">
                        <CornerDownLeft className="size-3"/> open
                    </span>
                    <span>↑↓ move</span>
                    <span>esc close</span>
                </div>
            </DialogContent>
        </Dialog>
    )
}
