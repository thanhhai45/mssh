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
	export class Workspace {
	    id: string;
	    name: string;
	    color: string;
	    awsProfile: string;
	    awsRegion: string;
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
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	        this.awsProfile = source["awsProfile"];
	        this.awsRegion = source["awsRegion"];
	    }
	}

}

