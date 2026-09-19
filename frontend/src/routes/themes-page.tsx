import {useEffect, useRef} from 'react'
import {Monitor, Moon, Sun} from 'lucide-react'
import {FitAddon} from '@xterm/addon-fit'
import {Terminal} from '@xterm/xterm'

import {useTheme} from '@/components/theme-provider'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {RadioGroup, RadioGroupItem} from '@/components/ui/radio-group'
import {useSettings} from '@/lib/settings-store'
import {TERMINAL_THEME_NAMES, TERMINAL_THEMES, terminalTheme} from '@/lib/terminal-themes'
import {cn} from '@/lib/utils'

const appThemes = [
    {value: 'light', label: 'Light', icon: Sun},
    {value: 'dark', label: 'Dark', icon: Moon},
    {value: 'system', label: 'System', icon: Monitor},
] as const

const cursorStyles = [
    {value: 'block', label: 'Block'},
    {value: 'underline', label: 'Underline'},
    {value: 'bar', label: 'Bar'},
] as const

const PREVIEW_LINES = [
    '\x1b[1;32mdeploy@web-1\x1b[0m:\x1b[1;34m~\x1b[0m$ ls --color',
    '\x1b[1;34mconfig\x1b[0m  \x1b[1;32mdeploy.sh\x1b[0m  README.md  \x1b[1;31merror.log\x1b[0m',
    '\x1b[1;32mdeploy@web-1\x1b[0m:\x1b[1;34m~\x1b[0m$ systemctl status nginx',
    '  \x1b[32m●\x1b[0m nginx.service — A high performance web server',
    '     Active: \x1b[1;32mactive (running)\x1b[0m since Mon 09:14:03 UTC',
    '\x1b[1;32mdeploy@web-1\x1b[0m:\x1b[1;34m~\x1b[0m$ \x1b[7m \x1b[0m',
]

/** A throwaway terminal that shows the settings without touching a session. */
function TerminalPreview() {
    const {settings} = useSettings()
    const containerRef = useRef<HTMLDivElement>(null)
    const terminalRef = useRef<Terminal | null>(null)

    useEffect(() => {
        const container = containerRef.current
        if (!container) return

        const terminal = new Terminal({
            convertEol: true,
            disableStdin: true,
            scrollback: 0,
            rows: PREVIEW_LINES.length,
        })
        const fit = new FitAddon()
        terminal.loadAddon(fit)
        terminal.open(container)

        PREVIEW_LINES.forEach((line) => terminal.writeln(line))

        terminalRef.current = terminal

        return () => {
            terminalRef.current = null
            terminal.dispose()
        }
    }, [])

    // Same idea as the real terminals: change the options, do not rebuild.
    useEffect(() => {
        const terminal = terminalRef.current
        if (!terminal) return

        terminal.options = {
            fontFamily: settings['terminal.fontFamily'],
            fontSize: Number(settings['terminal.fontSize']),
            lineHeight: Number(settings['terminal.lineHeight']),
            cursorStyle: settings['terminal.cursorStyle'] as 'block' | 'underline' | 'bar',
            theme: terminalTheme(settings['terminal.theme']),
        }
    }, [settings])

    return <div ref={containerRef} className="h-full w-full"/>
}

export function ThemesPage() {
    const {theme, setTheme} = useTheme()
    const {settings, setSetting, resetSetting, loading} = useSettings()

    if (loading) {
        return <p className="text-sm text-muted-foreground">Loading…</p>
    }

    return (
        <div className="flex max-w-3xl flex-col gap-8">
            <div>
                <h1 className="text-2xl font-semibold tracking-tight">Appearance</h1>
                <p className="text-sm text-muted-foreground">
                    The app theme is remembered per device. Everything else is stored
                    with your connections.
                </p>
            </div>

            {/* ---- App theme ---- */}
            <section className="grid gap-3">
                <Label>App theme</Label>
                <RadioGroup
                    value={theme}
                    onValueChange={(value) => setTheme(value as typeof theme)}
                    className="grid max-w-md gap-3 sm:grid-cols-3"
                >
                    {appThemes.map(({value, label, icon: Icon}) => (
                        <Label
                            key={value}
                            htmlFor={`theme-${value}`}
                            className="flex cursor-pointer flex-col items-center gap-3 rounded-lg border p-4 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-accent"
                        >
                            <Icon className="size-5"/>
                            <span className="text-sm font-medium">{label}</span>
                            <RadioGroupItem value={value} id={`theme-${value}`} className="sr-only"/>
                        </Label>
                    ))}
                </RadioGroup>
            </section>

            {/* ---- Terminal colours ---- */}
            <section className="grid gap-3">
                <Label>Terminal colours</Label>
                <div className="grid gap-2 sm:grid-cols-3 lg:grid-cols-5">
                    {TERMINAL_THEME_NAMES.map((name) => {
                        const preset = TERMINAL_THEMES[name]
                        const selected = settings['terminal.theme'] === name
                        return (
                            <button
                                key={name}
                                type="button"
                                onClick={() => void setSetting('terminal.theme', name)}
                                aria-pressed={selected}
                                className={cn(
                                    'flex flex-col gap-2 rounded-lg border p-3 text-left transition',
                                    selected ? 'border-primary' : 'hover:border-muted-foreground/40',
                                )}
                            >
                                <div
                                    className="flex h-8 items-end gap-1 rounded p-1"
                                    style={{backgroundColor: preset.theme.background}}
                                >
                                    {[
                                        preset.theme.red,
                                        preset.theme.green,
                                        preset.theme.yellow,
                                        preset.theme.blue,
                                        preset.theme.magenta,
                                        preset.theme.cyan,
                                    ].map((colour) => (
                                        <span
                                            key={colour}
                                            className="h-4 flex-1 rounded-sm"
                                            style={{backgroundColor: colour}}
                                        />
                                    ))}
                                </div>
                                <span className="text-xs font-medium">{preset.label}</span>
                            </button>
                        )
                    })}
                </div>
            </section>

            {/* ---- Type ---- */}
            <section className="grid gap-4">
                <Label>Type</Label>

                <div className="grid gap-3 sm:grid-cols-2">
                    <div className="grid gap-2">
                        <Label htmlFor="font-size" className="font-normal text-muted-foreground">
                            Size
                        </Label>
                        <Input
                            id="font-size"
                            type="number"
                            min={8}
                            max={32}
                            value={settings['terminal.fontSize']}
                            onChange={(event) =>
                                void setSetting('terminal.fontSize', event.target.value)
                            }
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="line-height" className="font-normal text-muted-foreground">
                            Line height
                        </Label>
                        <Input
                            id="line-height"
                            type="number"
                            min={1}
                            max={2}
                            step={0.1}
                            value={settings['terminal.lineHeight']}
                            onChange={(event) =>
                                void setSetting('terminal.lineHeight', event.target.value)
                            }
                        />
                    </div>
                </div>

                <div className="grid gap-2">
                    <Label htmlFor="font-family" className="font-normal text-muted-foreground">
                        Font
                    </Label>
                    <Input
                        id="font-family"
                        value={settings['terminal.fontFamily']}
                        onChange={(event) =>
                            void setSetting('terminal.fontFamily', event.target.value)
                        }
                        placeholder="Menlo, monospace"
                    />
                    <p className="text-xs text-muted-foreground">
                        A CSS font stack. Keep a generic <code>monospace</code> at the end so
                        the terminal still lines up if the first font is missing.
                    </p>
                </div>

                <div className="grid gap-2">
                    <Label className="font-normal text-muted-foreground">Cursor</Label>
                    <RadioGroup
                        value={settings['terminal.cursorStyle']}
                        onValueChange={(value) => void setSetting('terminal.cursorStyle', value)}
                        className="flex gap-4"
                    >
                        {cursorStyles.map(({value, label}) => (
                            <div key={value} className="flex items-center gap-2">
                                <RadioGroupItem value={value} id={`cursor-${value}`}/>
                                <Label htmlFor={`cursor-${value}`} className="cursor-pointer font-normal">
                                    {label}
                                </Label>
                            </div>
                        ))}
                    </RadioGroup>
                </div>
            </section>

            {/* ---- Preview ---- */}
            <section className="grid gap-3">
                <div className="flex items-center justify-between">
                    <Label>Preview</Label>
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                            void resetSetting('terminal.fontFamily')
                            void resetSetting('terminal.fontSize')
                            void resetSetting('terminal.lineHeight')
                            void resetSetting('terminal.cursorStyle')
                            void resetSetting('terminal.theme')
                        }}
                    >
                        Reset to defaults
                    </Button>
                </div>
                <div className="h-48 overflow-hidden rounded-lg border p-3"
                     style={{backgroundColor: terminalTheme(settings['terminal.theme']).background}}>
                    <TerminalPreview/>
                </div>
            </section>
        </div>
    )
}
