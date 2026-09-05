import {useState} from 'react'
import {Eye, EyeOff} from 'lucide-react'

import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'

export function PasswordDialog({
    connectionName,
    hint,
    onSubmit,
    onCancel,
}: {
    connectionName: string
    /** Shown above the field, e.g. why the previous attempt failed. */
    hint?: string
    onSubmit: (password: string, remember: boolean) => void
    onCancel: () => void
}) {
    const [password, setPassword] = useState('')
    const [remember, setRemember] = useState(true)
    const [visible, setVisible] = useState(false)

    function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault()
        if (!password) return
        onSubmit(password, remember)
    }

    return (
        <Dialog open onOpenChange={(open) => !open && onCancel()}>
            <DialogContent className="sm:max-w-sm">
                <form onSubmit={handleSubmit}>
                    <DialogHeader>
                        <DialogTitle>Password for {connectionName}</DialogTitle>
                        <DialogDescription>
                            {hint ?? 'This connection signs in with a password.'}
                        </DialogDescription>
                    </DialogHeader>

                    <div className="grid gap-3 py-4">
                        <Label htmlFor="session-password" className="sr-only">
                            Password
                        </Label>
                        <div className="flex gap-2">
                            <Input
                                id="session-password"
                                type={visible ? 'text' : 'password'}
                                value={password}
                                onChange={(event) => setPassword(event.target.value)}
                                autoComplete="off"
                                autoFocus
                            />
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                onClick={() => setVisible((current) => !current)}
                                aria-label={visible ? 'Hide password' : 'Show password'}
                            >
                                {visible ? <EyeOff/> : <Eye/>}
                            </Button>
                        </div>
                        <div className="flex items-center gap-2">
                            <Checkbox
                                id="session-remember"
                                checked={remember}
                                onCheckedChange={(value) => setRemember(value === true)}
                            />
                            <Label htmlFor="session-remember" className="cursor-pointer font-normal">
                                Save it in my keychain
                            </Label>
                        </div>
                    </div>

                    <DialogFooter>
                        <Button type="button" variant="outline" onClick={onCancel}>
                            Cancel
                        </Button>
                        <Button type="submit" disabled={!password}>
                            Connect
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}