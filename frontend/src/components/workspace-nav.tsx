import {useState, type SubmitEvent} from 'react'
import {Link, useRouterState} from '@tanstack/react-router'
import {ChevronRight, Plus, Server as ServerIcon} from 'lucide-react'

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
    const {workspaces, addServer} = useWorkspaces()
    const [dialogWorkspaceId, setDialogWorkspaceId] = useState<string | null>(null)
    const [name, setName] = useState('')
    const [command, setCommand] = useState('')

    function resetForm() {
        setName('')
        setCommand('')
    }

    function handleOpenChange(open: boolean) {
        if (!open) {
            setDialogWorkspaceId(null)
            resetForm()
        }
    }

    function handleSubmit(event: SubmitEvent) {
        event.preventDefault()
        if (!dialogWorkspaceId || !name.trim() || !command.trim()) return
        addServer(dialogWorkspaceId, {name: name.trim(), command: command.trim()})
        setDialogWorkspaceId(null)
        resetForm()
    }

    return (
        <SidebarGroup>
            <SidebarGroupLabel>Workspaces</SidebarGroupLabel>
            <SidebarMenu>
                {workspaces.map((workspace) => {
                    const groupPath = `/workspaces/${workspace.id}`
                    const isActiveGroup = pathname.startsWith(groupPath)

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
                                        {workspace.status === 'connected' && (
                                            <span className="size-1.5 shrink-0 rounded-full bg-emerald-500"/>
                                        )}
                                        <ChevronRight className="transition-transform group-data-[state=open]/collapsible:rotate-90"/>
                                    </SidebarMenuButton>
                                </CollapsibleTrigger>
                                <CollapsibleContent>
                                    <SidebarMenuSub>
                                        {workspace.servers.map((server) => {
                                            const to = `${groupPath}/servers/${server.id}`
                                            return (
                                                <SidebarMenuSubItem key={server.id}>
                                                    <SidebarMenuSubButton
                                                        asChild
                                                        isActive={pathname === to}
                                                        className="h-auto py-1.5"
                                                    >
                                                        <Link to={to}>
                                                            <div className="flex min-w-0 flex-col leading-tight">
                                                                <span className="truncate">{server.name}</span>
                                                                <span className="truncate text-xs text-muted-foreground">
                                                                    {server.command}
                                                                </span>
                                                            </div>
                                                        </Link>
                                                    </SidebarMenuSubButton>
                                                </SidebarMenuSubItem>
                                            )
                                        })}
                                        {workspace.servers.length === 0 && (
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
                            <DialogClose asChild>
                                <Button type="button" variant="outline">
                                    Cancel
                                </Button>
                            </DialogClose>
                            <Button type="submit" disabled={!name.trim() || !command.trim()}>
                                Add server
                            </Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>
        </SidebarGroup>
    )
}
