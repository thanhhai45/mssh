import {createContext, useCallback, useContext, useEffect, useMemo, useState} from 'react'

import {api, errorMessage} from '@/lib/api'

/**
 * Every setting the app understands, with the value used when the database has
 * nothing to say.
 *
 * The store keeps strings — that is all a key-value table can hold — and this
 * table is where they get their meaning. Adding a setting is one line here and
 * nothing at all in Go.
 */
export const SETTING_DEFAULTS = {
    'terminal.fontFamily': 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    'terminal.fontSize': '13',
    'terminal.lineHeight': '1.2',
    'terminal.cursorStyle': 'block',
    'terminal.theme': 'dracula',
} as const

export type SettingKey = keyof typeof SETTING_DEFAULTS

type SettingsContextValue = {
    /** Defaults, with anything stored in the database layered on top. */
    settings: Record<SettingKey, string>
    loading: boolean
    error: string | null
    setSetting: (key: SettingKey, value: string) => Promise<void>
    resetSetting: (key: SettingKey) => Promise<void>
}

const SettingsContext = createContext<SettingsContextValue | undefined>(undefined)

export function SettingsProvider({children}: {children: React.ReactNode}) {
    const [stored, setStored] = useState<Record<string, string>>({})
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        let cancelled = false

        api.getAllSettings()
            .then((all) => {
                if (!cancelled) setStored(all)
            })
            .catch((err) => {
                if (!cancelled) setError(errorMessage(err))
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })

        return () => {
            cancelled = true
        }
    }, [])

    const setSetting = useCallback(async (key: SettingKey, value: string) => {
        // Write through: the database is the record, but the screen should not
        // wait a round trip to show a font size the user just picked.
        setStored((previous) => ({...previous, [key]: value}))
        await api.setSetting(key, value)
    }, [])

    const resetSetting = useCallback(async (key: SettingKey) => {
        setStored((previous) => {
            const next = {...previous}
            delete next[key]
            return next
        })
        await api.deleteSetting(key)
    }, [])

    const settings = useMemo(() => {
        const merged = {...SETTING_DEFAULTS} as Record<SettingKey, string>
        for (const key of Object.keys(SETTING_DEFAULTS) as SettingKey[]) {
            const value = stored[key]
            if (value !== undefined && value !== '') {
                merged[key] = value
            }
        }
        return merged
    }, [stored])

    const value = useMemo(
        () => ({settings, loading, error, setSetting, resetSetting}),
        [settings, loading, error, setSetting, resetSetting],
    )

    return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>
}

export function useSettings() {
    const context = useContext(SettingsContext)
    if (!context) {
        throw new Error('useSettings must be used within a SettingsProvider')
    }
    return context
}
