import {useState} from 'react'
import {Link, useNavigate, useRouterState} from '@tanstack/react-router'
import {ChevronRight, MoreHorizontal, Pencil, Plus, Server as ServerIcon, Trash2} from 'lucide-react'

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
import {Button} from '@/components/ui/button'
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible'
import {
    Dialog,
    DialogClose,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
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
import {api, describeConnection, errorMessage, type Workspace} from '@/lib/api'
import {swatchClass} from '@/lib/colors'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

export function WorkspaceNav() {
    const pathname = useRouterState({select: (s) => s.location.pathname})
    const navigate = useNavigate()
    const {workspaces, connections, createConnection, deleteWorkspace} = useWorkspaces()

    // "New server" dialog: holds the workspace the new connection belongs to.
    const [dialogWorkspaceId, setDialogWorkspaceId] = useState<string | null>(null)
    const [name, setName] = useState('')
    const [command, setCommand] = useState('')
    const [saving, setSaving] = useState(false)
    const [formError, setFormError] = useState<string | null>(null)

    // Workspace dialog: mounted only while open, so it never has stale state.
    const [editing, setEditing] = useState<Workspace | null>(null)
    const [workspaceDialogOpen, setWorkspaceDialogOpen] = useState(false)

    // Delete confirmation.
    const [deleting, setDeleting] = useState<Workspace | null>(null)
    const [deleteBusy, setDeleteBusy] = useState(false)
    const [deleteError, setDeleteError] = useState<string | null>(null)

    function openCreateWorkspace() {
        setEditing(null)
        setWorkspaceDialogOpen(true)
    }

    function openEditWorkspace(workspace: Workspace) {
        setEditing(workspace)
        setWorkspaceDialogOpen(true)
    }

    function resetForm() {
        setName('')
        setCommand('')
        setFormError(null)
    }

    function handleOpenChange(open: boolean) {
        if (!open) {
            setDialogWorkspaceId(null)
            resetForm()
        }
    }

    async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault()
        if (!dialogWorkspaceId) return

        setSaving(true)
        setFormError(null)
        try {
            const parsed = await api.parseSSHCommand(command)
            await createConnection(dialogWorkspaceId, {
                name: name.trim(),
                kind: 'ssh',
                target: parsed.host,
                port: parsed.port,
                username: parsed.username,
                authMethod: parsed.keyPath ? 'key' : 'agent',
                keyPath: parsed.keyPath,
                awsProfile: '',
                awsRegion: '',
                extra: '',
                color: '',
            })
            setDialogWorkspaceId(null)
            resetForm()
        } catch (err) {
            setFormError(errorMessage(err))
        } finally {
            setSaving(false)
        }
    }

    async function handleDelete() {
        if (!deleting) return

        setDeleteBusy(true)
        setDeleteError(null)
        try {
            // Read this before the delete: afterwards the workspace is gone and
            // the route would point at nothing.
            const wasViewing = pathname.startsWith(`/workspaces/${deleting.id}`)

            await deleteWorkspace(deleting.id)

            if (wasViewing) {
                await navigate({to: '/'})
            }
            setDeleting(null)
        } catch (err) {
            setDeleteError(errorMessage(err))
        } finally {
            setDeleteBusy(false)
        }
    }

    const deletingCount = deleting ? (connections[deleting.id] ?? []).length : 0

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
                                        <DropdownMenuItem onClick={() => openEditWorkspace(workspace)}>
                                            <Pencil/>
                                            <span>Edit</span>
                                        </DropdownMenuItem>
                                        <DropdownMenuSeparator/>
                                        <DropdownMenuItem
                                            variant="destructive"
                                            onClick={() => {
                                                setDeleteError(null)
                                                setDeleting(workspace)
                                            }}
                                        >
                                            <Trash2/>
                                            <span>Delete</span>
                                        </DropdownMenuItem>
                                    </DropdownMenuContent>
                                </DropdownMenu>

                                <CollapsibleContent>
                                    <SidebarMenuSub>
                                        {list.map((connection) => {
                                            const to = `${groupPath}/servers/${connection.id}`
                                            return (
                                                <SidebarMenuSubItem key={connection.id}>
                                                    <SidebarMenuSubButton
                                                        asChild
                                                        isActive={pathname === to}
                                                        className="h-auto py-1.5"
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
                                                </SidebarMenuSubItem>
                                            )
                                        })}

                                        {list.length === 0 && (
                                            <SidebarMenuSubItem>
                                                <span className="block px-2 py-1.5 text-xs text-muted-foreground">
                                                    No servers yet
                                                </span>
                                            </SidebarMenuSubItem>
                                        )}

                                        <SidebarMenuSubItem>
                                            <button
                                                type="button"
                                                onClick={() => setDialogWorkspaceId(workspace.id)}
                                                className="flex h-7 w-full min-w-0 -translate-x-px items-center gap-2 rounded-md px-2 text-xs text-muted-foreground outline-hidden hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                                            >
                                                <Plus className="size-3.5 shrink-0"/>
                                                <span>New server</span>
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
                        onClick={openCreateWorkspace}
                        tooltip="New workspace"
                        className="text-muted-foreground"
                    >
                        <Plus/>
                        <span>New workspace</span>
                    </SidebarMenuButton>
                </SidebarMenuItem>
            </SidebarMenu>

            <Dialog open={dialogWorkspaceId !== null} onOpenChange={handleOpenChange}>
                <DialogContent>
                    <form onSubmit={handleSubmit}>
                        <DialogHeader>
                            <DialogTitle>New server</DialogTitle>
                            <DialogDescription>
                                Add a server you connect to over SSH within this workspace.
                            </DialogDescription>
                        </DialogHeader>
                        <div className="grid gap-4 py-4">
                            <div className="grid gap-2">
                                <Label htmlFor="server-name">Name</Label>
                                <Input
                                    id="server-name"
                                    value={name}
                                    onChange={(event) => setName(event.target.value)}
                                    placeholder="web-1"
                                    autoFocus
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="server-command">SSH command</Label>
                                <Input
                                    id="server-command"
                                    value={command}
                                    onChange={(event) => setCommand(event.target.value)}
                                    placeholder="ssh user@10.0.0.10 -p 22"
                                />
                            </div>
                        </div>
                        <DialogFooter>
                            {formError && (
                                <p className="mr-auto text-sm text-destructive">{formError}</p>
                            )}
                            <DialogClose asChild>
                                <Button type="button" variant="outline">Cancel</Button>
                            </DialogClose>
                            <Button type="submit" disabled={saving || !name.trim() || !command.trim()}>
                                {saving ? 'Adding…' : 'Add server'}
                            </Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>

            {workspaceDialogOpen && (
                <WorkspaceDialog
                    workspace={editing}
                    onClose={() => setWorkspaceDialogOpen(false)}
                />
            )}

            <AlertDialog
                open={deleting !== null}
                onOpenChange={(open) => !open && setDeleting(null)}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Delete “{deleting?.name}”?</AlertDialogTitle>
                        <AlertDialogDescription>
                            {deletingCount === 0
                                ? 'This workspace has no connections.'
                                : `Its ${deletingCount} connection${deletingCount === 1 ? '' : 's'} will be deleted too.`}
                            {' '}This cannot be undone.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    {deleteError && (
                        <p className="text-sm text-destructive">{deleteError}</p>
                    )}
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