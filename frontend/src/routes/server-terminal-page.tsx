import {useParams} from '@tanstack/react-router'

import {XtermView} from '@/components/xterm-view'
import {describeConnection} from '@/lib/api'
import {useWorkspaces} from '@/lib/workspaces-store'

const BANNER = [
    'Terminal is not wired to a real session yet.',
    '',
]

export function ServerTerminalPage() {
    const {workspaceId, serverId} = useParams({strict: false})
    const {workspaces, connections, loading} = useWorkspaces()

    const workspace = workspaces.find((w) => w.id === workspaceId)
    const connection = (connections[workspaceId ?? ''] ?? []).find((c) => c.id === serverId)

    if (loading) {
        return <p className="text-sm text-muted-foreground">Loading…</p>
    }
    if (!workspace || !connection) {
        return <p className="text-sm text-muted-foreground">Connection not found.</p>
    }

    return (
        <div className="flex flex-1 flex-col gap-4">
            <div>
                <h1 className="text-2xl font-semibold tracking-tight">{connection.name}</h1>
                <p className="text-sm text-muted-foreground">{describeConnection(connection)}</p>
            </div>
            <div className="h-[65vh] min-h-[360px] overflow-hidden rounded-lg border bg-[#09090b] p-3">
                <XtermView key={connection.id} banner={BANNER}/>
            </div>
        </div>
    )
}
