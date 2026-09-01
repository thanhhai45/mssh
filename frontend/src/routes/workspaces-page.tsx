import { Server } from 'lucide-react';

import {Badge} from '@/components/ui/badge';
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {Skeleton} from '@/components/ui/skeleton';
import { useWorkspaces } from '@/lib/workspaces-store';

export function WorkspacesPage() {
    const { workspaces, connections, loading, error } = useWorkspaces();
    return (
        <div className="flex flex-col gap-6">
            <div>
                <h1 className="text-2xl font-semibold tracking-tight">Workspaces</h1>
                <p className="text-sm text-muted-foreground">
                    Choose a workspace to.
                </p>
            </div>

            { error && (
                <p className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {error}
                </p>
            )}

            {loading ? (
                <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
                    <Skeleton className="h-28"/>
                    <Skeleton className="h-28"/>
                    <Skeleton className="h-28"/>
                </div>
            ) : workspaces.length === 0 ? (
                <p className="text-sm text-muted-foreground">No workspaces yet.</p>
            ) : (
                <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
                    {workspaces.map((workspace) => {
                        const list = connections[workspace.id] ?? []
                        return (
                            <Card key={workspace.id}>
                                <CardHeader>
                                    <div className="flex items-center gap-2">
                                        <Server className="size-4 text-muted-foreground"/>
                                        <CardTitle>{workspace.name}</CardTitle>
                                    </div>
                                    {workspace.awsProfile && (
                                        <CardAction>
                                            <Badge variant="secondary">{workspace.awsProfile}</Badge>
                                        </CardAction>
                                    )}
                                </CardHeader>
                                <CardContent>
                                    <span className="text-sm text-muted-foreground">
                                        {list.length} connection{list.length === 1 ? '' : 's'}
                                    </span>
                                </CardContent>
                            </Card>
                        )
                    })}
                </div>
            )}
        </div>
    )
}