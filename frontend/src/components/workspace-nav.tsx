import {useState} from 'react'
import {Link, useNavigate, useRouterState} from '@tanstack/react-router'
import {ChevronRight, Copy, MoreHorizontal, Pencil, Plus, Server as ServerIcon, Trash2} from 'lucide-react'

import {ConnectionDialog} from '@/components/connection-dialog'
import {WorkspaceDialog} from '@/components/workspace-dialog'
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
    SidebarGroup,
    SidebarGroupLabel,
    SidebarMenu,
    SidebarMenuAction,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarMenuSub,
    SidebarMenuSubButton,
    SidebarMenuSubItem,
} from '@/components/ui/sidebar'
import {
    describeConnection,
    errorMessage,
    toConnectionInput,
    type Connection,
    type Workspace,
} from '@/lib/api'
import {swatchClass} from '@/lib/colors'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

/** What the confirmation dialog is currently asking about. */
type PendingDelete =
    | {type: 'workspace'; workspace: Workspace}
    | {type: 'connection'; connection: Connection}

export function WorkspaceNav() {
    const pathname = useRouterState({select: (s) => s.location.pathname})
    const navigate = useNavigate()
    const {workspaces, connections, createConnection, deleteWorkspace, deleteConnection} =
        useWorkspaces()

    // Both dialogs are mounted only while open, so they never hold stale state.
    const [workspaceDialog, setWorkspaceDialog] = useState<Workspace | null | undefined>(undefined)
    const [connectionDialog, setConnectionDialog] = useState<{
        workspace: Workspace
        connection: Connection | null
    } | null>(null)

    const [pending, setPending] = useState<PendingDelete | null>(null)
    const [deleteBusy, setDeleteBusy] = useState(false)
    const [deleteError, setDeleteError] = useState<string | null>(null)

    async function duplicateConnection(workspace: Workspace, connection: Connection) {
        try {
            await createConnection(workspace.id, {
                ...toConnectionInput(connection),
                name: `${connection.name} copy`,
            })
        } catch (err) {
            // Nothing is open to show this in, so surface it the blunt way.
            window.alert(errorMessage(err))
        }
    }

    async function handleDelete() {
        if (!pending) return

        setDeleteBusy(true)
        setDeleteError(null)
        try {
            // Read this before deleting: afterwards the route points at nothing.
            const viewedPath =
                pending.type === 'workspace'
                    ? `/workspaces/${pending.workspace.id}`
                    : `/workspaces/${pending.connection.workspaceId}/servers/${pending.connection.id}`
            const wasViewing = pathname.startsWith(viewedPath)

            if (pending.type === 'workspace') {
                await deleteWorkspace(pending.workspace.id)
            } else {
                await deleteConnection(pending.connection.id)
            }

            if (wasViewing) {
                await navigate({to: '/'})
            }
            setPending(null)
        } catch (err) {
            setDeleteError(errorMessage(err))
        } finally {
            setDeleteBusy(false)
        }
    }

    const pendingCount =
        pending?.type === 'workspace' ? (connections[pending.workspace.id] ?? []).length : 0

    return (
        <SidebarGroup>
            <SidebarGroupLabel>Workspaces</SidebarGroupLabel>
            <SidebarMenu>
                {workspaces.map((workspace) => {
                    const groupPath = `/workspaces/${workspace.id}`
                    const isActiveGroup = pathname.startsWith(groupPath)
                    const list = connections[workspace.id] ?? []

                    return (
                        <Collapsible
                            key={workspace.id}
                            defaultOpen={isActiveGroup}
                            className="group/collapsible"
                        >
                            <SidebarMenuItem>
                                <CollapsibleTrigger asChild>
                                    <SidebarMenuButton tooltip={workspace.name}>
                                        <ServerIcon/>
                                        <span className="flex-1 truncate">{workspace.name}</span>
                                        <span
                                            className={cn(
                                                'size-2 shrink-0 rounded-full',
                                                swatchClass(workspace.color),
                                            )}
                                        />
                                        <ChevronRight className="transition-transform group-data-[state=open]/collapsible:rotate-90"/>
                                    </SidebarMenuButton>
                                </CollapsibleTrigger>

                                <DropdownMenu>
                                    <DropdownMenuTrigger asChild>
                                        <SidebarMenuAction showOnHover>
                                            <MoreHorizontal/>
                                            <span className="sr-only">Workspace actions</span>
                                        </SidebarMenuAction>
                                    </DropdownMenuTrigger>
                                    <DropdownMenuContent side="right" align="start" className="w-44">
                                        <DropdownMenuItem
                                            onClick={() => setConnectionDialog({workspace, connection: null})}
                                        >
                                            <Plus/>
                                            <span>New connection</span>
                                        </DropdownMenuItem>
                                        <DropdownMenuItem onClick={() => setWorkspaceDialog(workspace)}>
                                            <Pencil/>
                                            <span>Edit workspace</span>
                                        </DropdownMenuItem>
                                        <DropdownMenuSeparator/>
                                        <DropdownMenuItem
                                            variant="destructive"
                                            onClick={() => {
                                                setDeleteError(null)
                                                setPending({type: 'workspace', workspace})
                                            }}
                                        >
                                            <Trash2/>
                                            <span>Delete workspace</span>
                                        </DropdownMenuItem>
                                    </DropdownMenuContent>
                                </DropdownMenu>

                                <CollapsibleContent>
                                    <SidebarMenuSub>
                                        {list.map((connection) => {
                                            const to = `${groupPath}/servers/${connection.id}`
                                            return (
                                                <SidebarMenuSubItem
                                                    key={connection.id}
                                                    className="group/conn relative"
                                                >
                                                    <SidebarMenuSubButton
                                                        asChild
                                                        isActive={pathname === to}
                                                        className="h-auto py-1.5 pr-8"
                                                    >
                                                        <Link to={to}>
                                                            <div className="flex min-w-0 flex-col leading-tight">
                                                                <span className="truncate">
                                                                    {connection.name}
                                                                </span>
                                                                <span className="truncate text-xs text-muted-foreground">
                                                                    {describeConnection(connection)}
                                                                </span>
                                                            </div>
                                                        </Link>
                                                    </SidebarMenuSubButton>

                                                    <DropdownMenu>
                                                        <DropdownMenuTrigger asChild>
                                                            <button
                                                                type="button"
                                                                className="absolute right-1 top-2 flex size-6 items-center justify-center rounded-md text-muted-foreground opacity-0 transition hover:bg-sidebar-accent focus-visible:opacity-100 group-hover/conn:opacity-100"
                                                            >
                                                                <MoreHorizontal className="size-3.5"/>
                                                                <span className="sr-only">
                                                                    Connection actions
                                                                </span>
                                                            </button>
                                                        </DropdownMenuTrigger>
                                                        <DropdownMenuContent
                                                            side="right"
                                                            align="start"
                                                            className="w-40"
                                                        >
                                                            <DropdownMenuItem
                                                                onClick={() =>
                                                                    setConnectionDialog({workspace, connection})
                                                                }
                                                            >
                                                                <Pencil/>
                                                                <span>Edit</span>
                                                            </DropdownMenuItem>
                                                            <DropdownMenuItem
                                                                onClick={() =>
                                                                    duplicateConnection(workspace, connection)
                                                                }
                                                            >
                                                                <Copy/>
                                                                <span>Duplicate</span>
                                                            </DropdownMenuItem>
                                                            <DropdownMenuSeparator/>
                                                            <DropdownMenuItem
                                                                variant="destructive"
                                                                onClick={() => {
                                                                    setDeleteError(null)
                                                                    setPending({type: 'connection', connection})
                                                                }}
                                                            >
                                                                <Trash2/>
                                                                <span>Delete</span>
                                                            </DropdownMenuItem>
                                                        </DropdownMenuContent>
                                                    </DropdownMenu>
                                                </SidebarMenuSubItem>
                                            )
                                        })}

                                        {list.length === 0 && (
                                            <SidebarMenuSubItem>
                                                <span className="block px-2 py-1.5 text-xs text-muted-foreground">
                                                    No connections yet
                                                </span>
                                            </SidebarMenuSubItem>
                                        )}

                                        <SidebarMenuSubItem>
                                            <button
                                                type="button"
                                                onClick={() =>
                                                    setConnectionDialog({workspace, connection: null})
                                                }
                                                className="flex h-7 w-full min-w-0 -translate-x-px items-center gap-2 rounded-md px-2 text-xs text-muted-foreground outline-hidden hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                                            >
                                                <Plus className="size-3.5 shrink-0"/>
                                                <span>New connection</span>
                                            </button>
                                        </SidebarMenuSubItem>
                                    </SidebarMenuSub>
                                </CollapsibleContent>
                            </SidebarMenuItem>
                        </Collapsible>
                    )
                })}

                <SidebarMenuItem>
                    <SidebarMenuButton
                        onClick={() => setWorkspaceDialog(null)}
                        tooltip="New workspace"
                        className="text-muted-foreground"
                    >
                        <Plus/>
                        <span>New workspace</span>
                    </SidebarMenuButton>
                </SidebarMenuItem>
            </SidebarMenu>

            {workspaceDialog !== undefined && (
                <WorkspaceDialog
                    workspace={workspaceDialog}
                    onClose={() => setWorkspaceDialog(undefined)}
                />
            )}

            {connectionDialog && (
                <ConnectionDialog
                    workspace={connectionDialog.workspace}
                    connection={connectionDialog.connection}
                    onClose={() => setConnectionDialog(null)}
                />
            )}

            <AlertDialog
                open={pending !== null}
                onOpenChange={(open) => !open && setPending(null)}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>
                            {pending?.type === 'workspace'
                                ? `Delete “${pending.workspace.name}”?`
                                : `Delete “${pending?.connection.name}”?`}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                            {pending?.type === 'workspace'
                                ? pendingCount === 0
                                    ? 'This workspace has no connections. '
                                    : `Its ${pendingCount} connection${pendingCount === 1 ? '' : 's'} will be deleted too, along with any saved passwords. `
                                : 'Any password saved for it will be removed from your keychain too. '}
                            This cannot be undone.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    {deleteError && <p className="text-sm text-destructive">{deleteError}</p>}
                    <AlertDialogFooter>
                        <AlertDialogCancel disabled={deleteBusy}>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                            disabled={deleteBusy}
                            onClick={(event) => {
                                // Keep the dialog open while the delete runs, so an
                                // error has somewhere to appear.
                                event.preventDefault()
                                void handleDelete()
                            }}
                        >
                            {deleteBusy ? 'Deleting…' : 'Delete'}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </SidebarGroup>
    )
}