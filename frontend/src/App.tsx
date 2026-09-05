import {RouterProvider} from '@tanstack/react-router'

import {ThemeProvider} from '@/components/theme-provider'
import {TooltipProvider} from '@/components/ui/tooltip'
import { SessionStatusProvider } from './lib/session-status-store'
import {WorkspacesProvider} from '@/lib/workspaces-store'
import {router} from '@/router'

function App() {
    return (
        <ThemeProvider defaultTheme="system" storageKey="mssh-ui-theme">
            <TooltipProvider delayDuration={200}>
                <WorkspacesProvider>
                    <SessionStatusProvider>
                        <RouterProvider router={router}/>
                    </SessionStatusProvider>
                </WorkspacesProvider>
            </TooltipProvider>
        </ThemeProvider>
    )
}

export default App
