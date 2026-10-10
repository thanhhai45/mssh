import {useEffect, useState} from 'react'
import {Eye, EyeOff, TerminalSquare} from 'lucide-react'

import {ColorPicker} from '@/components/color-picker'
import {Button} from '@/components/ui/button'
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
    errorMessage,
    type AWSCredentialsSource,
    type AWSShellPreview,
    type Workspace,
} from '@/lib/api'
import {useWorkspaces} from '@/lib/workspaces-store'

export function WorkspaceDialog({
    workspace,
    onClose,
}: {
    /** Null or omitted means "create a new workspace". */
    workspace?: Workspace | null
    onClose: () => void
}) {
    const {createWorkspace, updateWorkspace, refresh} = useWorkspaces()
    const isEdit = Boolean(workspace)

    const [name, setName] = useState(workspace?.name ?? '')
    const [color, setColor] = useState(workspace?.color ?? 'slate')
    const [awsProfile, setAwsProfile] = useState(workspace?.awsProfile ?? '')
    const [awsRegion, setAwsRegion] = useState(workspace?.awsRegion ?? '')

    const [credentialsSource, setCredentialsSource] = useState<AWSCredentialsSource>(
        workspace?.awsCredentialsSource === 'stored' ? 'stored' : 'cli')
    const [accessKeyId, setAccessKeyId] = useState(workspace?.awsAccessKeyId ?? '')
    // What is typed now. Never filled from the saved value, because the saved
    // value never comes back from Go.
    const [secretAccessKey, setSecretAccessKey] = useState('')
    const [sessionToken, setSessionToken] = useState('')
    const [showSecret, setShowSecret] = useState(false)
    const [hasSavedSecret, setHasSavedSecret] = useState(false)

    // The shell import, in two steps: look, then decide. Nothing is written
    // until Save, like every other field in this dialog.
    const [preview, setPreview] = useState<AWSShellPreview | null>(null)
    const [useShellKeys, setUseShellKeys] = useState(false)
    const [readingShell, setReadingShell] = useState(false)

    const [saving, setSaving] = useState(false)
    const [error, setError] = useState<string | null>(null)

    // Whether a secret is saved is not part of the row, so it is asked for
    // separately — and the answer is a boolean, never the secret.
    useEffect(() => {
        if (!workspace) return

        let cancelled = false
        api.hasWorkspaceAWSSecret(workspace.id)
            .then((has) => {
                if (!cancelled) setHasSavedSecret(has)
            })
            .catch(() => {
                // Not knowing is the same as "nothing saved" for the UI.
            })

        return () => {
            cancelled = true
        }
    }, [workspace])

    const stored = credentialsSource === 'stored'
    const hasKeyPair = useShellKeys || (accessKeyId.trim() !== '' && (hasSavedSecret || secretAccessKey !== ''))
    // Stored keys come with no ~/.aws/config to take a region from. Go refuses
    // the combination too (store.ErrStoredKeysNeedRegion); this only says so
    // before the user presses Save.
    const missingRegion = stored && awsRegion.trim() === ''
    const canSave = name.trim() !== '' && (!stored || hasKeyPair) && !missingRegion

    async function readShell() {
        setReadingShell(true)
        setError(null)
        try {
            setPreview(await api.previewAWSFromShell())
        } catch (err) {
            setError(errorMessage(err))
        } finally {
            setReadingShell(false)
        }
    }

    function acceptShellKeys(found: AWSShellPreview) {
        setUseShellKeys(true)
        setAccessKeyId(found.accessKeyId)
        setSecretAccessKey('')
        setSessionToken('')
        if (!awsRegion.trim() && found.region) setAwsRegion(found.region)
        setPreview(null)
    }

    async function forgetSecret() {
        if (!workspace) return
        setError(null)
        try {
            await api.clearWorkspaceAWSSecret(workspace.id)
            setHasSavedSecret(false)
        } catch (err) {
            setError(errorMessage(err))
        }
    }

    async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault()
        setSaving(true)
        setError(null)
        try {
            const input = {
                name: name.trim(),
                color,
                awsProfile: awsProfile.trim(),
                awsRegion: awsRegion.trim(),
                awsCredentialsSource: credentialsSource,
                awsAccessKeyId: stored ? accessKeyId.trim() : '',
            }

            // The workspace first: a secret is stored against its id, and a
            // new workspace has none until it has been created.
            const saved = workspace
                ? await updateWorkspace(workspace.id, input)
                : await createWorkspace(input)

            // Secrets travel on their own, never inside `input`: ordinary fields
            // get logged and echoed back, and a secret must not ride with them.
            if (stored && useShellKeys) {
                // Go reads the shell again itself; the secret was never here.
                await api.importAWSFromShell(saved.id)
                await refresh()
            } else if (stored && secretAccessKey) {
                await api.setWorkspaceAWSSecret(saved.id, secretAccessKey, sessionToken.trim())
            } else if (!stored && hasSavedSecret) {
                // "Nothing is stored in mssh" is what the CLI option promises.
                await api.clearWorkspaceAWSSecret(saved.id)
            }
            onClose()
        } catch (err) {
            setError(errorMessage(err))
        } finally {
            setSaving(false)
        }
    }

    return (
        <Dialog open onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
                <form onSubmit={handleSubmit}>
                    <DialogHeader>
                        <DialogTitle>{isEdit ? 'Edit workspace' : 'New workspace'}</DialogTitle>
                        <DialogDescription>
                            A workspace groups connections that belong together — usually one
                            AWS account or one environment.
                        </DialogDescription>
                    </DialogHeader>

                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label htmlFor="workspace-name">Name</Label>
                            <Input
                                id="workspace-name"
                                value={name}
                                onChange={(event) => setName(event.target.value)}
                                placeholder="AWS Prod"
                                autoFocus
                            />
                        </div>

                        <div className="grid gap-2">
                            <Label>Colour</Label>
                            <ColorPicker value={color} onChange={setColor}/>
                        </div>

                        <div className="grid gap-3">
                            <Label>AWS credentials</Label>
                            <RadioGroup
                                value={credentialsSource}
                                onValueChange={(value) => setCredentialsSource(value as AWSCredentialsSource)}
                            >
                                <Label htmlFor="credentials-cli" className="flex cursor-pointer items-start gap-2 font-normal">
                                    <RadioGroupItem value="cli" id="credentials-cli" className="mt-0.5"/>
                                    <span className="grid gap-0.5">
                                        <span>Use the AWS CLI&apos;s configuration</span>
                                        <span className="text-xs text-muted-foreground">
                                            ~/.aws, SSO or the profile below. Nothing is stored in mssh.
                                        </span>
                                    </span>
                                </Label>
                                <Label htmlFor="credentials-stored" className="flex cursor-pointer items-start gap-2 font-normal">
                                    <RadioGroupItem value="stored" id="credentials-stored" className="mt-0.5"/>
                                    <span className="grid gap-0.5">
                                        <span>Use keys stored in mssh</span>
                                        <span className="text-xs text-muted-foreground">
                                            For machines without the AWS CLI set up. Kept in the local
                                            database on this machine, never sent anywhere.
                                        </span>
                                    </span>
                                </Label>
                            </RadioGroup>

                            {stored && (
                                <div className="grid gap-3 rounded-lg border p-3">
                                    {useShellKeys ? (
                                        <div className="flex items-center justify-between gap-2">
                                            <span className="text-sm text-muted-foreground">
                                                Keys from your shell (<code>{accessKeyId}</code>) will be
                                                saved when you press {isEdit ? 'Save' : 'Create'}.
                                            </span>
                                            <Button type="button" variant="outline" size="sm" onClick={() => setUseShellKeys(false)}>
                                                Undo
                                            </Button>
                                        </div>
                                    ) : (
                                        <>
                                            <div className="grid gap-2">
                                                <Label htmlFor="access-key-id">Access key ID</Label>
                                                <Input
                                                    id="access-key-id"
                                                    value={accessKeyId}
                                                    onChange={(event) => setAccessKeyId(event.target.value)}
                                                    placeholder="AKIA…"
                                                    autoComplete="off"
                                                    spellCheck={false}
                                                />
                                            </div>

                                            {hasSavedSecret ? (
                                                <div className="flex items-center justify-between gap-2">
                                                    <span className="text-sm text-muted-foreground">
                                                        A secret key is saved on this machine.
                                                    </span>
                                                    <Button type="button" variant="outline" size="sm" onClick={forgetSecret}>
                                                        Forget
                                                    </Button>
                                                </div>
                                            ) : (
                                                <>
                                                    <div className="grid gap-2">
                                                        <Label htmlFor="secret-access-key">Secret access key</Label>
                                                        <div className="flex gap-2">
                                                            <Input
                                                                id="secret-access-key"
                                                                type={showSecret ? 'text' : 'password'}
                                                                value={secretAccessKey}
                                                                onChange={(event) => setSecretAccessKey(event.target.value)}
                                                                autoComplete="off"
                                                                spellCheck={false}
                                                            />
                                                            <Button
                                                                type="button"
                                                                variant="ghost"
                                                                size="icon"
                                                                onClick={() => setShowSecret((shown) => !shown)}
                                                                aria-label={showSecret ? 'Hide secret' : 'Show secret'}
                                                            >
                                                                {showSecret ? <EyeOff/> : <Eye/>}
                                                            </Button>
                                                        </div>
                                                    </div>
                                                    <div className="grid gap-2">
                                                        <Label htmlFor="session-token">Session token (optional)</Label>
                                                        <Input
                                                            id="session-token"
                                                            type="password"
                                                            value={sessionToken}
                                                            onChange={(event) => setSessionToken(event.target.value)}
                                                            placeholder="Only for temporary credentials"
                                                            autoComplete="off"
                                                            spellCheck={false}
                                                        />
                                                    </div>
                                                </>
                                            )}

                                            <div className="flex items-center gap-2">
                                                <Button type="button" variant="outline" size="sm" onClick={readShell} disabled={readingShell}>
                                                    <TerminalSquare/>
                                                    {readingShell ? 'Reading your shell…' : 'Import from shell'}
                                                </Button>
                                                <span className="text-xs text-muted-foreground">
                                                    Reads AWS_* from ~/.zshrc or ~/.bashrc, nothing else.
                                                </span>
                                            </div>
                                        </>
                                    )}

                                    {preview && (
                                        <ShellPreview
                                            preview={preview}
                                            onUse={() => acceptShellKeys(preview)}
                                            onUseProfile={(profile) => {
                                                setAwsProfile(profile)
                                                setCredentialsSource('cli')
                                                setPreview(null)
                                            }}
                                            onCancel={() => setPreview(null)}
                                        />
                                    )}
                                </div>
                            )}
                        </div>

                        <div className="grid gap-3 sm:grid-cols-2">
                            <div className="grid gap-2">
                                <Label htmlFor="aws-profile">AWS profile</Label>
                                <Input
                                    id="aws-profile"
                                    value={awsProfile}
                                    onChange={(event) => setAwsProfile(event.target.value)}
                                    placeholder={stored ? 'Not used with stored keys' : 'default'}
                                    disabled={stored}
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="aws-region">AWS region</Label>
                                <Input
                                    id="aws-region"
                                    value={awsRegion}
                                    onChange={(event) => setAwsRegion(event.target.value)}
                                    placeholder="ap-southeast-1"
                                    aria-invalid={missingRegion}
                                />
                            </div>
                        </div>

                        <p className="text-xs text-muted-foreground">
                            {stored
                                ? 'A region is required with stored keys: there is no ~/.aws/config to take one from. Connections in this workspace use it, System SSH ProxyCommands included.'
                                : 'SSM connections in this workspace inherit these two unless they set their own. Leave both empty to let the AWS CLI use its own default.'}
                        </p>
                    </div>

                    <DialogFooter>
                        {error && <p className="mr-auto text-sm text-destructive">{error}</p>}
                        <Button type="button" variant="outline" onClick={onClose}>
                            Cancel
                        </Button>
                        <Button type="submit" disabled={saving || !canSave}>
                            {saving ? 'Saving…' : isEdit ? 'Save' : 'Create'}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}

/** What the login shell exports, shown before anything is saved. */
function ShellPreview({
    preview,
    onUse,
    onUseProfile,
    onCancel,
}: {
    preview: AWSShellPreview
    onUse: () => void
    onUseProfile: (profile: string) => void
    onCancel: () => void
}) {
    const hasPair = preview.accessKeyId !== '' && preview.hasSecretAccessKey

    if (!hasPair) {
        return (
            <div className="grid gap-2 rounded-md bg-muted p-3 text-sm">
                <p className="text-muted-foreground">No AWS key pair in your login shell.</p>
                {preview.profile && (
                    <div className="flex items-center justify-between gap-2">
                        <span className="text-muted-foreground">
                            It sets <code>AWS_PROFILE={preview.profile}</code> — that belongs with the
                            AWS CLI&apos;s configuration.
                        </span>
                        <Button type="button" size="sm" variant="outline" onClick={() => onUseProfile(preview.profile)}>
                            Use the profile
                        </Button>
                    </div>
                )}
                <div>
                    <Button type="button" size="sm" variant="ghost" onClick={onCancel}>
                        Close
                    </Button>
                </div>
            </div>
        )
    }

    return (
        <div className="grid gap-2 rounded-md bg-muted p-3 text-sm">
            <p>Found in your login shell:</p>
            <ul className="grid gap-0.5 text-xs text-muted-foreground">
                <li>AWS_ACCESS_KEY_ID: <code>{preview.accessKeyId}</code></li>
                <li>AWS_SECRET_ACCESS_KEY: set</li>
                {preview.hasSessionToken && <li>AWS_SESSION_TOKEN: set</li>}
                {preview.region && <li>Region: <code>{preview.region}</code></li>}
            </ul>
            <div className="flex gap-2">
                <Button type="button" size="sm" onClick={onUse}>
                    Use these
                </Button>
                <Button type="button" size="sm" variant="ghost" onClick={onCancel}>
                    Cancel
                </Button>
            </div>
        </div>
    )
}
