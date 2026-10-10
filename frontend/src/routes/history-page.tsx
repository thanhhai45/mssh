import {useEffect, useMemo, useState} from 'react'
import {Download, History, RefreshCw} from 'lucide-react'

import {KindIcon} from '@/components/kind-icon'
import {Badge} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {
    api,
    errorMessage,
    kindMeta,
    type SessionLogEntry,
    type SessionLogFilter,
} from '@/lib/api'
import {formatDuration} from '@/lib/relative-time'
import {useSessionStatus} from '@/lib/session-status-store'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

/** How far back to look. A date picker can come later; these answer "this week". */
const RANGES = [
    {value: 'today', label: 'Today', days: 1},
    {value: '7d', label: 'Last 7 days', days: 7},
    {value: '30d', label: 'Last 30 days', days: 30},
    {value: 'all', label: 'All time', days: 0},
] as const

type RangeValue = (typeof RANGES)[number]['value']

/** Start of the local day `days - 1` days ago, in epoch seconds; 0 for all time. */
function rangeStart(range: RangeValue): number {
    const days = RANGES.find((option) => option.value === range)?.days ?? 0
    if (days === 0) return 0
    const start = new Date()
    start.setHours(0, 0, 0, 0)
    start.setDate(start.getDate() - (days - 1))
    return Math.floor(start.getTime() / 1000)
}

/**
 * How each ending reads. Must cover every reason in store/sessionlog.go; an
 * empty reason is a session still open.
 */
const ENDINGS: Record<string, {label: string; variant: 'secondary' | 'destructive' | 'outline'; className?: string}> = {
    '': {label: 'Open', variant: 'outline', className: 'border-emerald-500/50 text-emerald-600 dark:text-emerald-400'},
    'closed': {label: 'Disconnected', variant: 'secondary'},
    'ended': {label: 'Ended', variant: 'secondary'},
    'failed': {label: 'Failed', variant: 'destructive'},
    'connection-deleted': {label: 'Connection deleted', variant: 'secondary'},
    'app-quit': {label: 'App quit', variant: 'secondary'},
    'interrupted': {label: 'Interrupted', variant: 'outline', className: 'border-amber-500/50 text-amber-600 dark:text-amber-400'},
}

/** The cap on rows shown. The export has no cap: it writes every match. */
const SHOWN = 500

const selectClass =
    'h-9 rounded-md border border-input bg-transparent px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30'

export function HistoryPage() {
    const {workspaces, connections} = useWorkspaces()
    // Changes on every session opening or closing anywhere in the app: the
    // signal to read the log again, so an open History page keeps up.
    const {statuses} = useSessionStatus()

    const [workspaceId, setWorkspaceId] = useState('')
    const [connectionId, setConnectionId] = useState('')
    const [range, setRange] = useState<RangeValue>('7d')
    const [entries, setEntries] = useState<SessionLogEntry[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [notice, setNotice] = useState<string | null>(null)
    const [reloads, setReloads] = useState(0)
    // When the rows were read, so an open session's running time is measured
    // against the same moment for every row — and render stays pure.
    const [readAt, setReadAt] = useState(0)

    const filter = useMemo<Partial<SessionLogFilter>>(
        () => ({workspaceId, connectionId, from: rangeStart(range)}),
        [workspaceId, connectionId, range],
    )

    useEffect(() => {
        let cancelled = false
        api.listSessionLog({...filter, limit: SHOWN})
            .then((rows) => {
                if (cancelled) return
                setEntries(rows)
                setReadAt(Math.floor(Date.now() / 1000))
                setError(null)
            })
            .catch((err) => {
                if (!cancelled) setError(errorMessage(err))
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [filter, statuses, reloads])

    // The machines a filter can name: the chosen workspace's, or everyone's.
    // Deleted machines still appear in the log; they are just not offered here.
    const machineOptions = (workspaceId ? [workspaceId] : workspaces.map((workspace) => workspace.id))
        .flatMap((id) => connections[id] ?? [])

    async function exportAs(format: 'csv' | 'json') {
        setNotice(null)
        try {
            const path = await api.exportSessionLog(filter, format)
            if (path) setNotice(`Saved to ${path}`)
        } catch (err) {
            setError(errorMessage(err))
        }
    }

    return (
        // min-w-0: the table's cells do not wrap, and without this a wide table
        // would stretch the whole page instead of scrolling inside its frame.
        <div className="flex min-w-0 flex-col gap-6 duration-300 animate-in fade-in">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h1 className="text-2xl font-semibold tracking-tight">History</h1>
                    <p className="text-sm text-muted-foreground">
                        Every session: which machine, when, for how long, and how it ended. Kept
                        on this machine, and kept even when a connection is deleted.
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => void exportAs('csv')}>
                        <Download/>
                        CSV
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => void exportAs('json')}>
                        <Download/>
                        JSON
                    </Button>
                </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
                <select
                    aria-label="Workspace"
                    className={selectClass}
                    value={workspaceId}
                    onChange={(event) => {
                        setWorkspaceId(event.target.value)
                        // A machine from another workspace would filter to nothing.
                        setConnectionId('')
                    }}
                >
                    <option value="">All workspaces</option>
                    {workspaces.map((workspace) => (
                        <option key={workspace.id} value={workspace.id}>{workspace.name}</option>
                    ))}
                </select>
                <select
                    aria-label="Machine"
                    className={selectClass}
                    value={connectionId}
                    onChange={(event) => setConnectionId(event.target.value)}
                >
                    <option value="">All machines</option>
                    {machineOptions.map((connection) => (
                        <option key={connection.id} value={connection.id}>{connection.name}</option>
                    ))}
                </select>
                <select
                    aria-label="Time range"
                    className={selectClass}
                    value={range}
                    onChange={(event) => setRange(event.target.value as RangeValue)}
                >
                    {RANGES.map((option) => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                </select>
                <Button variant="ghost" size="icon" onClick={() => setReloads((count) => count + 1)} title="Refresh">
                    <RefreshCw/>
                </Button>
                <span className="ml-auto text-xs text-muted-foreground">
                    {entries.length === SHOWN
                        ? `Showing the latest ${SHOWN}. Export writes every match.`
                        : `${entries.length} session${entries.length === 1 ? '' : 's'}`}
                </span>
            </div>

            {error && (
                <p className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {error}
                </p>
            )}
            {notice && <p className="text-sm text-muted-foreground">{notice}</p>}

            {!loading && entries.length === 0 ? (
                <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed py-16 text-center">
                    <History className="size-6 text-muted-foreground"/>
                    <p className="max-w-sm text-sm text-muted-foreground">
                        No sessions in this range. Every connection you open from now on is
                        recorded here.
                    </p>
                </div>
            ) : (
                <div className="overflow-x-auto rounded-lg border">
                    <table className="w-full text-sm">
                        <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
                            <tr>
                                <th className="px-3 py-2 font-medium">Opened</th>
                                <th className="px-3 py-2 font-medium">Duration</th>
                                <th className="px-3 py-2 font-medium">Machine</th>
                                <th className="px-3 py-2 font-medium">Workspace</th>
                                <th className="px-3 py-2 font-medium">AWS</th>
                                <th className="px-3 py-2 font-medium">Ended</th>
                            </tr>
                        </thead>
                        <tbody>
                            {entries.map((entry) => (
                                <SessionRow key={entry.id} entry={entry} readAt={readAt}/>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}

function SessionRow({entry, readAt}: {entry: SessionLogEntry; readAt: number}) {
    const ending = ENDINGS[entry.endReason] ?? {label: entry.endReason, variant: 'secondary' as const}
    const opened = new Date(entry.openedAt * 1000)
    const duration = entry.closedAt > 0
        ? formatDuration(entry.closedAt - entry.openedAt)
        : entry.endReason === ''
            ? formatDuration(Math.max(0, readAt - entry.openedAt))
            : '—'

    return (
        <tr className="border-t align-top">
            <td className="px-3 py-2 whitespace-nowrap tabular-nums" title={opened.toISOString()}>
                {opened.toLocaleString()}
            </td>
            <td className="px-3 py-2 whitespace-nowrap tabular-nums text-muted-foreground">{duration}</td>
            <td className="px-3 py-2">
                <div className="flex items-center gap-2">
                    <KindIcon kind={entry.kind} className="size-4 shrink-0 text-muted-foreground"/>
                    <span className="font-medium whitespace-nowrap">{entry.connectionName}</span>
                </div>
                <div className="pl-6 text-xs whitespace-nowrap text-muted-foreground">
                    {kindMeta(entry.kind).label} · {entry.username ? `${entry.username}@` : ''}{entry.target}
                </div>
            </td>
            <td className="px-3 py-2 whitespace-nowrap text-muted-foreground">{entry.workspaceName}</td>
            <td className="px-3 py-2 text-xs whitespace-nowrap text-muted-foreground">
                {entry.awsRegion || entry.awsProfile ? (
                    <>
                        <div>{[entry.awsProfile, entry.awsRegion].filter(Boolean).join(' · ')}</div>
                        <div>{entry.awsCredentialsSource === 'stored' ? 'stored keys' : 'AWS CLI'}</div>
                    </>
                ) : '—'}
            </td>
            <td className="px-3 py-2">
                <Badge variant={ending.variant} className={cn('whitespace-nowrap', ending.className)}>
                    {ending.label}
                </Badge>
                {entry.endMessage && (
                    <p className="mt-1 max-w-xs truncate text-xs text-muted-foreground" title={entry.endMessage}>
                        {entry.endMessage}
                    </p>
                )}
            </td>
        </tr>
    )
}
