import {createContext, useContext, useEffect, useState} from 'react'

type Theme = 'dark' | 'light' | 'system'

type ThemeProviderProps = {
    children: React.ReactNode
    defaultTheme?: Theme
    storageKey?: string
}

type ThemeProviderState = {
    theme: Theme
    /**
     * What is actually on screen. Differs from `theme` only for 'system', where
     * it follows the OS — including when the OS switches while mssh is open.
     * Things that must match the app, such as the 'auto' terminal theme, read
     * this rather than `theme`.
     */
    resolvedTheme: 'light' | 'dark'
    setTheme: (theme: Theme) => void
}

const darkQuery = '(prefers-color-scheme: dark)'

const initialState: ThemeProviderState = {
    theme: 'system',
    resolvedTheme: 'light',
    setTheme: () => null,
}

const ThemeProviderContext = createContext<ThemeProviderState>(initialState)

export function ThemeProvider({
    children,
    defaultTheme = 'system',
    storageKey = 'mssh-ui-theme',
    ...props
}: ThemeProviderProps) {
    const [theme, setTheme] = useState<Theme>(
        () => (localStorage.getItem(storageKey) as Theme) || defaultTheme,
    )
    const [systemDark, setSystemDark] = useState(
        () => window.matchMedia(darkQuery).matches,
    )

    // The OS can switch on its own — macOS does at sunset with Auto
    // appearance. Before this, 'system' only looked once, at startup.
    useEffect(() => {
        const query = window.matchMedia(darkQuery)
        const follow = (event: MediaQueryListEvent) => setSystemDark(event.matches)
        query.addEventListener('change', follow)
        return () => query.removeEventListener('change', follow)
    }, [])

    const resolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

    useEffect(() => {
        const root = window.document.documentElement
        root.classList.remove('light', 'dark')
        root.classList.add(resolvedTheme)
    }, [resolvedTheme])

    const value = {
        theme,
        resolvedTheme,
        setTheme: (theme: Theme) => {
            localStorage.setItem(storageKey, theme)
            setTheme(theme)
        },
    }

    return (
        <ThemeProviderContext.Provider {...props} value={value}>
            {children}
        </ThemeProviderContext.Provider>
    )
}

export const useTheme = () => {
    const context = useContext(ThemeProviderContext)

    if (context === undefined) {
        throw new Error('useTheme must be used within a ThemeProvider')
    }

    return context
}
