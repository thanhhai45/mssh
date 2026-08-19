import {createContext, useCallback, useContext, useMemo, useState} from 'react'

import {workspaces as initialWorkspaces} from '@/lib/workspaces'
import type {Server, Workspace} from '@/lib/workspaces'

type WorkspacesContextValue = {
    workspaces: Workspace[]
    addServer: (workspaceId: string, server: Omit<Server, 'id'>) => Server
}

const WorkspacesContext = createContext<WorkspacesContextValue | undefined>(undefined)

export function WorkspacesProvider({children}: {children: React.ReactNode}) {
    const [workspaces, setWorkspaces] = useState<Workspace[]>(initialWorkspaces)

    const addServer = useCallback((workspaceId: string, input: Omit<Server, 'id'>) => {
        const server: Server = {...input, id: crypto.randomUUID()}
        setWorkspaces((prev) =>
            prev.map((workspace) =>
                workspace.id === workspaceId
                    ? {...workspace, servers: [...workspace.servers, server]}
                    : workspace,
            ),
        )
        return server
    }, [])

    const value = useMemo(() => ({workspaces, addServer}), [workspaces, addServer])

    return <WorkspacesContext.Provider value={value}>{children}</WorkspacesContext.Provider>
}

export function useWorkspaces() {
    const context = useContext(WorkspacesContext)
    if (!context) {
        throw new Error('useWorkspaces must be used within a WorkspacesProvider')
    }
    return context
}
