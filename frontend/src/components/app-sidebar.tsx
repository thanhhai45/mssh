import * as React from 'react'
import {Link, useRouterState} from '@tanstack/react-router'
import {Palette} from 'lucide-react'

import {SidebarBrand} from '@/components/sidebar-brand'
import {WorkspaceNav} from '@/components/workspace-nav'
import {
    Sidebar,
    SidebarContent,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarRail,
} from '@/components/ui/sidebar'

export function AppSidebar(props: React.ComponentProps<typeof Sidebar>) {
    const pathname = useRouterState({select: (s) => s.location.pathname})

    return (
        <Sidebar collapsible="icon" {...props}>
            <SidebarHeader>
                <SidebarBrand/>
            </SidebarHeader>
            <SidebarContent>
                <WorkspaceNav/>
                <SidebarGroup>
                    <SidebarGroupLabel>General</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    asChild
                                    isActive={pathname === '/themes'}
                                    tooltip="Themes"
                                >
                                    <Link to="/themes">
                                        <Palette/>
                                        <span>Themes</span>
                                    </Link>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>
            </SidebarContent>
            <SidebarRail/>
        </Sidebar>
    )
}
