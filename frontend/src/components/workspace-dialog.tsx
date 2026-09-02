import {useState} from 'react'

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
import {errorMessage, type Workspace} from '@/lib/api'
import {useWorkspaces} from '@/lib/workspaces-store'

export function WorkspaceDialog({
    workspace,
    onClose,
}: {
    /** Null or omitted means "create a new workspace". */
    workspace?: Workspace | null
    onClose: () => void
}) {
    const {createWorkspace, updateWorkspace} = useWorkspaces()
    const isEdit = Boolean(workspace)

    const [name, setName] = useState(workspace?.name ?? '')
    const [color, setColor] = useState(workspace?.color ?? 'slate')
    const [awsProfile, setAwsProfile] = useState(workspace?.awsProfile ?? '')
    const [awsRegion, setAwsRegion] = useState(workspace?.awsRegion ?? '')
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState<string | null>(null)

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
            }
            if (workspace) {
                await updateWorkspace(workspace.id, input)
            } else {
                await createWorkspace(input)
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
            <DialogContent>
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

                        <div className="grid gap-3 sm:grid-cols-2">
                            <div className="grid gap-2">
                                <Label htmlFor="aws-profile">AWS profile</Label>
                                <Input
                                    id="aws-profile"
                                    value={awsProfile}
                                    onChange={(event) => setAwsProfile(event.target.value)}
                                    placeholder="default"
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="aws-region">AWS region</Label>
                                <Input
                                    id="aws-region"
                                    value={awsRegion}
                                    onChange={(event) => setAwsRegion(event.target.value)}
                                    placeholder="ap-southeast-1"
                                />
                            </div>
                        </div>

                        <p className="text-xs text-muted-foreground">
                            SSM connections in this workspace inherit these two unless they set
                            their own. Leave both empty to let the AWS CLI use its own default.
                        </p>
                    </div>

                    <DialogFooter>
                        {error && <p className="mr-auto text-sm text-destructive">{error}</p>}
                        <Button type="button" variant="outline" onClick={onClose}>
                            Cancel
                        </Button>
                        <Button type="submit" disabled={saving || !name.trim()}>
                            {saving ? 'Saving…' : isEdit ? 'Save' : 'Create'}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}