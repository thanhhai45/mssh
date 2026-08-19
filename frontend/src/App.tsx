import {RouterProvider} from '@tanstack/react-router'

import {ThemeProvider} from '@/components/theme-provider'
import {TooltipProvider} from '@/components/ui/tooltip'
import {WorkspacesProvider} from '@/lib/workspaces-store'
import {router} from '@/router'

function App() {
    return (
        <ThemeProvider defaultTheme="system" storageKey="mssh-ui-theme">
            <TooltipProvider delayDuration={200}>
                <WorkspacesProvider>
                    <RouterProvider router={router}/>
                </WorkspacesProvider>
            </TooltipProvider>
        </ThemeProvider>
    )
}

export default App
