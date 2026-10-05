import {FitAddon} from '@xterm/addon-fit'
import {SearchAddon, type ISearchResultChangeEvent} from '@xterm/addon-search'
import {Terminal, type ITerminalOptions} from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

import {api, onSessionOutput} from '@/lib/api'
import type {SessionId, TabId} from '@/lib/ids'

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
    search: SearchAddon
    sessionId: SessionId | null
    /** Removes the Wails output listener. */
    stopListening: (() => void) | null
    /** xterm can only measure itself once it is in the document. */
    opened: boolean
}

// Keyed by tab id, not by session id. A terminal belongs to the tab on screen
// and outlives any one session in it - which is what keeps the scrollback
// readbale after a session ends, and what lets a reconnect reuse the tab.
const entries = new Map<TabId, TerminalEntry>()

let terminalOptions: ITerminalOptions = {
  // Required by the search addon's highlighting: it marks matches with
  // registerDecoration, which xterm still classes as proposed API and refuses
  // to run without this. Without it every findNext call throws, and the find
  // bar reports "No matches" for text plainly on the screen.
  allowProposedApi: true,
  convertEol: false,
  fontSize: 13,
  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
  lineHeight: 1.2,
  cursorStyle: 'block',
  cursorBlink: true,
  scrollback: 10_000,
}

export function applyTerminalSettings(options: ITerminalOptions): void {
  terminalOptions = {...terminalOptions, ...options}

  for (const [tabId, entry] of entries) {
    entry.terminal.options = options
    resizeTerminal(tabId)
  }
}

function create(tabId: TabId): TerminalEntry {
    const node = document.createElement('div')
    node.style.width = '100%'
    node.style.height = '100%'

    const terminal = new Terminal(terminalOptions)
    const fit = new FitAddon()
    terminal.loadAddon(fit)

    // One search addon per terminal, not one per find bar: the matches and the
    // current position belong to the scrollback, which outlives every
    // component that might want to look at it.
    const search = new SearchAddon()
    terminal.loadAddon(search)

    // Keystrokes go straight to Go. The session id is read from the entry
    // on every keystroke rather than captured here: after a reconnect this
    // tab has a different session id, and captured id would keep typing
    // into the old session.
    terminal.onData((data) => {
        const sessionId = entries.get(tabId)?.sessionId
        if (!sessionId) return
        void api.writeToSession(sessionId, data).catch(() => {})
    })

    terminal.attachCustomKeyEventHandler((event) => {
      if (event.type !== 'keydown') return true
      
      if (event.metaKey && event.key === 'k') {
        clearTerminal(tabId)
        return false
      }
      return true
    })

    const entry: TerminalEntry = {
        node, terminal, fit, search,
        sessionId: null,
        stopListening: null,
        opened: false,
    }

    entries.set(tabId, entry)
    return entry
}

/**
 * Points a tab at a session: output from that session starts arriving in this
 * terminal, and keystrokes start going to it.
 *
 * Replacing one session with another is allowed and is what a reconnect does.
 * The old subscription is torn down first, or both would write here at once.
 */
export function attachSession(tabId: TabId, sessionId: SessionId): void {
    const entry = entries.get(tabId) ?? create(tabId)

    entry.stopListening?.()
    entry.sessionId = sessionId
    entry.stopListening = onSessionOutput(sessionId, (chunk) => {
        entry.terminal.write(chunk)
    })
}

/**
 * Unhooks the session but keeps the terminal and its scrollback.
 *
 * This is the whole reason the entries are keyed by tab: after a session ends,
 * what is on screen is often the only clue about why.
 */
export function detachSession(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry) return

    entry.stopListening?.()
    entry.stopListening = null
    entry.sessionId = null
}

/** The session a tab is currently showing, if any. */
export function sessionOfTab(tabId: TabId): SessionId | null {
    return entries.get(tabId)?.sessionId ?? null
}

/**
 * Writes a local line into whichever terminal is showing that session.
 *
 * The status store knows about sessions but not about tabs, and it should not
 * have to: "say it where the user is looking" is this module's business.
 */
export function writeToSessionTerminal(sessionId: SessionId, text: string): void {
    for (const entry of entries.values()) {
        if (entry.sessionId === sessionId) {
            entry.terminal.write(text)
            return
        }
    }
}

/** Puts the terminal into a container, creating it on first use. */
export function attachTerminal(tabId: TabId, container: HTMLElement): void {
    const entry = entries.get(tabId) ?? create(tabId)

    container.appendChild(entry.node)

    // open() measures the node, so it can only run once the node is in the
    // document. That is why creation and opening are separate steps.
    if (!entry.opened) {
        entry.terminal.open(entry.node)
        entry.opened = true
    }

    resizeTerminal(tabId)
    entry.terminal.focus()
}

/**
 * Takes the terminal out of the page without destroying it.
 *
 * This is the whole point of the module: removeChild is lifting a sheet of
 * paper off the desk, dispose() is tearing it up.
 */
export function detachTerminal(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry) return
    entry.node.remove()
}

/** Really destroys a terminal. Call it when the session ends for good. */
export function disposeTerminal(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry) return

    entry.stopListening?.()
    entry.node.remove()
    entry.terminal.dispose()
    entries.delete(tabId)
}

/** Refits the terminal to its container and tells the remote shell the new size. */
export function resizeTerminal(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry || !entry.opened) return

    // A detached or zero-sized container makes fit() throw or produce nonsense.
    if (!entry.node.isConnected || entry.node.clientWidth === 0) return

    entry.fit.fit()

    if (!entry.sessionId) return

    void api
        .resizeSession(entry.sessionId, entry.terminal.cols, entry.terminal.rows)
        .catch(() => {})
}

/** Writes a local line, without sending anything to the remote machine. */
export function writeToTerminal(tabId: TabId, text: string): void {
    const entry = entries.get(tabId) ?? create(tabId)
    entry.terminal.write(text)
}

export function hasTerminal(tabId: TabId): boolean {
    return entries.has(tabId)
}

/** Terminal geometry, for the first ConnectSession call. */
export function terminalSize(tabId: TabId): {cols: number; rows: number} {
    const entry = entries.get(tabId)
    if (!entry || !entry.opened) return {cols: 80, rows: 24}
    return {cols: entry.terminal.cols, rows: entry.terminal.rows}
}

/**
* Clears the screen and scrollback without destroying the terminal, so a
* reconnect starts clean while the node stays attached to the page.
*/
export function resetTerminal(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry) return
    entry.terminal.reset()
}

export function clearTerminal(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry) return
    entry.terminal.clear()
}

export function focusTerminal(tabId: TabId): void {
    entries.get(tabId)?.terminal.focus()
}

/**
 * Colour for search highlights
 * Fixed on purpose rather than take from the terminal theme: a highlights has
 * to stand out against the theme, not agree with it. The two overview-ruler
 * fields are the only ones the addon's types make required - its way of saying
 * that a match you cannot spot on the scrollbar is half a search
 */

const SEARCH_DECORATIONS = {
    matchBackground: '#3b4a63',
    matchOverviewRuler: '#3b4a63',
    activeMatchBackground: '#8a6d00',
    activeMatchColorOverviewRuler: '#e3b341',
}

export type TerminalSearchRequest = {
    term: string
    direction: 'next' | 'previous'
    incremental: boolean
}

export function findInTerminal(tabId: TabId, request: TerminalSearchRequest): void {
    const entry = entries.get(tabId)
    if(!entry) return
  
    if (request.term === '') {
        entry.search.clearDecorations()
        return
    }
    
    const options = {
        decorations: SEARCH_DECORATIONS,
        incremental: request.incremental,
    }
    
    if (request.direction === 'next') {
        entry.search.findNext(request.term, options)
    } else {
        entry.search.findPrevious(request.term, options)
    }
}

/** Removes the highlights. The scrollback is not youch */
export function clearTerminalSearch(tabId: TabId): void {
    const entry = entries.get(tabId)
    if (!entry) return
    entry.search.clearDecorations()
}

export function onTerminalSearchResults(
    tabId: TabId,
    handler: (results: ISearchResultChangeEvent) => void,
): () => void {
    const entry = entries.get(tabId) ?? create(tabId)
    const subscription = entry.search.onDidChangeResults(handler)
    return () => subscription.dispose()
}
