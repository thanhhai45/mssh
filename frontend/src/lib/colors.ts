export const WORKSPACE_COLORS = [
    'slate', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'violet',
] as const

export type WorkspaceColor = (typeof WORKSPACE_COLORS)[number]

/**
 * Every class is written out in full. Tailwind scans the source for complete
 * class strings, so a template like `bg-${color}-500` would never ship.
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

export function isWorkspaceColor(value: string): value is WorkspaceColor {
    return (WORKSPACE_COLORS as readonly string[]).includes(value)
}

/** Falls back to slate so a colour the database does not know about still renders. */
export function swatchClass(color: string): string {
    return isWorkspaceColor(color) ? SWATCH[color] : SWATCH.slate
}