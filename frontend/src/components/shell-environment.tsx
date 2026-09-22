import {useCallback, useEffect, useState} from 'react'
import {AlertTriangle, RefreshCw, TerminalSquare} from 'lucide-react'

import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {api, errorMessage, type ShellEnvironmentReport} from '@/lib/api'
import {cn} from '@/lib/utils'

/**
 * What mssh took from the user's login shell.
 *
 * An app launched from Finder gets none of the exports in ~/.zshrc, so mssh
 * runs the login shell at startup and adopts its environment. Doing that
 * silently would make behaviour depend on state nobody can see; this is the
 * other half of the bargain — it happens automatically, and it says what it
 * did.
 *
 * Values of anything that looks like a secret are dropped on the Go side, so
 * they never reach this component at all.
 */
export function ShellEnvironment() {
    const [report, setReport] = useState<ShellEnvironmentReport | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [busy, setBusy] = useState(false)
    const [showEverything, setShowEverything] = useState(false)

    const load = useCallback(async (reload: boolean) => {
        setBusy(true)
        setError(null)
        try {
            setReport(reload
                ? await api.reloadShellEnvironment()
                : await api.getShellEnvironment())
        } catch (err) {
            setError(errorMessage(err))
        } finally {
            setBusy(false)
        }
    }, [])

    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        void load(false)
    }, [load])

    const variables = report?.variables ?? []
    const shown = showEverything ? variables : variables.filter((row) => row.interesting)
    const hiddenCount = variables.length - shown.length

    return (
        <section className="grid gap-3">
            <div className="flex items-center justify-between">
                <Label>Environment</Label>
                <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => void load(true)}
                >
                    <RefreshCw className={cn(busy && 'animate-spin')}/>
                    {busy ? 'Reading…' : 'Reload'}
                </Button>
            </div>

            <p className="text-sm text-muted-foreground">
                mssh runs your login shell at startup and takes its environment, so
                anything exported in <code>~/.zshrc</code> reaches <code>ssh</code> and{' '}
                <code>aws</code> the same way it would in a terminal. Values that look
                like secrets are never sent to this screen.
            </p>

            {error && (
                <p className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
                    {error}
                </p>
            )}

            {report && (
                <div className="grid gap-3 rounded-lg border p-3">
                    <div className="flex flex-wrap items-center gap-2 text-sm">
                        <TerminalSquare className="size-4 shrink-0 text-muted-foreground"/>
                        <code className="truncate">{report.shell}</code>
                        <span className="text-muted-foreground">
                            · {report.durationMs} ms · {variables.length} variables
                        </span>
                    </div>

                    {report.error && (
                        <div className="flex gap-2 rounded-md border border-amber-500/40 bg-amber-500/5 p-3 text-sm">
                            <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-500"/>
                            <div>
                                <p className="font-medium">
                                    Your login shell could not be read
                                </p>
                                <p className="text-muted-foreground">{report.error}</p>
                                <p className="mt-1 text-muted-foreground">
                                    mssh fell back to guessing a few common directories,
                                    so connections may still work — but exports from your
                                    shell profile will not be there.
                                </p>
                            </div>
                        </div>
                    )}

                    {shown.length > 0 && (
                        <dl className="grid gap-1 text-xs">
                            {shown.map((row) => (
                                <div
                                    key={row.name}
                                    className="grid grid-cols-[minmax(0,14rem)_1fr] items-baseline gap-3"
                                >
                                    <dt className="truncate font-medium">{row.name}</dt>
                                    {/* Capped and scrollable: PATH alone runs to
                                        forty entries, and left to itself it buries
                                        the AWS variables this screen exists to
                                        show. Nothing is hidden, only folded. */}
                                    <dd className="max-h-16 min-w-0 overflow-y-auto break-all font-mono text-muted-foreground">
                                        {row.masked ? (
                                            <span
                                                className="italic"
                                                title="Withheld: this name looks like a secret"
                                            >
                                                •••••••• set, not shown
                                            </span>
                                        ) : (
                                            row.value
                                        )}
                                    </dd>
                                </div>
                            ))}
                        </dl>
                    )}

                    {variables.length > 0 && (
                        <Button
                            variant="ghost"
                            size="sm"
                            className="justify-self-start"
                            onClick={() => setShowEverything((wasShown) => !wasShown)}
                        >
                            {showEverything
                                ? 'Show only what affects connections'
                                : `Show all ${variables.length} variables`}
                        </Button>
                    )}

                    {!showEverything && hiddenCount > 0 && shown.length === 0 && (
                        <p className="text-sm text-muted-foreground">
                            None of the variables mssh uses for connections are set.
                        </p>
                    )}
                </div>
            )}
        </section>
    )
}
