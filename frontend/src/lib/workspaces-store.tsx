import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';

import {
    api,
    errorMessage,
    type Connection,
    type ConnectionInput,
    type Workspace,
    type WorkspaceInput
} from '@/lib/api';

type WorkspacesContextValue = {
    workspaces: Workspace[];
    connections: Record<string, Connection[]>;
    loading: boolean;
    error: string | null;
    refresh: () => Promise<void>;
    createWorkspace: (input: WorkspaceInput) => Promise<Workspace>;
    updateWorkspace: (id: string, input: WorkspaceInput) => Promise<Workspace>;
    deleteWorkspace: (id: string) => Promise<void>;

    createConnection: (workspaceId: string, input: ConnectionInput) => Promise<Connection>;
    updateConnection: (id: string, input: ConnectionInput) => Promise<Connection>;
    deleteConnection: (id: string)  => Promise<void>;
    moveConnection: (id: string, toWorkspaceId: string) => Promise<void>;
}

const WorkspacesContext = createContext<WorkspacesContextValue | undefined>(undefined);

export function WorkspacesProvider({children}: {children: React.ReactNode}) {
    const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
    const [connections, setConnections] = useState<Record<string, Connection[]>>({});
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);

    const refresh = useCallback(async () => {
        try {
            const list = await api.listWorkspaces();
            const perWorkspace = await Promise.all(list.map((w) => api.listConnections(w.id)));
            const grouped: Record<string, Connection[]> = {};
            list.forEach((workspace, index) => {
                grouped[workspace.id] = perWorkspace[index];
            });
            setWorkspaces(list);
            setConnections(grouped);
            setError(null);
        } catch (err) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        void refresh();
    }, [refresh]);

    const createWorkspace = useCallback(async (input: WorkspaceInput) => {
        const created = await api.createWorkspace(input);
        await refresh();
        return created;
    }, [refresh]);

    const updateWorkspace = useCallback(async (id: string, input: WorkspaceInput) => {
        const updated = await api.updateWorkspace(id, input);
        await refresh();
        return updated;
    }, [refresh]);

    const deleteWorkspace = useCallback(async (id: string) => {
        await api.deleteWorkspace(id);
        await refresh();
    }, [refresh]);

    const createConnection = useCallback(async (workspaceId: string, input: ConnectionInput) => {
        const created = await api.createConnection(workspaceId, input);
        await refresh();
        return created;
    }, [refresh]);

    const updateConnection = useCallback(async (id: string, input: ConnectionInput) => {
        const updated = await api.updateConnection(id, input);
        await refresh();
        return updated;
    }, [refresh]);

    const deleteConnection = useCallback(async (id: string) => {
        await api.deleteConnection(id);
        await refresh();
    }, [refresh]);

    const moveConnection = useCallback(async (id: string, toWorkspaceId: string) => {
        await api.moveConnection(id, toWorkspaceId);
        await refresh();
    }, [refresh]);

    const value = useMemo(() => ({
        workspaces,
        connections,
        loading,
        error,
        refresh,
        createWorkspace,
        updateWorkspace,
        deleteWorkspace,
        createConnection,
        updateConnection,
        deleteConnection,
        moveConnection
    }), [
        workspaces,
        connections,
        loading,
        error,
        refresh,
        createWorkspace,
        updateWorkspace,
        deleteWorkspace,
        createConnection,
        updateConnection,
        deleteConnection,
        moveConnection
    ]);

    return (
        <WorkspacesContext.Provider value={value}>
            {children}
        </WorkspacesContext.Provider>
    );
}

export function useWorkspaces() {
    const context = useContext(WorkspacesContext);
    if (!context) {
        throw new Error('useWorkspaces must be used within a WorkspacesProvider');
    }
    return context;
}