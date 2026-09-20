import {Link} from '@tanstack/react-router'
import {Clock, Plus, TerminalSquare} from 'lucide-react'

import {WorkspaceDialog} from '@/components/workspace-dialog'
import {Badge} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card'
import {Skeleton} from '@/components/ui/skeleton'
import {
    kindMeta,
    sessionDotClass,
    type Connection,
    type SessionState,
} from '@/lib/api'
import {accentTextClass, accentTintClass, swatchClass} from '@/lib/colors'
import {KindIcon} from '@/components/kind-icon'
import {relativeTime} from '@/lib/relative-time'
import {useSessionStatus} from '@/lib/session-status-store'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'
import {useState} from 'react'

/** How many connections the "Recent" strip shows before it stops. */
const RECENT_LIMIT = 6

export function WorkspacesPage() {
    const {workspaces, connections, loading, error} = useWorkspaces()
    const {stateOf} = useSessionStatus()
    const [newWorkspaceOpen, setNewWorkspaceOpen] = useState(false)

    // Flatten every workspace's list once, so "recent" can cross workspaces —
    // which is the whole point: you resume work, not a folder.
    const everyConnection = workspaces.flatMap((workspace) =>
        (connections[workspace.id] ?? []).map((connection) => ({workspace, connection})),
    )

    const recent = everyConnection
        .filter((row) => row.connection.lastUsedAt > 0)
        .sort((a, b) => b.connection.lastUsedAt - a.connection.lastUsedAt)
        .slice(0, RECENT_LIMIT)

    return (
        <div className="flex flex-col gap-8 duration-300 animate-in fade-in">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h1 className="text-2xl font-semibold tracking-tight">Workspaces</h1>
                    <p className="text-sm text-muted-foreground">
                        {workspaces.length === 0
                            ? 'Nothing here yet.'
                            : `${workspaces.length} workspace${workspaces.length === 1 ? '' : 's'}, ${everyConnection.length} connection${everyConnection.length === 1 ? '' : 's'}.`}
                    </p>
                </div>
                <Button variant="outline" onClick={() => setNewWorkspaceOpen(true)}>
                    <Plus/>
                    New workspace
                </Button>
            </div>

            {error && (
                <p className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {error}
                </p>
            )}

            {loading ? (
                <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
                    <Skeleton className="h-40"/>
                    <Skeleton className="h-40"/>
                    <Skeleton className="h-40"/>
                </div>
            ) : workspaces.length === 0 ? (
                <EmptyWorkspaces onCreate={() => setNewWorkspaceOpen(true)}/>
            ) : (
                <>
                    {recent.length > 0 && (
                        <section className="flex flex-col gap-3">
                            <h2 className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                                <Clock className="size-3.5"/>
                                Pick up where you left off
                            </h2>
                            <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                                {recent.map(({workspace, connection}) => (
                                    <RecentRow
                                        key={connection.id}
                                        connection={connection}
                                        workspaceColor={workspace.color}
                                        workspaceName={workspace.name}
                                        state={stateOf(connection.id)}
                                    />
                                ))}
                            </div>
                        </section>
                    )}

                    <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
                        {workspaces.map((workspace) => (
                            <WorkspaceCard
                                key={workspace.id}
                                name={workspace.name}
                                color={workspace.color}
                                awsProfile={workspace.awsProfile}
                                connections={connections[workspace.id] ?? []}
                            />
                        ))}
                    </section>
                </>
            )}

            {newWorkspaceOpen && (
                <WorkspaceDialog workspace={null} onClose={() => setNewWorkspaceOpen(false)}/>
            )}
        </div>
    )
}

function RecentRow({
    connection,
    workspaceColor,
    workspaceName,
    state,
}: {
    connection: Connection
    workspaceColor: string
    workspaceName: string
    state: SessionState | undefined
}) {

    return (
        <Link
            // The typed form, not a built string: the router checks the params
            // against the route pattern at compile time.
            to="/workspaces/$workspaceId/servers/$serverId"
            params={{workspaceId: connection.workspaceId, serverId: connection.id}}
            className="flex items-center gap-3 rounded-lg border bg-card p-3 transition-colors hover:bg-accent"
        >
            <KindIcon
                kind={connection.kind}
                className={cn('size-4 shrink-0', accentTextClass(workspaceColor))}
            />
            <div className="flex min-w-0 flex-1 flex-col leading-tight">
                <span className="truncate text-sm font-medium">{connection.name}</span>
                <span className="truncate text-xs text-muted-foreground">
                    {workspaceName} · {relativeTime(connection.lastUsedAt)}
                </span>
            </div>
            <span
                className={cn(
                    'size-2 shrink-0 rounded-full transition-colors duration-200',
                    sessionDotClass(state),
                )}
            />
        </Link>
    )
}

function WorkspaceCard({
    name,
    color,
    awsProfile,
    connections,
}: {
    name: string
    color: string
    awsProfile: string
    connections: Connection[]
}) {
    // Count by kind rather than listing every connection: at a glance, what
    // matters about a workspace is how you get into it.
    const byKind = new Map<string, number>()
    for (const connection of connections) {
        byKind.set(connection.kind, (byKind.get(connection.kind) ?? 0) + 1)
    }

    const lastUsed = connections.reduce(
        (newest, connection) => Math.max(newest, connection.lastUsedAt),
        0,
    )

    return (
        <Card
            className={cn(
                'overflow-hidden bg-linear-to-br to-transparent transition-shadow hover:shadow-md',
                accentTintClass(color),
            )}
        >
            <CardHeader>
                <div className="flex items-center gap-2">
                    <span className={cn('size-2.5 shrink-0 rounded-full', swatchClass(color))}/>
                    <CardTitle className={cn('truncate', accentTextClass(color))}>
                        {name}
                    </CardTitle>
                    {awsProfile && (
                        <Badge variant="secondary" className="ml-auto shrink-0">
                            {awsProfile}
                        </Badge>
                    )}
                </div>
            </CardHeader>

            <CardContent className="flex flex-col gap-3">
                {connections.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        No connections yet. Add one from the sidebar.
                    </p>
                ) : (
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                        {[...byKind].map(([kind, count]) => {
                            return (
                                <span
                                    key={kind}
                                    className="flex items-center gap-1.5 text-sm text-muted-foreground"
                                    title={kindMeta(kind).label}
                                >
                                    <KindIcon kind={kind} className="size-4"/>
                                    <span className="tabular-nums">{count}</span>
                                </span>
                            )
                        })}
                    </div>
                )}

                <p className="text-xs text-muted-foreground">
                    {connections.length} connection{connections.length === 1 ? '' : 's'}
                    {lastUsed > 0 && ` · last used ${relativeTime(lastUsed)}`}
                </p>
            </CardContent>
        </Card>
    )
}

function EmptyWorkspaces({onCreate}: {onCreate: () => void}) {
    return (
        <div className="flex flex-col items-center gap-4 rounded-lg border border-dashed py-16 text-center">
            <div className="flex size-12 items-center justify-center rounded-xl bg-muted">
                <TerminalSquare className="size-6 text-muted-foreground"/>
            </div>
            <div className="max-w-sm">
                <p className="font-medium">No workspaces yet</p>
                <p className="text-sm text-muted-foreground">
                    A workspace is a drawer for connections that belong together — one
                    customer, one AWS account, one environment. Most people start with
                    one called “Production” and regret nothing.
                </p>
            </div>
            <Button onClick={onCreate}>
                <Plus/>
                Create the first one
            </Button>
        </div>
    )
}
