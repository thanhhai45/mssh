import {useState, type SubmitEvent} from 'react'
import {Link, useRouterState} from '@tanstack/react-router'
import {ChevronRight, Plus, Server as ServerIcon} from 'lucide-react'
import {api, describeConnection, errorMessage} from '@/lib/api'

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
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {
    SidebarGroup,
    SidebarGroupLabel,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarMenuSub,
    SidebarMenuSubButton,
    SidebarMenuSubItem,
} from '@/components/ui/sidebar'
import {useWorkspaces} from '@/lib/workspaces-store'

export function WorkspaceNav() {
    const pathname = useRouterState({select: (s) => s.location.pathname})
    const {workspaces, connections, createConnection} = useWorkspaces()
    const [dialogWorkspaceId, setDialogWorkspaceId] = useState<string | null>(null)
    const [name, setName] = useState('')
    const [command, setCommand] = useState('')
    const [saving, setSaving] = useState(false)
    const [formError, setFormError] = useState<string | null>(null)

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

    async function handleSubmit(event: SubmitEvent) {
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
                                        <span className="shrink-0 text-xs text-muted-foreground">
                                            {(connections[workspace.id] ?? []).length}
                                        </span>
                                        <ChevronRight className="transition-transform group-data-[state=open]/collapsible:rotate-90"/>
                                    </SidebarMenuButton>
                                </CollapsibleTrigger>
                                <CollapsibleContent>
                                    <SidebarMenuSub>
                                        {list.map((connection: any) => {
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
                                                                <span className="truncate">{connection.name}</span>
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
        </SidebarGroup>
    )
}
