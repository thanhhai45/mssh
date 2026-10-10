import {useState} from 'react'
import {ShieldQuestion} from 'lucide-react'

import {Button} from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {errorMessage, type HostKeyPrompt} from '@/lib/api'

/**
 * The question ssh asks on first contact — "are you sure you want to continue
 * connecting?" — asked here, because for an ssm-ssh instance there is no way
 * to run ssh against it outside mssh.
 *
 * It never sees the key: only the fingerprint to compare, and a yes. Go
 * records exactly the key it was shown.
 */
export function HostKeyDialog({
    prompt,
    onTrust,
    onCancel,
}: {
    prompt: HostKeyPrompt
    /** Records the key and connects again. Rejects if either fails. */
    onTrust: () => Promise<void>
    onCancel: () => void
}) {
    const [trusting, setTrusting] = useState(false)
    const [error, setError] = useState<string | null>(null)

    async function trust() {
        setTrusting(true)
        setError(null)
        try {
            await onTrust()
        } catch (err) {
            setError(errorMessage(err))
            setTrusting(false)
        }
    }

    return (
        <Dialog open onOpenChange={(open) => !open && onCancel()}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <ShieldQuestion className="size-5 shrink-0"/>
                        First connection to {prompt.host}
                    </DialogTitle>
                    <DialogDescription>
                        This machine has never connected to {prompt.host}, so there is nothing
                        to check its identity against yet. It presented this key:
                    </DialogDescription>
                </DialogHeader>

                <div className="grid gap-3 py-2 text-sm">
                    <div className="grid gap-1 rounded-md bg-muted p-3">
                        <span className="text-xs text-muted-foreground">{prompt.keyType}</span>
                        <code className="break-all text-sm">{prompt.fingerprint}</code>
                    </div>
                    <p className="text-muted-foreground">
                        If you can, compare it with the server&apos;s own before trusting it. On
                        the server, <code>ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub</code>{' '}
                        prints it — for an EC2 instance, open it once with the Session Manager
                        kind and run that there.
                    </p>
                    <p className="text-xs text-muted-foreground">
                        Trusting it records the key in <code>{prompt.knownHostsPath}</code>, the
                        file the ssh command uses too. If the key ever changes, mssh will refuse
                        to connect rather than ask again.
                    </p>
                </div>

                <DialogFooter>
                    {error && <p className="mr-auto text-sm text-destructive">{error}</p>}
                    <Button type="button" variant="outline" onClick={onCancel} disabled={trusting}>
                        Cancel
                    </Button>
                    <Button type="button" onClick={trust} disabled={trusting}>
                        {trusting ? 'Connecting…' : 'Trust and connect'}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
