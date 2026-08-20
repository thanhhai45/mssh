import {useParams} from '@tanstack/react-router'

import {XtermView} from '@/components/xterm-view'
import {useWorkspaces} from '@/lib/workspaces-store'

export function ServerTerminalPage() {
    const {workspaceId, serverId} = useParams({strict: false})
    const {workspaces} = useWorkspaces()
    const workspace = workspaces.find((w) => w.id === workspaceId)
    const server = workspace?.servers.find((s) => s.id === serverId)

    if (!workspace || !server) {
        return <p className="text-sm text-muted-foreground">Server not found.</p>
    }

    return (
        <div className="flex flex-1 flex-col gap-4">
            <div>
                <h1 className="text-2xl font-semibold tracking-tight">{server.name}</h1>
                <p className="text-sm text-muted-foreground">{server.command}</p>
            </div>
            <div className="h-[65vh] min-h-[360px] overflow-hidden rounded-lg border bg-[#09090b] p-3">
                <XtermView
                    key={server.id}
                    banner={[
                        `Connecting to ${workspace.name} · ${server.name}...`,
                        server.command,
                        'This is a local preview terminal — no SSH connection is wired up yet.',
                        '',
                    ]}
                />
            </div>
        </div>
    )
}
