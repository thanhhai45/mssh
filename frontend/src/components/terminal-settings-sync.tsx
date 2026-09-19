import {useEffect} from 'react'

import {useSettings} from '@/lib/settings-store'
import {applyTerminalSettings} from '@/lib/terminal-session'
import {terminalTheme} from '@/lib/terminal-themes'

/**
 * Pushes settings into the terminal store whenever they change.
 *
 * It renders nothing. Terminals live outside React, so something has to carry
 * changes across that line, and a component is the only thing that can watch
 * React state. Keeping it separate leaves SettingsProvider unaware that
 * terminals exist at all.
 */
export function TerminalSettingsSync() {
    const {settings} = useSettings()

    useEffect(() => {
        const theme = terminalTheme(settings['terminal.theme'])

        applyTerminalSettings({
            fontFamily: settings['terminal.fontFamily'],
            fontSize: Number(settings['terminal.fontSize']),
            lineHeight: Number(settings['terminal.lineHeight']),
            cursorStyle: settings['terminal.cursorStyle'] as 'block' | 'underline' | 'bar',
            theme,
        })

        document.documentElement.style.setProperty(
            '--terminal-background',
            theme.background ?? '#000000',
        )
    }, [settings])

    return null
}
