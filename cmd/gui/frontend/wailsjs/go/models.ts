export namespace main {
	
	export class ConfigState {
	    appName: string;
	    path: string;
	    loadError: string;
	    tickIntervalSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new ConfigState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.path = source["path"];
	        this.loadError = source["loadError"];
	        this.tickIntervalSeconds = source["tickIntervalSeconds"];
	    }
	}

}

export namespace process {
	
	export class DiscoveredApp {
	    name: string;
	    childCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredApp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.childCount = source["childCount"];
	    }
	}
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
	    threads: number;
	    memMB: number;
	    cpuPercent: number;
	    cpuTimeMs: number;
	    cmdline?: string;
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
	        this.threads = source["threads"];
	        this.memMB = source["memMB"];
	        this.cpuPercent = source["cpuPercent"];
	        this.cpuTimeMs = source["cpuTimeMs"];
	        this.cmdline = source["cmdline"];
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

