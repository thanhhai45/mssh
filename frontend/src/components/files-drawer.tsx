import {useCallback, useEffect, useState} from 'react'
import {
    ArrowDown,
    ArrowUp,
    CornerLeftUp,
    Download,
    File,
    FileSymlink,
    Folder,
    FolderSymlink,
    RefreshCw,
    Upload,
    X,
} from 'lucide-react'

import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle} from '@/components/ui/sheet'
import {
    api,
    errorMessage,
    kindMeta,
    onTransferChanged,
    usesSSH,
    type RemoteEntry,
    type RemoteListing,
    type Transfer,
} from '@/lib/api'
import {formatBytes} from '@/lib/bytes'
import type {SessionId} from '@/lib/ids'
import {relativeTime} from '@/lib/relative-time'
import {useTransfers} from '@/lib/transfers-store'
import {cn} from '@/lib/utils'

type FilesDrawerProps = {
    open: boolean
    onOpenChange: (open: boolean) => void
    /** Called once the drawer has closed, to hand the keyboard back. */
    onClosed: () => void
    connectionName: string
    kind: string
    /** The active tab's session, or null when that tab is not connected. */
    sessionId: SessionId | null
}

/**
 * The files of the machine a tab is connected to, over that tab's own SSH
 * connection: no second login, no second tunnel.
 */
export function FilesDrawer({open, onOpenChange, onClosed, connectionName, kind, sessionId}: FilesDrawerProps) {
    // The last directory each session listed, so closing the drawer and
    // opening it again comes back to it. Only directories that listed: a typo
    // remembered here would greet the user with an error every time.
    const [paths, setPaths] = useState<Record<string, string>>({})
    const remember = useCallback((sessionId: SessionId, path: string) => {
        setPaths((previous) => (previous[sessionId] === path ? previous : {...previous, [sessionId]: path}))
    }, [])

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent
                className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-lg"
                onCloseAutoFocus={(event) => {
                    // Radix would focus the page body: there is no trigger to
                    // return to when ⌘E opened the drawer.
                    event.preventDefault()
                    onClosed()
                }}
            >
                <SheetHeader className="border-b">
                    <SheetTitle>Files</SheetTitle>
                    <SheetDescription>{connectionName}</SheetDescription>
                </SheetHeader>

                {!usesSSH(kind) ? (
                    <Unavailable>
                        Files travel over an SSH connection. {kindMeta(kind).label} connections run a
                        program instead of holding one open, so there is nothing to browse them over yet.
                    </Unavailable>
                ) : !sessionId ? (
                    <Unavailable>
                        Connect this tab first: files travel over its SSH connection.
                    </Unavailable>
                ) : (
                    <Browser
                        // A new session starts clean: the last one's listing and
                        // errors say nothing about this one.
                        key={sessionId}
                        sessionId={sessionId}
                        // '' is where the session logs in.
                        initialPath={paths[sessionId] ?? ''}
                        remember={remember}
                    />
                )}
            </SheetContent>
        </Sheet>
    )
}

function Unavailable({children}: {children: React.ReactNode}) {
    return <p className="p-4 text-sm text-muted-foreground">{children}</p>
}

/** The directory a remote path sits in. Remote paths are always POSIX. */
function parentOf(remotePath: string): string {
    const slash = remotePath.lastIndexOf('/')
    return slash > 0 ? remotePath.slice(0, slash) : '/'
}

type Loaded = {
    /** Which request this answers, so a slow answer to an old one is not mistaken for the current. */
    key: string
    listing?: RemoteListing
    error?: string
}

function Browser({sessionId, initialPath, remember}: {
    sessionId: SessionId
    initialPath: string
    remember: (sessionId: SessionId, path: string) => void
}) {
    const {transfersOf, cancel, clearFinished} = useTransfers()
    /** The directory asked for. Until it lists, the one on screen is `shown`. */
    const [path, setPath] = useState(initialPath)
    const [reloads, setReloads] = useState(0)
    const [loaded, setLoaded] = useState<Loaded | null>(null)
    // The last directory that loaded. It stays on screen while the next one
    // loads, and when that fails: an error should not leave the user with nothing.
    const [shown, setShown] = useState<RemoteListing | null>(null)
    const [actionError, setActionError] = useState<string | null>(null)

    const key = `${path}\n${reloads}`
    useEffect(() => {
        let cancelled = false
        api.listRemoteDirectory(sessionId, path)
            .then((listing) => {
                if (cancelled) return
                setLoaded({key, listing})
                setShown(listing)
            })
            .catch((err) => {
                if (!cancelled) setLoaded({key, error: errorMessage(err)})
            })
        return () => {
            cancelled = true
        }
    }, [sessionId, path, key])

    const loading = loaded?.key !== key
    const error = !loading ? loaded?.error : undefined

    const currentDir = shown?.path
    useEffect(() => {
        if (currentDir) remember(sessionId, currentDir)
    }, [sessionId, currentDir, remember])

    // An upload finishing into this directory changes what is in it.
    useEffect(() => {
        if (!currentDir) return
        return onTransferChanged((transfer) => {
            if (
                transfer.sessionId === sessionId &&
                transfer.direction === 'upload' &&
                transfer.state === 'done' &&
                parentOf(transfer.destination) === currentDir
            ) {
                setReloads((count) => count + 1)
            }
        })
    }, [sessionId, currentDir])

    async function download(entry: RemoteEntry) {
        setActionError(null)
        try {
            await api.downloadFile(sessionId, entry.path)
        } catch (err) {
            setActionError(errorMessage(err))
        }
    }

    async function upload() {
        if (!shown) return
        setActionError(null)
        try {
            await api.uploadFiles(sessionId, shown.path)
        } catch (err) {
            setActionError(errorMessage(err))
        }
    }

    const transfers = transfersOf(sessionId)
    const notice = actionError ?? error

    return (
        <div className="flex min-h-0 flex-1 flex-col">
            <div className="flex items-center gap-1 border-b p-2">
                <Button
                    variant="ghost"
                    size="icon-sm"
                    title="Up one directory"
                    disabled={!shown?.parent}
                    onClick={() => shown?.parent && setPath(shown.parent)}
                >
                    <CornerLeftUp/>
                </Button>
                <Input
                    // Reset to the directory on screen whenever an answer
                    // arrives - after a bad path too, so the field never names
                    // a directory other than the one Upload would send to. The
                    // error above still says what was typed.
                    key={`${shown?.path}\n${loaded?.key}`}
                    defaultValue={shown?.path ?? ''}
                    placeholder={loading ? 'Loading…' : '/path/to/directory'}
                    aria-label="Path"
                    className="h-7 font-mono text-xs"
                    onKeyDown={(event) => {
                        if (event.key === 'Enter') setPath(event.currentTarget.value.trim())
                    }}
                />
                <Button
                    variant="ghost"
                    size="icon-sm"
                    title="Refresh"
                    onClick={() => {
                        // The directory on screen, not a bad path asked for since.
                        if (shown) setPath(shown.path)
                        setReloads((count) => count + 1)
                    }}
                >
                    <RefreshCw className={cn(loading && 'animate-spin')}/>
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    title={shown ? `Upload to ${shown.path}` : undefined}
                    onClick={() => void upload()}
                    disabled={!shown}
                >
                    <Upload/>
                    Upload
                </Button>
            </div>

            {notice && (
                <div className="flex items-start gap-2 border-b bg-destructive/5 p-3 text-sm text-destructive">
                    <p className="min-w-0 flex-1 whitespace-pre-wrap">{notice}</p>
                    {actionError && (
                        <button
                            type="button"
                            onClick={() => setActionError(null)}
                            className="shrink-0 rounded p-0.5 hover:bg-destructive/10"
                            title="Dismiss"
                        >
                            <X className="size-4"/>
                        </button>
                    )}
                </div>
            )}

            <ul className="min-h-0 flex-1 overflow-y-auto py-1">
                {shown && shown.entries.length === 0 && (
                    <li className="px-4 py-6 text-center text-sm text-muted-foreground">This directory is empty.</li>
                )}
                {shown?.entries.map((entry) => (
                    <EntryRow
                        key={entry.path}
                        entry={entry}
                        onOpen={() => setPath(entry.path)}
                        onDownload={() => void download(entry)}
                    />
                ))}
            </ul>

            {transfers.length > 0 && (
                <div className="flex max-h-[40%] flex-col border-t">
                    <div className="flex items-center justify-between px-4 pt-3 pb-1">
                        <span className="text-xs font-medium text-muted-foreground">Transfers</span>
                        {transfers.some((transfer) => transfer.state !== 'running') && (
                            <Button variant="ghost" size="xs" onClick={() => clearFinished(sessionId)}>
                                Clear finished
                            </Button>
                        )}
                    </div>
                    <ul className="min-h-0 overflow-y-auto px-2 pb-2">
                        {transfers.map((transfer) => (
                            <TransferRow key={transfer.id} transfer={transfer} onCancel={() => cancel(transfer.id)}/>
                        ))}
                    </ul>
                </div>
            )}
        </div>
    )
}

function EntryRow({entry, onOpen, onDownload}: {
    entry: RemoteEntry
    onOpen: () => void
    onDownload: () => void
}) {
    const Icon = entry.isDir
        ? (entry.isLink ? FolderSymlink : Folder)
        : (entry.isLink ? FileSymlink : File)
    const details = `${entry.mode} · ${new Date(entry.modifiedAt * 1000).toLocaleString()}`

    const content = (
        <>
            <Icon className={cn('size-4 shrink-0', entry.isDir ? 'text-sky-500' : 'text-muted-foreground')}/>
            <span className="min-w-0 flex-1 truncate">{entry.name}</span>
            {!entry.isDir && (
                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{formatBytes(entry.size)}</span>
            )}
            <span className="w-16 shrink-0 text-right text-xs text-muted-foreground">{relativeTime(entry.modifiedAt)}</span>
        </>
    )
    const row = 'flex w-full items-center gap-2 px-4 py-1.5 text-left text-sm'

    // A directory opens on a click, the way a link would. A file does nothing
    // on a click - downloading means a dialog, which should never be a surprise -
    // and has its own button, plus a double click for those who expect one.
    if (entry.isDir) {
        return (
            <li>
                <button type="button" className={cn(row, 'hover:bg-accent')} title={details} onClick={onOpen}>
                    {content}
                    {/* Where a file has its Download button, so the columns line up. */}
                    <span className="size-6 shrink-0"/>
                </button>
            </li>
        )
    }
    return (
        <li className={cn(row, 'hover:bg-accent')} title={details} onDoubleClick={onDownload}>
            {content}
            <Button
                variant="ghost"
                size="icon-xs"
                title="Download"
                className="text-muted-foreground hover:text-foreground"
                onClick={onDownload}
            >
                <Download/>
            </Button>
        </li>
    )
}

function TransferRow({transfer, onCancel}: {transfer: Transfer; onCancel: () => void}) {
    const Arrow = transfer.direction === 'download' ? ArrowDown : ArrowUp
    const percent = transfer.total > 0 ? Math.min(100, (transfer.done / transfer.total) * 100) : 0

    let status: string
    switch (transfer.state) {
        case 'running':
            status = transfer.total > 0
                ? `${formatBytes(transfer.done)} of ${formatBytes(transfer.total)}`
                : formatBytes(transfer.done)
            break
        case 'done':
            status = transfer.direction === 'download' ? `Saved to ${transfer.destination}` : 'Uploaded'
            break
        case 'cancelled':
            status = 'Cancelled'
            break
        default:
            status = transfer.message
    }

    return (
        <li className="flex items-start gap-2 rounded-md px-2 py-1.5">
            <Arrow className="mt-0.5 size-4 shrink-0 text-muted-foreground"/>
            <div className="min-w-0 flex-1">
                <div className="truncate text-sm" title={transfer.source}>{transfer.name}</div>
                {transfer.state === 'running' && (
                    <div className="my-1 h-1 overflow-hidden rounded-full bg-muted">
                        <div className="h-full bg-primary transition-[width] duration-100" style={{width: `${percent}%`}}/>
                    </div>
                )}
                <div
                    className={cn(
                        'truncate text-xs',
                        transfer.state === 'failed' ? 'text-destructive' : 'text-muted-foreground',
                    )}
                    title={status}
                >
                    {status}
                </div>
            </div>
            {transfer.state === 'running' && (
                <Button variant="ghost" size="icon-xs" title="Cancel" onClick={onCancel}>
                    <X/>
                </Button>
            )}
        </li>
    )
}
