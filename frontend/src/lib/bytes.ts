const UNITS = ['B', 'KB', 'MB', 'GB', 'TB']

/**
 * Turns a size in bytes into "812 B", "4.2 MB" or "1.1 GB".
 *
 * Powers of 1024, as Finder did for years and ls -h still does; one decimal
 * below 10 of a unit and none above, so a list of sizes stays easy to scan.
 */
export function formatBytes(bytes: number): string {
    let value = bytes
    let unit = 0
    while (value >= 1024 && unit < UNITS.length - 1) {
        value /= 1024
        unit++
    }
    if (unit === 0) return `${bytes} B`
    return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${UNITS[unit]}`
}
