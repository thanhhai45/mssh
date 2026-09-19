import {Outlet} from '@tanstack/react-router'

import {AppSidebar} from '@/components/app-sidebar'
import {CommandPalette} from '@/components/command-palette'
import {HeaderBreadcrumb} from '@/components/header-breadcrumb'
import {SidebarInset, SidebarProvider, SidebarTrigger} from '@/components/ui/sidebar'

export function RootLayout() {
    return (
        <SidebarProvider>
            <AppSidebar/>
            <SidebarInset>
                <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
                    <SidebarTrigger className="-ml-1"/>
                    <HeaderBreadcrumb/>
                    <kbd className="ml-auto hidden shrink-0 rounded border bg-muted px-1.5 py-0.5 font-sans text-[11px] text-muted-foreground sm:inline">
                        ⌘P
                    </kbd>
                </header>
                <div className="flex flex-1 flex-col gap-4 p-6">
                    <Outlet/>
                </div>
            </SidebarInset>

            {/* Mounted once, at the top: the palette is reachable from every
                page, so it cannot belong to any one of them. */}
            <CommandPalette/>
        </SidebarProvider>
    )
}
