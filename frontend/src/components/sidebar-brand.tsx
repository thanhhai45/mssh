import {TerminalSquare} from 'lucide-react'

export function SidebarBrand() {
    return (
        <div className="flex h-12 items-center gap-2 px-2 group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:p-0">
            <div className="flex aspect-square size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                <TerminalSquare className="size-4"/>
            </div>
            <div className="grid flex-1 text-left text-sm leading-tight group-data-[collapsible=icon]:hidden">
                <span className="truncate font-semibold">mssh</span>
                <span className="truncate text-xs text-muted-foreground">SSH workspace manager</span>
            </div>
        </div>
    )
}
