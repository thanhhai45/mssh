import {Check} from 'lucide-react'
import {WORKSPACE_COLORS, swatchClass} from '@/lib/colors'
import {cn} from '@/lib/utils'

export function ColorPicker({
    value,
    onChange,
}: {
    value: string
    onChange: (color: string) => void
}) {
    return (
        <div className="flex flex-wrap gap-2">
            {WORKSPACE_COLORS.map((color) => (
                <button
                    key={color}
                    type="button"
                    onClick={() => onChange(color)}
                    aria-label={color}
                    aria-pressed={value === color}
                    className={cn(
                        'flex size-7 items-center justify-center rounded-full transition',
                        'ring-offset-2 ring-offset-background',
                        swatchClass(color),
                        value === color && 'ring-2 ring-ring',
                    )}
                >
                    {value === color && <Check className="size-4 text-white"/>}
                </button>
            ))}
        </div>
    )
}