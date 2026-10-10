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
    | 'github-dark'
    | 'github-light'
    | 'iterm2-default'
    | 'vscode-dark-modern'
    | 'dracula'
    | 'nord'
    | 'solarized-dark'
    | 'gruvbox-dark'
    | 'tokyo-night'

/**
 * Not a palette but a choice: GitHub Light while the app is light, GitHub Dark
 * while it is dark. The default, so a terminal never sits black inside a white
 * window, or white inside a black one, unless someone chose that.
 */
export const AUTO_THEME = 'auto'

/*
 * The first four are the palettes people already know from elsewhere, copied
 * from github.com/mbadolato/iTerm2-Color-Schemes (windowsterminal/) rather
 * than typed from memory. Two adjustments, both to selection only:
 *
 *  - GitHub sets selectionBackground to the foreground colour. Windows
 *    Terminal draws selection translucent; xterm.js draws it as given, which
 *    would hide the selected text entirely. Same colour, a quarter opacity.
 *  - iTerm2 selects in pale blue but keeps white text, unreadable in xterm.js;
 *    iTerm2 itself switches selected text to black, so that is set too.
 */
export const TERMINAL_THEMES: Record<TerminalThemeName, {label: string; theme: ITheme}> = {
    'github-dark': {
        label: 'GitHub Dark',
        theme: {
            background: '#0d1117',
            foreground: '#e6edf3',
            cursor: '#2f81f7',
            selectionBackground: '#e6edf340',
            black: '#484f58',
            red: '#ff7b72',
            green: '#3fb950',
            yellow: '#d29922',
            blue: '#58a6ff',
            magenta: '#bc8cff',
            cyan: '#39c5cf',
            white: '#b1bac4',
            brightBlack: '#6e7681',
            brightRed: '#ffa198',
            brightGreen: '#56d364',
            brightYellow: '#e3b341',
            brightBlue: '#79c0ff',
            brightMagenta: '#d2a8ff',
            brightCyan: '#56d4dd',
            brightWhite: '#ffffff',
        },
    },

    'github-light': {
        label: 'GitHub Light',
        theme: {
            background: '#ffffff',
            foreground: '#1f2328',
            cursor: '#0969da',
            selectionBackground: '#1f232840',
            black: '#24292f',
            red: '#cf222e',
            green: '#116329',
            yellow: '#4d2d00',
            blue: '#0969da',
            magenta: '#8250df',
            cyan: '#1b7c83',
            white: '#6e7781',
            brightBlack: '#57606a',
            brightRed: '#a40e26',
            brightGreen: '#1a7f37',
            brightYellow: '#633c01',
            brightBlue: '#218bff',
            brightMagenta: '#a475f9',
            brightCyan: '#3192aa',
            brightWhite: '#8c959f',
        },
    },

    'iterm2-default': {
        label: 'iTerm2 Default',
        theme: {
            background: '#000000',
            foreground: '#ffffff',
            cursor: '#e5e5e5',
            selectionBackground: '#c1deff',
            selectionForeground: '#000000',
            black: '#000000',
            red: '#c91b00',
            green: '#00c200',
            yellow: '#c7c400',
            blue: '#2225c4',
            magenta: '#ca30c7',
            cyan: '#00c5c7',
            white: '#ffffff',
            brightBlack: '#686868',
            brightRed: '#ff6e67',
            brightGreen: '#5ffa68',
            brightYellow: '#fffc67',
            brightBlue: '#6871ff',
            brightMagenta: '#ff77ff',
            brightCyan: '#60fdff',
            brightWhite: '#ffffff',
        },
    },

    'vscode-dark-modern': {
        label: 'VS Code Dark Modern',
        theme: {
            background: '#1f1f1f',
            foreground: '#cccccc',
            cursor: '#ffffff',
            selectionBackground: '#3a3d41',
            black: '#000000',
            red: '#cd3131',
            green: '#0dbc79',
            yellow: '#e5e510',
            blue: '#2472c8',
            magenta: '#bc3fbc',
            cyan: '#11a8cd',
            white: '#e5e5e5',
            brightBlack: '#666666',
            brightRed: '#f14c4c',
            brightGreen: '#23d18b',
            brightYellow: '#f5f543',
            brightBlue: '#3b8eea',
            brightMagenta: '#d670d6',
            brightCyan: '#29b8db',
            brightWhite: '#e5e5e5',
        },
    },

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

/**
 * The palette a setting stands for. `appearance` is the app's light or dark as
 * actually shown (ThemeProvider's resolvedTheme), which 'auto' follows. A name
 * the app does not know — say, from a newer version — behaves as 'auto'.
 */
export function resolveTerminalTheme(name: string, appearance: 'light' | 'dark'): TerminalThemeName {
    if (name in TERMINAL_THEMES) return name as TerminalThemeName
    return appearance === 'dark' ? 'github-dark' : 'github-light'
}

export function terminalTheme(name: string, appearance: 'light' | 'dark'): ITheme {
    return TERMINAL_THEMES[resolveTerminalTheme(name, appearance)].theme
}
