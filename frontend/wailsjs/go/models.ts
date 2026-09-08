export namespace conflict {
	
	export class Participant {
	    entryID: string;
	    displayName: string;
	    priority: discovery.Priority;
	    overlappingPaths: string[];
	
	    static createFrom(source: any = {}) {
	        return new Participant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entryID = source["entryID"];
	        this.displayName = source["displayName"];
	        this.priority = this.convertValues(source["priority"], discovery.Priority);
	        this.overlappingPaths = source["overlappingPaths"];
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
	export class Group {
	    participants: Participant[];
	    relationship: string;
	    pathCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.participants = this.convertValues(source["participants"], Participant);
	        this.relationship = source["relationship"];
	        this.pathCount = source["pathCount"];
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
	
	export class Result {
	    groups: Group[];
	    unavailable: string[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], Group);
	        this.unavailable = source["unavailable"];
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

export namespace gamedetect {
	
	export class Detection {
	    state: string;
	    libraryPath?: string;
	    paksPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new Detection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.libraryPath = source["libraryPath"];
	        this.paksPath = source["paksPath"];
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
	    unsupportedCompanionPak?: boolean;
	
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
	        this.unsupportedCompanionPak = source["unsupportedCompanionPak"];
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
	export class NexusAccountState {
	    configured: boolean;
	    verified: boolean;
	    name?: string;
	    isPremium: boolean;
	    hourlyRemaining: number;
	    dailyRemaining: number;
	    hourlyResetUnix: number;
	    dailyResetUnix: number;
	
	    static createFrom(source: any = {}) {
	        return new NexusAccountState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.verified = source["verified"];
	        this.name = source["name"];
	        this.isPremium = source["isPremium"];
	        this.hourlyRemaining = source["hourlyRemaining"];
	        this.dailyRemaining = source["dailyRemaining"];
	        this.hourlyResetUnix = source["hourlyResetUnix"];
	        this.dailyResetUnix = source["dailyResetUnix"];
	    }
	}
	export class NexusFileSummary {
	    fileId: number;
	    name: string;
	    fileName: string;
	    version: string;
	    sizeKb: number;
	    categoryName: string;
	    isPrimary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NexusFileSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileId = source["fileId"];
	        this.name = source["name"];
	        this.fileName = source["fileName"];
	        this.version = source["version"];
	        this.sizeKb = source["sizeKb"];
	        this.categoryName = source["categoryName"];
	        this.isPrimary = source["isPrimary"];
	    }
	}
	export class NexusKeyState {
	    configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NexusKeyState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	    }
	}
	export class NexusLink {
	    present: boolean;
	    game?: string;
	    modId?: number;
	    fileId?: number;
	    modName?: string;
	    author?: string;
	    pictureUrl?: string;
	    fileName?: string;
	    version?: string;
	    sizeKb?: number;
	    premium: boolean;
	    needsWebsite: boolean;
	    files?: NexusFileSummary[];
	
	    static createFrom(source: any = {}) {
	        return new NexusLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.present = source["present"];
	        this.game = source["game"];
	        this.modId = source["modId"];
	        this.fileId = source["fileId"];
	        this.modName = source["modName"];
	        this.author = source["author"];
	        this.pictureUrl = source["pictureUrl"];
	        this.fileName = source["fileName"];
	        this.version = source["version"];
	        this.sizeKb = source["sizeKb"];
	        this.premium = source["premium"];
	        this.needsWebsite = source["needsWebsite"];
	        this.files = this.convertValues(source["files"], NexusFileSummary);
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
	export class NexusProtocolState {
	    ownership: string;
	    ownerName?: string;
	    ownerPath?: string;
	    userChoice: boolean;
	    machineWide: boolean;
	    canRegister: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NexusProtocolState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ownership = source["ownership"];
	        this.ownerName = source["ownerName"];
	        this.ownerPath = source["ownerPath"];
	        this.userChoice = source["userChoice"];
	        this.machineWide = source["machineWide"];
	        this.canRegister = source["canRegister"];
	        this.enabled = source["enabled"];
	    }
	}
	export class UpdateCheckResult {
	    available: boolean;
	    release?: update.Release;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.release = this.convertValues(source["release"], update.Release);
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
	    nexusModId?: number;
	    nexusFileId?: number;
	    nexusVersion?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scannerID = source["scannerID"];
	        this.tags = source["tags"];
	        this.nexusModId = source["nexusModId"];
	        this.nexusFileId = source["nexusFileId"];
	        this.nexusVersion = source["nexusVersion"];
	    }
	}
	export class NexusProtocolSnapshot {
	    command?: string;
	    icon?: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new NexusProtocolSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	    }
	}
	export class Settings {
	    modRoot?: string;
	    theme?: string;
	    defaultViewMode?: string;
	    accentColor?: string;
	    libraryProvider?: string;
	    lastSeenVersion?: string;
	    nexusProtocol?: NexusProtocolSnapshot;
	    nexusProtocolOptOut?: boolean;
	    skipDestructiveDelay?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modRoot = source["modRoot"];
	        this.theme = source["theme"];
	        this.defaultViewMode = source["defaultViewMode"];
	        this.accentColor = source["accentColor"];
	        this.libraryProvider = source["libraryProvider"];
	        this.lastSeenVersion = source["lastSeenVersion"];
	        this.nexusProtocol = this.convertValues(source["nexusProtocol"], NexusProtocolSnapshot);
	        this.nexusProtocolOptOut = source["nexusProtocolOptOut"];
	        this.skipDestructiveDelay = source["skipDestructiveDelay"];
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
	    encrypted: boolean;
	
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
	        this.encrypted = source["encrypted"];
	    }
	}

}

export namespace mutation {
	
	export class CompanionCleanupFailure {
	    entryID: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CompanionCleanupFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entryID = source["entryID"];
	        this.message = source["message"];
	    }
	}
	export class CompanionCleanupResult {
	    succeeded: string[];
	    failed: CompanionCleanupFailure[];
	
	    static createFrom(source: any = {}) {
	        return new CompanionCleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.succeeded = source["succeeded"];
	        this.failed = this.convertValues(source["failed"], CompanionCleanupFailure);
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
	export class EncryptionFailure {
	    entryID: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new EncryptionFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entryID = source["entryID"];
	        this.message = source["message"];
	    }
	}
	export class EncryptionBatchResult {
	    succeeded: string[];
	    failed: EncryptionFailure[];
	
	    static createFrom(source: any = {}) {
	        return new EncryptionBatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.succeeded = source["succeeded"];
	        this.failed = this.convertValues(source["failed"], EncryptionFailure);
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

export namespace nexus {
	
	export class DownloadRequest {
	    game: string;
	    modId: number;
	    fileId: number;
	    premium: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game = source["game"];
	        this.modId = source["modId"];
	        this.fileId = source["fileId"];
	        this.premium = source["premium"];
	    }
	}

}

export namespace update {
	
	export class ReleaseAsset {
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new ReleaseAsset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class Version {
	    tag: string;
	    year: number;
	    month: number;
	    day: number;
	    prerelease?: string;
	
	    static createFrom(source: any = {}) {
	        return new Version(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag = source["tag"];
	        this.year = source["year"];
	        this.month = source["month"];
	        this.day = source["day"];
	        this.prerelease = source["prerelease"];
	    }
	}
	export class Release {
	    version: Version;
	    htmlURL: string;
	    notes: string;
	    asset: ReleaseAsset;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = this.convertValues(source["version"], Version);
	        this.htmlURL = source["htmlURL"];
	        this.notes = source["notes"];
	        this.asset = this.convertValues(source["asset"], ReleaseAsset);
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

