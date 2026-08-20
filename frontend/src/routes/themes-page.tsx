import {Monitor, Moon, Sun} from 'lucide-react'

import {useTheme} from '@/components/theme-provider'
import {Label} from '@/components/ui/label'
import {RadioGroup, RadioGroupItem} from '@/components/ui/radio-group'

const options = [
    {value: 'light', label: 'Light', icon: Sun},
    {value: 'dark', label: 'Dark', icon: Moon},
    {value: 'system', label: 'System', icon: Monitor},
] as const

export function ThemesPage() {
    const {theme, setTheme} = useTheme()

    return (
        <div className="flex flex-col gap-6">
            <div>
                <h1 className="text-2xl font-semibold tracking-tight">Themes</h1>
                <p className="text-sm text-muted-foreground">
                    Choose how mssh looks on your device.
                </p>
            </div>
            <RadioGroup
                value={theme}
                onValueChange={(value) => setTheme(value as typeof theme)}
                className="grid max-w-md gap-3 sm:grid-cols-3"
            >
                {options.map(({value, label, icon: Icon}) => (
                    <Label
                        key={value}
                        htmlFor={`theme-${value}`}
                        className="flex cursor-pointer flex-col items-center gap-3 rounded-lg border p-4 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-accent"
                    >
                        <Icon className="size-5"/>
                        <span className="text-sm font-medium">{label}</span>
                        <RadioGroupItem value={value} id={`theme-${value}`} className="sr-only"/>
                    </Label>
                ))}
            </RadioGroup>
        </div>
    )
}
