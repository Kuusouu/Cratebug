export namespace discovery {
	
	export class Issue {
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class Priority {
	    value: number;
	    kind: string;
	    raw: string;
	    trailingNines: number;
	
	    static createFrom(source: any = {}) {
	        return new Priority(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.kind = source["kind"];
	        this.raw = source["raw"];
	        this.trailingNines = source["trailingNines"];
	    }
	}
	export class Sidecars {
	    utoc?: string;
	    ucas?: string;
	
	    static createFrom(source: any = {}) {
	        return new Sidecars(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.utoc = source["utoc"];
	        this.ucas = source["ucas"];
	    }
	}
	export class Entry {
	    id: string;
	    primaryPath?: string;
	    relativeFolder: string;
	    displayName: string;
	    state: string;
	    disabledFormat?: string;
	    kind: string;
	    bundleFormat?: string;
	    sidecars: Sidecars;
	    priority: Priority;
	    issues?: Issue[];
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.primaryPath = source["primaryPath"];
	        this.relativeFolder = source["relativeFolder"];
	        this.displayName = source["displayName"];
	        this.state = source["state"];
	        this.disabledFormat = source["disabledFormat"];
	        this.kind = source["kind"];
	        this.bundleFormat = source["bundleFormat"];
	        this.sidecars = this.convertValues(source["sidecars"], Sidecars);
	        this.priority = this.convertValues(source["priority"], Priority);
	        this.issues = this.convertValues(source["issues"], Issue);
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
	
	export class Library {
	    root: string;
	    entries: Entry[];
	
	    static createFrom(source: any = {}) {
	        return new Library(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
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
	

}

export namespace mutation {
	
	export class Result {
	    id: string;
	    previousPrimaryPath: string;
	    primaryPath: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.previousPrimaryPath = source["previousPrimaryPath"];
	        this.primaryPath = source["primaryPath"];
	        this.state = source["state"];
	    }
	}

}

