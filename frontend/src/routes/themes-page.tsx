import {useEffect, useRef, type ReactNode} from 'react'
import type {ITheme} from '@xterm/xterm'
import {Monitor, Moon, Sun} from 'lucide-react'
import {FitAddon} from '@xterm/addon-fit'
import {Terminal} from '@xterm/xterm'

import {useTheme} from '@/components/theme-provider'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {RadioGroup, RadioGroupItem} from '@/components/ui/radio-group'
import {useSettings} from '@/lib/settings-store'
import {AUTO_THEME, TERMINAL_THEME_NAMES, TERMINAL_THEMES, terminalTheme} from '@/lib/terminal-themes'
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
    const {resolvedTheme} = useTheme()
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
            theme: terminalTheme(settings['terminal.theme'], resolvedTheme),
        }
    }, [settings, resolvedTheme])

    return <div ref={containerRef} className="h-full w-full"/>
}

/**
 * Two lines of a terminal drawn with plain spans in a palette's own colours and
 * the chosen font — what a card shows instead of a row of swatches, so each one
 * looks like the terminal it would give. No xterm here: ten live terminals for
 * a picker would be a lot of machinery for two lines of text.
 */
function MiniTerminal({theme, fontFamily, className}: {theme: ITheme; fontFamily: string; className?: string}) {
    const paint = (colour: string | undefined, text: string, bold = false): ReactNode => (
        <span style={{color: colour, fontWeight: bold ? 600 : undefined}}>{text}</span>
    )
    return (
        <div
            // The border is for the light palettes: white on a white card would
            // otherwise have no edge at all.
            className={cn(
                'grid gap-0.5 overflow-hidden rounded border border-border p-2 text-[10px] leading-snug whitespace-pre',
                className,
            )}
            style={{backgroundColor: theme.background, color: theme.foreground, fontFamily}}
        >
            <div>
                {paint(theme.green, 'web-1', true)}:{paint(theme.blue, '~', true)}$ ls
            </div>
            <div>
                {paint(theme.blue, 'src', true)} {paint(theme.green, 'run.sh', true)} {paint(theme.red, 'err.log', true)}
            </div>
            <div>
                {paint(theme.yellow, 'warn')} {paint(theme.magenta, 'v2')} {paint(theme.cyan, 'ok')}{' '}
                <span style={{backgroundColor: theme.cursor, color: theme.background}}> </span>
            </div>
        </div>
    )
}

function ThemeCard({
    selected,
    onSelect,
    label,
    description,
    children,
}: {
    selected: boolean
    onSelect: () => void
    label: string
    description?: string
    children: ReactNode
}) {
    return (
        <button
            type="button"
            onClick={onSelect}
            aria-pressed={selected}
            className={cn(
                // min-w-0: the sample text never wraps, and a grid cell will not
                // shrink below its content unless told it may.
                'flex min-w-0 flex-col gap-2 rounded-lg border p-2 text-left transition',
                selected ? 'border-primary ring-1 ring-primary' : 'hover:border-muted-foreground/40',
            )}
        >
            {children}
            <span className="grid px-1 leading-tight">
                <span className="text-xs font-medium">{label}</span>
                {description && <span className="text-[11px] text-muted-foreground">{description}</span>}
            </span>
        </button>
    )
}

export function ThemesPage() {
    const {theme, setTheme, resolvedTheme} = useTheme()
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
                    <ThemeCard
                        selected={!(settings['terminal.theme'] in TERMINAL_THEMES)}
                        onSelect={() => void setSetting('terminal.theme', AUTO_THEME)}
                        label="Auto"
                        description="GitHub, following the app"
                    >
                        {/* One frame split down the middle, so it reads as one terminal in
                            two moods rather than two cards squeezed together. */}
                        <div className="grid grid-cols-2 overflow-hidden rounded border border-border [&>*]:min-w-0">
                            <MiniTerminal
                                theme={TERMINAL_THEMES['github-light'].theme}
                                fontFamily={settings['terminal.fontFamily']}
                                className="rounded-none border-0"
                            />
                            <MiniTerminal
                                theme={TERMINAL_THEMES['github-dark'].theme}
                                fontFamily={settings['terminal.fontFamily']}
                                className="rounded-none border-0"
                            />
                        </div>
                    </ThemeCard>
                    {TERMINAL_THEME_NAMES.map((name) => (
                        <ThemeCard
                            key={name}
                            selected={settings['terminal.theme'] === name}
                            onSelect={() => void setSetting('terminal.theme', name)}
                            label={TERMINAL_THEMES[name].label}
                        >
                            <MiniTerminal theme={TERMINAL_THEMES[name].theme} fontFamily={settings['terminal.fontFamily']}/>
                        </ThemeCard>
                    ))}
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
                     style={{backgroundColor: terminalTheme(settings['terminal.theme'], resolvedTheme).background}}>
                    <TerminalPreview/>
                </div>
            </section>
        </div>
    )
}
