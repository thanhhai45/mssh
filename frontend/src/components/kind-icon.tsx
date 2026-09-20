import {Cloud, FileKey, KeyRound, Waypoints, type LucideIcon} from 'lucide-react'

import {isConnectionKind, type ConnectionKind} from '@/lib/api'

/**
 * One icon per connection kind.
 *
 * Each picture answers the question the kinds actually differ on — *what gets
 * you in* — rather than decorating them:
 *
 * - ssh          a key you hold
 * - ssm          a cloud, and no key at all
 * - ssm-ssh      a path through the cloud to your own account
 * - ssh-config   a file that already knows all of this
 */
const KIND_ICONS: Record<ConnectionKind, LucideIcon> = {
    'ssh': KeyRound,
    'ssm': Cloud,
    'ssm-ssh': Waypoints,
    'ssh-config': FileKey,
}

/**
 * The icon for a connection kind.
 *
 * A component taking the kind, rather than a function returning a component.
 * The lookup is stable either way, but `const Icon = kindIcon(k)` inside a
 * render body reads to both React's lint rules and to a person like a
 * component being built on the fly — and one that really was would silently
 * lose its state on every render. Keeping the lookup inside removes the
 * question.
 */
export function KindIcon({kind, className}: {kind: string; className?: string}) {
    const Icon = isConnectionKind(kind) ? KIND_ICONS[kind] : KIND_ICONS.ssh
    return <Icon className={className}/>
}
