import {Cloud, FileKey, KeyRound, Waypoints, type LucideIcon} from 'lucide-react'

import {isConnectionKind, type ConnectionKind} from '@/lib/api'

/**
 * One icon per connection kind.
 *
 * Each picture answers the question the kind actually differs on — *what gets
 * you in* — rather than decorating it:
 *
 * - ssh          a key you hold
 * - ssm          a cloud, and no key at all
 * - ssm-ssh      a path through the cloud to your own account
 * - ssh-config   a file that already knows all of this
 *
 * It lives apart from KIND_META because that table is plain data crossing the
 * Go boundary, and these are React components.
 */
const KIND_ICONS: Record<ConnectionKind, LucideIcon> = {
    'ssh': KeyRound,
    'ssm': Cloud,
    'ssm-ssh': Waypoints,
    'ssh-config': FileKey,
}

/** Falls back to the ssh icon, the same way kindMeta falls back to its row. */
export function kindIcon(kind: string): LucideIcon {
    return isConnectionKind(kind) ? KIND_ICONS[kind] : KIND_ICONS.ssh
}
