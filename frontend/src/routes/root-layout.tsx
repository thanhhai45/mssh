import {Outlet} from '@tanstack/react-router'

import {AppSidebar} from '@/components/app-sidebar'
import {Separator} from '@/components/ui/separator'
import {SidebarInset, SidebarProvider, SidebarTrigger} from '@/components/ui/sidebar'

export function RootLayout() {
    return (
        <SidebarProvider>
            <AppSidebar/>
            <SidebarInset>
                <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
                    <SidebarTrigger className="-ml-1"/>
                    <Separator orientation="vertical" className="mr-2 h-4"/>
                </header>
                <div className="flex flex-1 flex-col gap-4 p-6">
                    <Outlet/>
                </div>
            </SidebarInset>
        </SidebarProvider>
    )
}
