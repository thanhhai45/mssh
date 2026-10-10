export namespace files {
	
	export class Entry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    isLink: boolean;
	    size: number;
	    modifiedAt: number;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.isLink = source["isLink"];
	        this.size = source["size"];
	        this.modifiedAt = source["modifiedAt"];
	        this.mode = source["mode"];
	    }
	}
	export class Listing {
	    path: string;
	    parent: string;
	    entries: Entry[];
	
	    static createFrom(source: any = {}) {
	        return new Listing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.parent = source["parent"];
	        this.entries = this.convertValues(source["entries"], Entry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Transfer {
	    id: string;
	    sessionId: string;
	    direction: string;
	    name: string;
	    source: string;
	    destination: string;
	    done: number;
	    total: number;
	    state: string;
	    message: string;
	    startedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Transfer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.direction = source["direction"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.destination = source["destination"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.state = source["state"];
	        this.message = source["message"];
	        this.startedAt = source["startedAt"];
	    }
	}

}

export namespace session {
	
	export class Info {
	    sessionId: string;
	    connectionId: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.connectionId = source["connectionId"];
	    }
	}

}

export namespace store {
	
	export class Connection {
	    id: string;
	    workspaceId: string;
	    name: string;
	    kind: string;
	    target: string;
	    port: number;
	    username: string;
	    authMethod: string;
	    keyPath: string;
	    awsProfile: string;
	    awsRegion: string;
	    extra: string;
	    color: string;
	    sortOrder: number;
	    createdAt: number;
	    updatedAt: number;
	    lastUsedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authMethod = source["authMethod"];
	        this.keyPath = source["keyPath"];
	        this.awsProfile = source["awsProfile"];
	        this.awsRegion = source["awsRegion"];
	        this.extra = source["extra"];
	        this.color = source["color"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastUsedAt = source["lastUsedAt"];
	    }
	}
	export class ConnectionInput {
	    name: string;
	    kind: string;
	    target: string;
	    port: number;
	    username: string;
	    authMethod: string;
	    keyPath: string;
	    awsProfile: string;
	    awsRegion: string;
	    extra: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authMethod = source["authMethod"];
	        this.keyPath = source["keyPath"];
	        this.awsProfile = source["awsProfile"];
	        this.awsRegion = source["awsRegion"];
	        this.extra = source["extra"];
	        this.color = source["color"];
	    }
	}
	export class ParsedSSHCommand {
	    username: string;
	    host: string;
	    port: number;
	    keyPath: string;
	
	    static createFrom(source: any = {}) {
	        return new ParsedSSHCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.keyPath = source["keyPath"];
	    }
	}
	export class ResolvedAWS {
	    profile: string;
	    region: string;
	
	    static createFrom(source: any = {}) {
	        return new ResolvedAWS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.region = source["region"];
	    }
	}
	export class SessionLogEntry {
	    id: string;
	    connectionId: string;
	    workspaceId: string;
	    connectionName: string;
	    workspaceName: string;
	    kind: string;
	    target: string;
	    username: string;
	    awsProfile: string;
	    awsRegion: string;
	    awsCredentialsSource: string;
	    openedAt: number;
	    closedAt: number;
	    endReason: string;
	    endMessage: string;
	    reconnectOf: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.connectionId = source["connectionId"];
	        this.workspaceId = source["workspaceId"];
	        this.connectionName = source["connectionName"];
	        this.workspaceName = source["workspaceName"];
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.username = source["username"];
	        this.awsProfile = source["awsProfile"];
	        this.awsRegion = source["awsRegion"];
	        this.awsCredentialsSource = source["awsCredentialsSource"];
	        this.openedAt = source["openedAt"];
	        this.closedAt = source["closedAt"];
	        this.endReason = source["endReason"];
	        this.endMessage = source["endMessage"];
	        this.reconnectOf = source["reconnectOf"];
	    }
	}
	export class SessionLogFilter {
	    workspaceId: string;
	    connectionId: string;
	    from: number;
	    to: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionLogFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.connectionId = source["connectionId"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.limit = source["limit"];
	    }
	}
	export class Workspace {
	    id: string;
	    name: string;
	    color: string;
	    awsProfile: string;
	    awsRegion: string;
	    awsCredentialsSource: string;
	    awsAccessKeyId: string;
	    sortOrder: number;
	    createdAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.awsProfile = source["awsProfile"];
	        this.awsRegion = source["awsRegion"];
	        this.awsCredentialsSource = source["awsCredentialsSource"];
	        this.awsAccessKeyId = source["awsAccessKeyId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class WorkspaceInput {
	    name: string;
	    color: string;
	    awsProfile: string;
	    awsRegion: string;
	    awsCredentialsSource: string;
	    awsAccessKeyId: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	        this.awsProfile = source["awsProfile"];
	        this.awsRegion = source["awsRegion"];
	        this.awsCredentialsSource = source["awsCredentialsSource"];
	        this.awsAccessKeyId = source["awsAccessKeyId"];
	    }
	}

}

export namespace transport {
	
	export class AWSShellPreview {
	    accessKeyId: string;
	    hasSecretAccessKey: boolean;
	    hasSessionToken: boolean;
	    region: string;
	    profile: string;
	
	    static createFrom(source: any = {}) {
	        return new AWSShellPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessKeyId = source["accessKeyId"];
	        this.hasSecretAccessKey = source["hasSecretAccessKey"];
	        this.hasSessionToken = source["hasSessionToken"];
	        this.region = source["region"];
	        this.profile = source["profile"];
	    }
	}
	export class HostKeyPrompt {
	    host: string;
	    keyType: string;
	    fingerprint: string;
	    knownHostsPath: string;
	
	    static createFrom(source: any = {}) {
	        return new HostKeyPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.keyType = source["keyType"];
	        this.fingerprint = source["fingerprint"];
	        this.knownHostsPath = source["knownHostsPath"];
	    }
	}

}

