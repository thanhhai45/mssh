const MINUTE = 60
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/**
 * Turns a unix timestamp in seconds into "3h ago".
 *
 * Go stores seconds; JavaScript's Date works in milliseconds. Mixing the two up
 * gives answers that are wrong by a factor of a thousand and still look
 * plausible, which is why the conversion happens here, once, and nowhere else.
 */
export function relativeTime(unixSeconds: number): string {
    // 0 is the "never used" marker the database writes, not 1970.
    if (!unixSeconds) return 'never'

    const seconds = Math.floor(Date.now() / 1000) - unixSeconds
    if (seconds < MINUTE) return 'just now'
    if (seconds < HOUR) return `${Math.floor(seconds / MINUTE)}m ago`
    if (seconds < DAY) return `${Math.floor(seconds / HOUR)}h ago`
    if (seconds < 30 * DAY) return `${Math.floor(seconds / DAY)}d ago`

    return new Date(unixSeconds * 1000).toLocaleDateString()
}
