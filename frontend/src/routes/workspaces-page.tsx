import {Server} from 'lucide-react'

import {Badge} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {Card, CardAction, CardContent, CardHeader, CardTitle} from '@/components/ui/card'
import {useWorkspaces} from '@/lib/workspaces-store'

export function WorkspacesPage() {
    const {workspaces} = useWorkspaces()

    return (
        <div className="flex flex-col gap-6">
            <div>
                <h1 className="text-2xl font-semibold tracking-tight">Workspaces</h1>
                <p className="text-sm text-muted-foreground">
                    Choose a workspace to connect to.
                </p>
            </div>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
                {workspaces.map((workspace) => (
                    <Card key={workspace.id}>
                        <CardHeader>
                            <div className="flex items-center gap-2">
                                <Server className="size-4 text-muted-foreground"/>
                                <CardTitle>{workspace.name}</CardTitle>
                            </div>
                            <CardAction>
                                <Badge variant={workspace.status === 'connected' ? 'default' : 'secondary'}>
                                    {workspace.status === 'connected' ? 'Connected' : 'Offline'}
                                </Badge>
                            </CardAction>
                        </CardHeader>
                        <CardContent className="flex items-center justify-between gap-2">
                            <span className="text-sm text-muted-foreground">
                                {workspace.servers.length} server{workspace.servers.length === 1 ? '' : 's'}
                            </span>
                            <Button size="sm" variant={workspace.status === 'connected' ? 'outline' : 'default'}>
                                {workspace.status === 'connected' ? 'Disconnect' : 'Connect'}
                            </Button>
                        </CardContent>
                    </Card>
                ))}
            </div>
        </div>
    )
}
