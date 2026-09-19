import {Link, useRouterState} from '@tanstack/react-router'
import {ChevronRight} from 'lucide-react'

import {sessionDotClass} from '@/lib/api'
import {accentTextClass} from '@/lib/colors'
import {useSessionStatus} from '@/lib/session-status-store'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

const STATE_LABEL: Record<string, string> = {
    connecting: 'connecting',
    connected: 'connected',
    disconnected: 'disconnected',
    error: 'error',
}

/**
 * Where you are, read out of the URL.
 *
 * The route already holds the answer, so nothing here is stored: no context, no
 * prop threaded down from the page. A breadcrumb that keeps its own state is a
 * breadcrumb that can disagree with the address bar.
 */
export function HeaderBreadcrumb() {
    const pathname = useRouterState({select: (state) => state.location.pathname})
    const {workspaces, connections} = useWorkspaces()
    const {stateOf} = useSessionStatus()

    const match = pathname.match(/^\/workspaces\/([^/]+)\/servers\/([^/]+)$/)

    if (pathname === '/themes') {
        return <Crumbs trail={['Appearance']}/>
    }
    if (!match) {
        return <Crumbs trail={['Workspaces']}/>
    }

    const [, workspaceId, connectionId] = match
    const workspace = workspaces.find((candidate) => candidate.id === workspaceId)
    const connection = (connections[workspaceId] ?? []).find(
        (candidate) => candidate.id === connectionId,
    )

    // The lists load after the first paint, so both can legitimately be missing
    // for a frame. Showing the trail without names beats showing nothing.
    const state = stateOf(connectionId)

    return (
        <nav className="flex min-w-0 items-center gap-1.5 text-sm">
            <Link to="/" className="shrink-0 text-muted-foreground hover:text-foreground">
                Workspaces
            </Link>
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60"/>
            <span
                className={cn(
                    'shrink-0 truncate',
                    workspace ? accentTextClass(workspace.color) : 'text-muted-foreground',
                )}
            >
                {workspace?.name ?? '…'}
            </span>
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60"/>
            <span className="truncate font-medium">{connection?.name ?? '…'}</span>
            {state && (
                <span className="flex shrink-0 items-center gap-1.5 pl-1 text-xs text-muted-foreground">
                    <span
                        className={cn(
                            'size-1.5 rounded-full transition-colors duration-200',
                            sessionDotClass(state),
                        )}
                    />
                    {STATE_LABEL[state]}
                </span>
            )}
        </nav>
    )
}

function Crumbs({trail}: {trail: string[]}) {
    return (
        <nav className="flex min-w-0 items-center gap-1.5 truncate text-sm font-medium">
            {trail.join(' › ')}
        </nav>
    )
}
