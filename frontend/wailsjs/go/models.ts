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
	    folders: string[];
	    entries: Entry[];
	
	    static createFrom(source: any = {}) {
	        return new Library(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.folders = source["folders"];
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

export namespace install {
	
	export class ApplyItem {
	    id: string;
	    modName: string;
	    destinationFolder: string;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ApplyItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.modName = source["modName"];
	        this.destinationFolder = source["destinationFolder"];
	        this.overwrite = source["overwrite"];
	    }
	}
	export class ApplyResult {
	    installedEntryIDs: string[];
	    reconciledLibrary: discovery.Library;
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installedEntryIDs = source["installedEntryIDs"];
	        this.reconciledLibrary = this.convertValues(source["reconciledLibrary"], discovery.Library);
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
	export class CollisionInfo {
	    hasCollision: boolean;
	    existingModID?: string;
	    collidingFiles?: string[];
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new CollisionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasCollision = source["hasCollision"];
	        this.existingModID = source["existingModID"];
	        this.collidingFiles = source["collidingFiles"];
	        this.description = source["description"];
	    }
	}
	export class PreviewItem {
	    id: string;
	    modName: string;
	    originalStem: string;
	    sourcePath: string;
	    bundleFormat: string;
	    files: string[];
	    totalSizeBytes: number;
	    destinationFolder: string;
	    collision: CollisionInfo;
	    identity: modtype.Identity;
	    issues?: discovery.Issue[];
	
	    static createFrom(source: any = {}) {
	        return new PreviewItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.modName = source["modName"];
	        this.originalStem = source["originalStem"];
	        this.sourcePath = source["sourcePath"];
	        this.bundleFormat = source["bundleFormat"];
	        this.files = source["files"];
	        this.totalSizeBytes = source["totalSizeBytes"];
	        this.destinationFolder = source["destinationFolder"];
	        this.collision = this.convertValues(source["collision"], CollisionInfo);
	        this.identity = this.convertValues(source["identity"], modtype.Identity);
	        this.issues = this.convertValues(source["issues"], discovery.Issue);
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
	export class PreviewResult {
	    sessionId: string;
	    items: PreviewItem[];
	
	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.items = this.convertValues(source["items"], PreviewItem);
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

export namespace main {
	
	export class MetadataState {
	    document: metadata.Document;
	    recovered: boolean;
	    recoveryReason?: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.document = this.convertValues(source["document"], metadata.Document);
	        this.recovered = source["recovered"];
	        this.recoveryReason = source["recoveryReason"];
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

export namespace metadata {
	
	export class Tag {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class ModRecord {
	    scannerID: string;
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ModRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scannerID = source["scannerID"];
	        this.tags = source["tags"];
	    }
	}
	export class Settings {
	    modRoot?: string;
	    theme?: string;
	    defaultViewMode?: string;
	    accentColor?: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modRoot = source["modRoot"];
	        this.theme = source["theme"];
	        this.defaultViewMode = source["defaultViewMode"];
	        this.accentColor = source["accentColor"];
	    }
	}
	export class Document {
	    schemaVersion: number;
	    settings: Settings;
	    mods?: Record<string, ModRecord>;
	    tags?: Tag[];
	
	    static createFrom(source: any = {}) {
	        return new Document(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.mods = this.convertValues(source["mods"], ModRecord, true);
	        this.tags = this.convertValues(source["tags"], Tag);
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

export namespace modtype {
	
	export class Identity {
	    category: string;
	    characterID: string;
	    characterName: string;
	    skinID: string;
	    skinName: string;
	
	    static createFrom(source: any = {}) {
	        return new Identity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.characterID = source["characterID"];
	        this.characterName = source["characterName"];
	        this.skinID = source["skinID"];
	        this.skinName = source["skinName"];
	    }
	}

}

export namespace mutation {
	
	export class Result {
	    id: string;
	    previousID?: string;
	    previousPrimaryPath: string;
	    primaryPath: string;
	    previousFolderPath?: string;
	    folderPath?: string;
	    deleted?: boolean;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.previousID = source["previousID"];
	        this.previousPrimaryPath = source["previousPrimaryPath"];
	        this.primaryPath = source["primaryPath"];
	        this.previousFolderPath = source["previousFolderPath"];
	        this.folderPath = source["folderPath"];
	        this.deleted = source["deleted"];
	        this.state = source["state"];
	    }
	}

}

