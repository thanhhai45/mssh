import type {ITheme} from '@xterm/xterm'

/**
 * A terminal never sends a colour. It sends "colour number 4", and the client
 * decides what that looks like. These tables are that decision — sixteen ANSI
 * slots plus the background, foreground and cursor.
 *
 * That is the whole secret of terminal themes: the remote machine has no idea
 * which one you picked.
 */
export type TerminalThemeName =
    | 'dracula'
    | 'nord'
    | 'solarized-dark'
    | 'gruvbox-dark'
    | 'tokyo-night'

export const TERMINAL_THEMES: Record<TerminalThemeName, {label: string; theme: ITheme}> = {
    'dracula': {
        label: 'Dracula',
        theme: {
            background: '#282a36',
            foreground: '#f8f8f2',
            cursor: '#f8f8f2',
            selectionBackground: '#44475a',
            black: '#21222c',
            red: '#ff5555',
            green: '#50fa7b',
            yellow: '#f1fa8c',
            blue: '#bd93f9',
            magenta: '#ff79c6',
            cyan: '#8be9fd',
            white: '#f8f8f2',
            brightBlack: '#6272a4',
            brightRed: '#ff6e6e',
            brightGreen: '#69ff94',
            brightYellow: '#ffffa5',
            brightBlue: '#d6acff',
            brightMagenta: '#ff92df',
            brightCyan: '#a4ffff',
            brightWhite: '#ffffff',
        },
    },

    'nord': {
        label: 'Nord',
        theme: {
            background: '#2e3440',
            foreground: '#d8dee9',
            cursor: '#d8dee9',
            selectionBackground: '#434c5e',
            black: '#3b4252',
            red: '#bf616a',
            green: '#a3be8c',
            yellow: '#ebcb8b',
            blue: '#81a1c1',
            magenta: '#b48ead',
            cyan: '#88c0d0',
            white: '#e5e9f0',
            brightBlack: '#4c566a',
            brightRed: '#bf616a',
            brightGreen: '#a3be8c',
            brightYellow: '#ebcb8b',
            brightBlue: '#81a1c1',
            brightMagenta: '#b48ead',
            brightCyan: '#8fbcbb',
            brightWhite: '#eceff4',
        },
    },

    'solarized-dark': {
        label: 'Solarized Dark',
        theme: {
            background: '#002b36',
            foreground: '#839496',
            cursor: '#93a1a1',
            selectionBackground: '#073642',
            black: '#073642',
            red: '#dc322f',
            green: '#859900',
            yellow: '#b58900',
            blue: '#268bd2',
            magenta: '#d33682',
            cyan: '#2aa198',
            white: '#eee8d5',
            brightBlack: '#586e75',
            brightRed: '#cb4b16',
            brightGreen: '#93a1a1',
            brightYellow: '#657b83',
            brightBlue: '#839496',
            brightMagenta: '#6c71c4',
            brightCyan: '#93a1a1',
            brightWhite: '#fdf6e3',
        },
    },

    'gruvbox-dark': {
        label: 'Gruvbox Dark',
        theme: {
            background: '#282828',
            foreground: '#ebdbb2',
            cursor: '#ebdbb2',
            selectionBackground: '#504945',
            black: '#282828',
            red: '#cc241d',
            green: '#98971a',
            yellow: '#d79921',
            blue: '#458588',
            magenta: '#b16286',
            cyan: '#689d6a',
            white: '#a89984',
            brightBlack: '#928374',
            brightRed: '#fb4934',
            brightGreen: '#b8bb26',
            brightYellow: '#fabd2f',
            brightBlue: '#83a598',
            brightMagenta: '#d3869b',
            brightCyan: '#8ec07c',
            brightWhite: '#ebdbb2',
        },
    },

    'tokyo-night': {
        label: 'Tokyo Night',
        theme: {
            background: '#1a1b26',
            foreground: '#c0caf5',
            cursor: '#c0caf5',
            selectionBackground: '#33467c',
            black: '#15161e',
            red: '#f7768e',
            green: '#9ece6a',
            yellow: '#e0af68',
            blue: '#7aa2f7',
            magenta: '#bb9af7',
            cyan: '#7dcfff',
            white: '#a9b1d6',
            brightBlack: '#414868',
            brightRed: '#f7768e',
            brightGreen: '#9ece6a',
            brightYellow: '#e0af68',
            brightBlue: '#7aa2f7',
            brightMagenta: '#bb9af7',
            brightCyan: '#7dcfff',
            brightWhite: '#c0caf5',
        },
    },
}

export const TERMINAL_THEME_NAMES = Object.keys(TERMINAL_THEMES) as TerminalThemeName[]

/** Falls back to Dracula so a name the app does not know still renders. */
export function terminalTheme(name: string): ITheme {
    return (TERMINAL_THEMES[name as TerminalThemeName] ?? TERMINAL_THEMES.dracula).theme
}
