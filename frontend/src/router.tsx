import {createRootRoute, createRoute, createRouter} from '@tanstack/react-router'

import {RootLayout} from '@/routes/root-layout'
import {ServerTerminalPage} from '@/routes/server-terminal-page'
import {ThemesPage} from '@/routes/themes-page'
import {WorkspacesPage} from '@/routes/workspaces-page'

const rootRoute = createRootRoute({
    component: RootLayout,
})

const workspacesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: WorkspacesPage,
})

const serverTerminalRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspaces/$workspaceId/servers/$serverId',
    component: ServerTerminalPage,
})

const themesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/themes',
    component: ThemesPage,
})

const routeTree = rootRoute.addChildren([workspacesRoute, serverTerminalRoute, themesRoute])

export const router = createRouter({routeTree})

declare module '@tanstack/react-router' {
    interface Register {
        router: typeof router
    }
}
