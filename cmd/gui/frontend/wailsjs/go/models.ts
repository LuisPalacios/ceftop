export namespace main {
	
	export class AppInfo {
	    name: string;
	    friendlyName: string;
	    iconSrc: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.friendlyName = source["friendlyName"];
	        this.iconSrc = source["iconSrc"];
	    }
	}
	export class ConfigState {
	    appName: string;
	    path: string;
	    loadError: string;
	    tickIntervalSeconds: number;
	    catalogPath: string;
	    catalogError: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.path = source["path"];
	        this.loadError = source["loadError"];
	        this.tickIntervalSeconds = source["tickIntervalSeconds"];
	        this.catalogPath = source["catalogPath"];
	        this.catalogError = source["catalogError"];
	    }
	}
	export class HostView {
	    exe: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new HostView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exe = source["exe"];
	        this.label = source["label"];
	    }
	}
	export class DiscoveredAppView {
	    name: string;
	    friendlyName: string;
	    childCount: number;
	    iconSrc: string;
	    hosts?: HostView[];
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredAppView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.friendlyName = source["friendlyName"];
	        this.childCount = source["childCount"];
	        this.iconSrc = source["iconSrc"];
	        this.hosts = this.convertValues(source["hosts"], HostView);
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

}

export namespace process {
	
	export class KillResult {
	    killed: boolean;
	    err?: string;
	    errno?: string;
	
	    static createFrom(source: any = {}) {
	        return new KillResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.killed = source["killed"];
	        this.err = source["err"];
	        this.errno = source["errno"];
	    }
	}
	export class ProcessNode {
	    pid: number;
	    ppid: number;
	    role: string;
	    name: string;
	    host?: string;
	    hostPid?: number;
	    hostLabel?: string;
	    threads: number;
	    memMB: number;
	    cpuPercent: number;
	    cpuTimeMs: number;
	    children?: ProcessNode[];
	
	    static createFrom(source: any = {}) {
	        return new ProcessNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.ppid = source["ppid"];
	        this.role = source["role"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.hostPid = source["hostPid"];
	        this.hostLabel = source["hostLabel"];
	        this.threads = source["threads"];
	        this.memMB = source["memMB"];
	        this.cpuPercent = source["cpuPercent"];
	        this.cpuTimeMs = source["cpuTimeMs"];
	        this.children = this.convertValues(source["children"], ProcessNode);
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
	export class ProcessSnapshot {
	    target: string;
	    roots: ProcessNode[];
	    // Go type: time
	    captured: any;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.roots = this.convertValues(source["roots"], ProcessNode);
	        this.captured = this.convertValues(source["captured"], null);
	        this.total = source["total"];
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

}

