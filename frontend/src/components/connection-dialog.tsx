import {useEffect, useState} from 'react'
import {Cloud, CloudCog, Eye, EyeOff, KeyRound} from 'lucide-react'

import {ColorPicker} from '@/components/color-picker'
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
import {RadioGroup, RadioGroupItem} from '@/components/ui/radio-group'
import {
    api,
    AUTH_META,
    AUTH_METHODS,
    CONNECTION_KINDS,
    emptyConnectionInput,
    errorMessage,
    KIND_META,
    toConnectionInput,
    usesAWS,
    usesSSH,
    type AuthMethod,
    type Connection,
    type ConnectionInput,
    type ConnectionKind,
    type Workspace,
} from '@/lib/api'
import {cn} from '@/lib/utils'
import {useWorkspaces} from '@/lib/workspaces-store'

const KIND_ICON: Record<ConnectionKind, typeof KeyRound> = {
    'ssh': KeyRound,
    'ssm': Cloud,
    'ssm-ssh': CloudCog,
}

export function ConnectionDialog({
    workspace,
    connection,
    onClose,
}: {
    workspace: Workspace
    /** Null or omitted means "create a new connection". */
    connection?: Connection | null
    onClose: () => void
}) {
    const {createConnection, updateConnection} = useWorkspaces()
    const isEdit = Boolean(connection)

    // One object holding every field, even the ones the current kind hides.
    // Switching kind back and forth must not lose what was already typed.
    const [input, setInput] = useState<ConnectionInput>(
        connection ? toConnectionInput(connection) : emptyConnectionInput(),
    )
    const [sshCommand, setSSHCommand] = useState('')
    const [password, setPassword] = useState('')
    const [remember, setRemember] = useState(true)
    const [showPassword, setShowPassword] = useState(false)
    const [hasStored, setHasStored] = useState(false)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState<string | null>(null)

    // Whether a password sits in the keychain is not part of the row, so it has
    // to be asked for separately.
    useEffect(() => {
        if (!connection) return

        let cancelled = false
        api.hasConnectionPassword(connection.id)
            .then((has) => {
                if (!cancelled) setHasStored(has)
            })
            .catch(() => {
                // Not knowing is the same as "nothing saved" for the UI.
            })

        return () => {
            cancelled = true
        }
    }, [connection])

    function set<K extends keyof ConnectionInput>(key: K, value: ConnectionInput[K]) {
        setInput((prev) => ({...prev, [key]: value}))
    }

    async function fillFromCommand() {
        setError(null)
        try {
            const parsed = await api.parseSSHCommand(sshCommand)
            setInput((prev) => ({
                ...prev,
                target: parsed.host,
                port: parsed.port,
                username: parsed.username,
                keyPath: parsed.keyPath,
                authMethod: parsed.keyPath ? 'key' : prev.authMethod,
            }))
        } catch (err) {
            setError(errorMessage(err))
        }
    }

    async function forgetPassword() {
        if (!connection) return
        setError(null)
        try {
            await api.deleteConnectionPassword(connection.id)
            setHasStored(false)
        } catch (err) {
            setError(errorMessage(err))
        }
    }

    async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault()
        setSaving(true)
        setError(null)
        try {
            const saved = connection
                ? await updateConnection(connection.id, input)
                : await createConnection(workspace.id, input)

            const wantsPassword = usesSSH(input.kind) && input.authMethod === 'password'

            if (wantsPassword) {
                if (password && remember) {
                    await api.setConnectionPassword(saved.id, password)
                }
            } else if (hasStored) {
                // This connection no longer authenticates with a password, so
                // the stored one is dead weight.
                await api.deleteConnectionPassword(saved.id)
            }

            onClose()
        } catch (err) {
            setError(errorMessage(err))
        } finally {
            setSaving(false)
        }
    }

    const showSSHFields = usesSSH(input.kind)
    const showAWSFields = usesAWS(input.kind)
    const showPasswordField = showSSHFields && input.authMethod === 'password'

    return (
        <Dialog open onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
                <form onSubmit={handleSubmit}>
                    <DialogHeader>
                        <DialogTitle>{isEdit ? 'Edit connection' : 'New connection'}</DialogTitle>
                        <DialogDescription>
                            In workspace <span className="font-medium">{workspace.name}</span>.
                        </DialogDescription>
                    </DialogHeader>

                    <div className="grid gap-5 py-4">
                        {/* ---- Kind ---- */}
                        <div className="grid gap-2">
                            <Label>How to reach it</Label>
                            <div className="grid gap-2 sm:grid-cols-3">
                                {CONNECTION_KINDS.map((kind) => {
                                    const Icon = KIND_ICON[kind]
                                    const selected = input.kind === kind
                                    return (
                                        <button
                                            key={kind}
                                            type="button"
                                            onClick={() => set('kind', kind)}
                                            aria-pressed={selected}
                                            className={cn(
                                                'flex flex-col gap-1 rounded-lg border p-3 text-left transition',
                                                selected
                                                    ? 'border-primary bg-primary/5'
                                                    : 'hover:border-muted-foreground/40',
                                            )}
                                        >
                                            <Icon className="size-4 text-muted-foreground"/>
                                            <span className="text-sm font-medium">
                                                {KIND_META[kind].label}
                                            </span>
                                            <span className="text-xs leading-snug text-muted-foreground">
                                                {KIND_META[kind].hint}
                                            </span>
                                        </button>
                                    )
                                })}
                            </div>
                        </div>

                        {/* ---- Name ---- */}
                        <div className="grid gap-2">
                            <Label htmlFor="conn-name">Name</Label>
                            <Input
                                id="conn-name"
                                value={input.name}
                                onChange={(e) => set('name', e.target.value)}
                                placeholder="web-1"
                                autoFocus
                            />
                        </div>

                        {/* ---- Paste helper, direct SSH only ---- */}
                        {input.kind === 'ssh' && (
                            <div className="grid gap-2">
                                <Label htmlFor="conn-paste">Paste an ssh command (optional)</Label>
                                <div className="flex gap-2">
                                    <Input
                                        id="conn-paste"
                                        value={sshCommand}
                                        onChange={(e) => setSSHCommand(e.target.value)}
                                        placeholder="ssh -p 2222 deploy@10.0.4.12"
                                    />
                                    <Button
                                        type="button"
                                        variant="secondary"
                                        onClick={fillFromCommand}
                                        disabled={!sshCommand.trim()}
                                    >
                                        Fill
                                    </Button>
                                </div>
                            </div>
                        )}

                        {/* ---- Target ---- */}
                        <div className="grid gap-2">
                            <Label htmlFor="conn-target">
                                {showAWSFields ? 'Instance ID' : 'Host'}
                            </Label>
                            <Input
                                id="conn-target"
                                value={input.target}
                                onChange={(e) => set('target', e.target.value)}
                                placeholder={showAWSFields ? 'i-0abc123456789' : '10.0.4.12'}
                            />
                        </div>

                        {/* ---- SSH-only fields ---- */}
                        {showSSHFields && (
                            <>
                                <div className="grid gap-3 sm:grid-cols-[1fr_120px]">
                                    <div className="grid gap-2">
                                        <Label htmlFor="conn-username">Username</Label>
                                        <Input
                                            id="conn-username"
                                            value={input.username}
                                            onChange={(e) => set('username', e.target.value)}
                                            placeholder="ec2-user"
                                        />
                                    </div>
                                    <div className="grid gap-2">
                                        <Label htmlFor="conn-port">Port</Label>
                                        <Input
                                            id="conn-port"
                                            type="number"
                                            value={input.port || ''}
                                            onChange={(e) => set('port', Number(e.target.value) || 0)}
                                            placeholder="22"
                                        />
                                    </div>
                                </div>

                                <div className="grid gap-2">
                                    <Label>Authentication</Label>
                                    <RadioGroup
                                        value={input.authMethod}
                                        onValueChange={(value) => set('authMethod', value as AuthMethod)}
                                        className="gap-2"
                                    >
                                        {AUTH_METHODS.map((method) => (
                                            <div key={method} className="flex items-start gap-2">
                                                <RadioGroupItem
                                                    value={method}
                                                    id={`auth-${method}`}
                                                    className="mt-1"
                                                />
                                                <Label
                                                    htmlFor={`auth-${method}`}
                                                    className="grid cursor-pointer gap-0.5 font-normal"
                                                >
                                                    <span className="font-medium">
                                                        {AUTH_META[method].label}
                                                    </span>
                                                    <span className="text-xs text-muted-foreground">
                                                        {AUTH_META[method].hint}
                                                    </span>
                                                </Label>
                                            </div>
                                        ))}
                                    </RadioGroup>
                                </div>

                                {input.authMethod === 'key' && (
                                    <div className="grid gap-2">
                                        <Label htmlFor="conn-keypath">Key file</Label>
                                        <Input
                                            id="conn-keypath"
                                            value={input.keyPath}
                                            onChange={(e) => set('keyPath', e.target.value)}
                                            placeholder="~/.ssh/id_ed25519"
                                        />
                                    </div>
                                )}

                                {showPasswordField && (
                                    <div className="grid gap-2 rounded-lg border p-3">
                                        {hasStored ? (
                                            <div className="flex items-center justify-between gap-2">
                                                <span className="text-sm text-muted-foreground">
                                                    A password is saved in your keychain.
                                                </span>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    size="sm"
                                                    onClick={forgetPassword}
                                                >
                                                    Forget
                                                </Button>
                                            </div>
                                        ) : (
                                            <>
                                                <Label htmlFor="conn-password">Password</Label>
                                                <div className="flex gap-2">
                                                    <Input
                                                        id="conn-password"
                                                        type={showPassword ? 'text' : 'password'}
                                                        value={password}
                                                        onChange={(e) => setPassword(e.target.value)}
                                                        placeholder="Leave empty to be asked on connect"
                                                        autoComplete="off"
                                                    />
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="icon"
                                                        onClick={() => setShowPassword((v) => !v)}
                                                        aria-label={showPassword ? 'Hide password' : 'Show password'}
                                                    >
                                                        {showPassword ? <EyeOff/> : <Eye/>}
                                                    </Button>
                                                </div>
                                                <div className="flex items-center gap-2">
                                                    <Checkbox
                                                        id="conn-remember"
                                                        checked={remember}
                                                        onCheckedChange={(v) => setRemember(v === true)}
                                                        disabled={!password}
                                                    />
                                                    <Label
                                                        htmlFor="conn-remember"
                                                        className="cursor-pointer font-normal"
                                                    >
                                                        Save it in my keychain
                                                    </Label>
                                                </div>
                                            </>
                                        )}
                                    </div>
                                )}
                            </>
                        )}

                        {/* ---- AWS-only fields ---- */}
                        {showAWSFields && (
                            <div className="grid gap-3 sm:grid-cols-2">
                                <div className="grid gap-2">
                                    <Label htmlFor="conn-profile">AWS profile</Label>
                                    <Input
                                        id="conn-profile"
                                        value={input.awsProfile}
                                        onChange={(e) => set('awsProfile', e.target.value)}
                                        placeholder={
                                            workspace.awsProfile
                                                ? `inherits ${workspace.awsProfile}`
                                                : 'aws cli default'
                                        }
                                    />
                                </div>
                                <div className="grid gap-2">
                                    <Label htmlFor="conn-region">AWS region</Label>
                                    <Input
                                        id="conn-region"
                                        value={input.awsRegion}
                                        onChange={(e) => set('awsRegion', e.target.value)}
                                        placeholder={
                                            workspace.awsRegion
                                                ? `inherits ${workspace.awsRegion}`
                                                : 'aws cli default'
                                        }
                                    />
                                </div>
                            </div>
                        )}

                        {/* ---- Colour ---- */}
                        <div className="grid gap-2">
                            <Label>Colour</Label>
                            <ColorPicker value={input.color} onChange={(c) => set('color', c)}/>
                        </div>
                    </div>

                    <DialogFooter>
                        {error && <p className="mr-auto text-sm text-destructive">{error}</p>}
                        <Button type="button" variant="outline" onClick={onClose}>
                            Cancel
                        </Button>
                        <Button
                            type="submit"
                            disabled={saving || !input.name.trim() || !input.target.trim()}
                        >
                            {saving ? 'Saving…' : isEdit ? 'Save' : 'Create'}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}