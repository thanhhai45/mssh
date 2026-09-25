import appicon from '@/assets/appicon.png'

export function SidebarBrand() {
    return (
        <div className="flex h-12 items-center gap-2 px-2 group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:p-0">
            {/* The artwork is already a rounded dark square, so it stands on its
                own rather than sitting inside a coloured tile. alt is empty on
                purpose: the name is spelled out right beside it, and a screen
                reader saying "mssh" twice helps nobody. */}
            <img
                src={appicon}
                alt=""
                className="size-8 shrink-0 rounded-lg"
            />
            <div className="grid flex-1 text-left text-sm leading-tight group-data-[collapsible=icon]:hidden">
                <span className="truncate font-semibold">mssh</span>
                <span className="truncate text-xs text-muted-foreground">SSH workspace manager</span>
            </div>
        </div>
    )
}
