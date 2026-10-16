/**
 * The app's keyboard shortcuts, and the one place that knows which system it
 * is running on.
 *
 * macOS keeps ⌘+letter. Everywhere else it is Ctrl+Shift+letter, as in GNOME
 * Terminal, Windows Terminal and VS Code's terminal: Ctrl+letter alone belongs
 * to the shell (Ctrl+W deletes a word, Ctrl+E jumps to the end of the line,
 * Ctrl+K cuts to it, Ctrl+P is the previous command), and a terminal app has no
 * business taking it.
 */

/**
 * Read once, when the module loads. The user agent rather than Wails'
 * Environment(): that answers with a Promise, and neither a keydown handler nor
 * the first render can wait for one.
 */
export const isMac = navigator.userAgent.includes('Mac')

const letters = {
    find: 'f',
    files: 'e',
    newTab: 't',
    closeTab: 'w',
    clear: 'k',
    palette: 'p',
    sidebar: 'b',
    copy: 'c',
    paste: 'v',
} as const

export type Shortcut = keyof typeof letters

/** Whether this key press is the shortcut, on this system. */
export function isShortcut(event: KeyboardEvent, shortcut: Shortcut): boolean {
    // With Shift held, event.key is the capital letter.
    if (event.key.toLowerCase() !== letters[shortcut]) return false
    if (isMac) return event.metaKey
    return event.ctrlKey && event.shiftKey && !event.altKey && !event.metaKey
}

/** The shortcut as a person reads it: ⌘F on a Mac, Ctrl+Shift+F elsewhere. */
export function label(shortcut: Shortcut): string {
    const letter = letters[shortcut].toUpperCase()
    return isMac ? `⌘${letter}` : `Ctrl+Shift+${letter}`
}
