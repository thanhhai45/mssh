/**
 * Three ids run through this app, and at runtime all three are plain strings:
 *
 *   connectionId  which machine        lives in the database, forever
 *   tabId         which window on screen   lives until the tab is closed
 *   sessionId     which conversation       replaced whenever a tab reconnects
 *
 * Confusing two of them is the single most likely bug in the session code, and
 * `string` lets every one of those mistakes compile. Branding makes the
 * compiler tell them apart while the runtime value stays exactly what Go sent.
 *
 * Only tabId and sessionId are branded. connectionId is deliberately left as a
 * plain string: it comes out of the generated Wails models and off the router's
 * params, and branding it would mean a cast at every one of those boundaries
 * for far less protection. The mistakes that actually happen are "I passed a
 * connection id where a tab or a session was wanted", and those are caught.
 */

declare const brand: unique symbol

type Branded<Name extends string> = string & {readonly [brand]: Name}

/** Identifies one terminal on screen. Invented here; Go never sees one. */
export type TabId = Branded<'TabId'>

/** Identifies one live conversation with a machine. Invented in Go. */
export type SessionId = Branded<'SessionId'>

// Mints a tab id.
export function newTabId(): TabId {
    return crypto.randomUUID() as TabId
}

/**
 * Marks a string coming from Go as a session id.
 *
 * The only sanctioned way to make one, so `grep asSessionId` lists every place
 * an unchecked string crosses into the typed world.
 */
export function asSessionId(raw: string): SessionId {
    return raw as SessionId
}