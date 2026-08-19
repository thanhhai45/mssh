export type WorkspaceStatus = 'connected' | 'disconnected'

export type Server = {
    id: string
    name: string
    command: string
}

export type Workspace = {
    id: string
    name: string
    status: WorkspaceStatus
    servers: Server[]
}

export const workspaces: Workspace[] = [
    {
        id: 'prod-web-01',
        name: 'prod-web-01',
        status: 'connected',
        servers: [
            {id: 'web-1', name: 'web-1', command: 'ssh deploy@10.0.4.12 -p 22'},
            {id: 'web-2', name: 'web-2', command: 'ssh deploy@10.0.4.13 -p 22'},
        ],
    },
    {
        id: 'staging-api',
        name: 'staging-api',
        status: 'disconnected',
        servers: [{id: 'api-1', name: 'api-1', command: 'ssh ubuntu@10.0.6.30 -p 22'}],
    },
    {
        id: 'db-replica',
        name: 'db-replica',
        status: 'disconnected',
        servers: [],
    },
]
