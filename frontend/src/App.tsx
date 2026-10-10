import {RouterProvider} from '@tanstack/react-router'

import {ThemeProvider} from '@/components/theme-provider'
import {TerminalSettingsSync} from '@/components/terminal-settings-sync'
import {TooltipProvider} from '@/components/ui/tooltip'
import { SessionStatusProvider } from '@/lib/session-status-store'
import { SettingsProvider } from '@/lib/settings-store'
import {WorkspacesProvider} from '@/lib/workspaces-store'
import {TabsProvider} from '@/lib/tabs-store'
import {TransfersProvider} from '@/lib/transfers-store'
import {router} from '@/router'

function App() {
    return (
        <ThemeProvider defaultTheme="system" storageKey="mssh-ui-theme">
            <TooltipProvider delayDuration={200}>
                <SettingsProvider>
                    <TerminalSettingsSync/>
                    <WorkspacesProvider>
                        <SessionStatusProvider>
                            <TabsProvider>
                                {/* Above the router: a transfer outlives the page that started it. */}
                                <TransfersProvider>
                                    <RouterProvider router={router}/>
                                </TransfersProvider>
                            </TabsProvider>
                        </SessionStatusProvider>
                    </WorkspacesProvider>
                </SettingsProvider>
            </TooltipProvider>
        </ThemeProvider>
    )
}

export default App
