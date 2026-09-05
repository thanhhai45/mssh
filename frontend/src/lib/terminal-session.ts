import {FitAddon} from '@xterm/addon-fit'
import {Terminal} from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

import {api, onSessionOutput} from '@/lib/api'

/**
 * Terminals live here, at module scope, outside React.
 *
 * A terminal holds state React cannot rebuild: scrollback, cursor position, a
 * vim session halfway through. So React must not own its lifecycle. Components
 * borrow the DOM node while they are mounted and hand it back afterwards; the
 * terminal itself only dies when the session does.
 */
type TerminalEntry = {
    node: HTMLDivElement
    terminal: Terminal
    fit: FitAddon
    /** Removes the Wails output listener. */
    stopListening: () => void
    /** xterm can only measure itself once it is in the document. */
    opened: boolean
}

const entries = new Map<string, TerminalEntry>()

// Hardcoded for now; buổi 13 moves these into settings.
const TERMINAL_OPTIONS = {
    convertEol: false,
    fontSize: 13,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    cursorBlink: true,
    scrollback: 10_000,
    theme: {
        background: '#09090b',
        foreground: '#e4e4e7',
        cursor: '#e4e4e7',
    },
} as const

function create(connectionId: string): TerminalEntry {
    const node = document.createElement('div')
    node.style.width = '100%'
    node.style.height = '100%'

    const terminal = new Terminal(TERMINAL_OPTIONS)
    const fit = new FitAddon()
    terminal.loadAddon(fit)

    // Keystrokes go straight to Go. Typing into a session that is not open
    // fails, and that is fine: there is nothing useful to say about it.
    terminal.onData((data) => {
        void api.writeToSession(connectionId, data).catch(() => {})
    })

    const stopListening = onSessionOutput(connectionId, (chunk: any) => {
        terminal.write(chunk)
    })

    const entry: TerminalEntry = {node, terminal, fit, stopListening, opened: false}
    entries.set(connectionId, entry)
    return entry
}

/** Puts the terminal into a container, creating it on first use. */
export function attachTerminal(connectionId: string, container: HTMLElement): void {
    const entry = entries.get(connectionId) ?? create(connectionId)

    container.appendChild(entry.node)

    // open() measures the node, so it can only run once the node is in the
    // document. That is why creation and opening are separate steps.
    if (!entry.opened) {
        entry.terminal.open(entry.node)
        entry.opened = true
    }

    resizeTerminal(connectionId)
    entry.terminal.focus()
}

/**
 * Takes the terminal out of the page without destroying it.
 *
 * This is the whole point of the module: removeChild is lifting a sheet of
 * paper off the desk, dispose() is tearing it up.
 */
export function detachTerminal(connectionId: string): void {
    const entry = entries.get(connectionId)
    if (!entry) return
    entry.node.remove()
}

/** Really destroys a terminal. Call it when the session ends for good. */
export function disposeTerminal(connectionId: string): void {
    const entry = entries.get(connectionId)
    if (!entry) return

    entry.stopListening()
    entry.node.remove()
    entry.terminal.dispose()
    entries.delete(connectionId)
}

/** Refits the terminal to its container and tells the remote shell the new size. */
export function resizeTerminal(connectionId: string): void {
    const entry = entries.get(connectionId)
    if (!entry || !entry.opened) return

    // A detached or zero-sized container makes fit() throw or produce nonsense.
    if (!entry.node.isConnected || entry.node.clientWidth === 0) return

    entry.fit.fit()
    void api
        .resizeSession(connectionId, entry.terminal.cols, entry.terminal.rows)
        .catch(() => {})
}

/** Writes a local line, without sending anything to the remote machine. */
export function writeToTerminal(connectionId: string, text: string): void {
    const entry = entries.get(connectionId) ?? create(connectionId)
    entry.terminal.write(text)
}

export function hasTerminal(connectionId: string): boolean {
    return entries.has(connectionId)
}

/** Terminal geometry, for the first ConnectSession call. */
export function terminalSize(connectionId: string): {cols: number; rows: number} {
    const entry = entries.get(connectionId)
    if (!entry || !entry.opened) return {cols: 80, rows: 24}
    return {cols: entry.terminal.cols, rows: entry.terminal.rows}
}

/**
* Clears the screen and scrollback without destroying the terminal, so a
* reconnect starts clean while the node stays attached to the page.
*/
export function resetTerminal(connectionId: string): void {
  const entry = entries.get(connectionId)
  if (!entry) return
  entry.terminal.reset()
}