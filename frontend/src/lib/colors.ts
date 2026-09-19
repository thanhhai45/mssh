export const WORKSPACE_COLORS = [
    'slate', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'violet',
] as const

export type WorkspaceColor = (typeof WORKSPACE_COLORS)[number]

/**
 * Every class is written out in full. Tailwind scans the source for complete
 * class strings, so a template like `bg-${color}-500` would never ship.
 *
 * That constraint is why this file is a set of lookup tables rather than one
 * clever function: each role a colour plays in the UI needs its own table.
 */
const SWATCH: Record<WorkspaceColor, string> = {
    slate: 'bg-slate-500',
    red: 'bg-red-500',
    orange: 'bg-orange-500',
    amber: 'bg-amber-500',
    green: 'bg-green-500',
    teal: 'bg-teal-500',
    blue: 'bg-blue-500',
    violet: 'bg-violet-500',
}

/** Text in the workspace colour, readable on both the light and dark ground. */
const TEXT: Record<WorkspaceColor, string> = {
    slate: 'text-slate-600 dark:text-slate-400',
    red: 'text-red-600 dark:text-red-400',
    orange: 'text-orange-600 dark:text-orange-400',
    amber: 'text-amber-600 dark:text-amber-400',
    green: 'text-green-600 dark:text-green-400',
    teal: 'text-teal-600 dark:text-teal-400',
    blue: 'text-blue-600 dark:text-blue-400',
    violet: 'text-violet-600 dark:text-violet-400',
}

/**
 * The stripe down the left of a workspace in the sidebar.
 *
 * It only ever appears on an element that already has `border-l-2`, so this
 * table sets the colour and nothing else.
 */
const EDGE: Record<WorkspaceColor, string> = {
    slate: 'border-l-slate-500',
    red: 'border-l-red-500',
    orange: 'border-l-orange-500',
    amber: 'border-l-amber-500',
    green: 'border-l-green-500',
    teal: 'border-l-teal-500',
    blue: 'border-l-blue-500',
    violet: 'border-l-violet-500',
}

/**
 * A wash of colour behind a card.
 *
 * Deliberately faint — /8 rather than /100. The colour is there to tell two
 * workspaces apart at a glance, not to be looked at, and anything stronger
 * fights the text sitting on top of it.
 */
const TINT: Record<WorkspaceColor, string> = {
    slate: 'from-slate-500/8',
    red: 'from-red-500/8',
    orange: 'from-orange-500/8',
    amber: 'from-amber-500/8',
    green: 'from-green-500/8',
    teal: 'from-teal-500/8',
    blue: 'from-blue-500/8',
    violet: 'from-violet-500/8',
}

export function isWorkspaceColor(value: string): value is WorkspaceColor {
    return (WORKSPACE_COLORS as readonly string[]).includes(value)
}

/** Falls back to slate so a colour the database does not know about still renders. */
function lookUp(table: Record<WorkspaceColor, string>, color: string): string {
    return isWorkspaceColor(color) ? table[color] : table.slate
}

export function swatchClass(color: string): string {
    return lookUp(SWATCH, color)
}

export function accentTextClass(color: string): string {
    return lookUp(TEXT, color)
}

export function accentEdgeClass(color: string): string {
    return lookUp(EDGE, color)
}

export function accentTintClass(color: string): string {
    return lookUp(TINT, color)
}
