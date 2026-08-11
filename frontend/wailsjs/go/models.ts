export namespace catalog {
	
	export class Form {
	    form: string;
	    name: string;
	    path: string;
	    status: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new Form(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.form = source["form"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.status = source["status"];
	        this.description = source["description"];
	    }
	}
	export class Parent {
	    key: string;
	    label?: string;
	    repo: string;
	    ref: string;
	    forms: Form[];
	
	    static createFrom(source: any = {}) {
	        return new Parent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.repo = source["repo"];
	        this.ref = source["ref"];
	        this.forms = this.convertValues(source["forms"], Form);
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
	
	export class AuthStatus {
	    state: string;
	    login?: string;
	    avatar?: string;
	    userCode?: string;
	    verificationUri?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.login = source["login"];
	        this.avatar = source["avatar"];
	        this.userCode = source["userCode"];
	        this.verificationUri = source["verificationUri"];
	        this.error = source["error"];
	    }
	}
	export class CIRun {
	    name: string;
	    branch: string;
	    status: string;
	    conclusion: string;
	    url: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CIRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.branch = source["branch"];
	        this.status = source["status"];
	        this.conclusion = source["conclusion"];
	        this.url = source["url"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class CatalogEntry {
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class CreatedProject {
	    url: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new CreatedProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.dir = source["dir"];
	    }
	}
	export class Drift {
	    dir: string;
	    latest: string;
	
	    static createFrom(source: any = {}) {
	        return new Drift(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.latest = source["latest"];
	    }
	}
	export class LangShare {
	    name: string;
	    pct: number;
	
	    static createFrom(source: any = {}) {
	        return new LangShare(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pct = source["pct"];
	    }
	}
	export class LoadedCatalog {
	    name: string;
	    official: boolean;
	    error?: string;
	    parents?: catalog.Parent[];
	
	    static createFrom(source: any = {}) {
	        return new LoadedCatalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.official = source["official"];
	        this.error = source["error"];
	        this.parents = this.convertValues(source["parents"], catalog.Parent);
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
	export class LocalRemote {
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalRemote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class LocalOverview {
	    branch: string;
	    branches: string[];
	    remotes: LocalRemote[];
	    lastCommit: string;
	    changes: number;
	    docs: string[];
	
	    static createFrom(source: any = {}) {
	        return new LocalOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.branches = source["branches"];
	        this.remotes = this.convertValues(source["remotes"], LocalRemote);
	        this.lastCommit = source["lastCommit"];
	        this.changes = source["changes"];
	        this.docs = source["docs"];
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
	export class LocalProject {
	    dir: string;
	    name: string;
	    rel: string;
	    kind: string;
	    remote?: string;
	    branch?: string;
	    template?: string;
	    source?: string;
	    commit?: string;
	    variables?: Record<string, string>;
	    features?: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new LocalProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.name = source["name"];
	        this.rel = source["rel"];
	        this.kind = source["kind"];
	        this.remote = source["remote"];
	        this.branch = source["branch"];
	        this.template = source["template"];
	        this.source = source["source"];
	        this.commit = source["commit"];
	        this.variables = source["variables"];
	        this.features = source["features"];
	    }
	}
	
	export class PreviewEntry {
	    path: string;
	    binary: boolean;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new PreviewEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.binary = source["binary"];
	        this.size = source["size"];
	    }
	}
	export class Repo {
	    name: string;
	    fullName: string;
	    owner: string;
	    description: string;
	    htmlUrl: string;
	    cloneUrl: string;
	    private: boolean;
	    language: string;
	    updatedAt: string;
	    archived: boolean;
	    avatarUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Repo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.owner = source["owner"];
	        this.description = source["description"];
	        this.htmlUrl = source["htmlUrl"];
	        this.cloneUrl = source["cloneUrl"];
	        this.private = source["private"];
	        this.language = source["language"];
	        this.updatedAt = source["updatedAt"];
	        this.archived = source["archived"];
	        this.avatarUrl = source["avatarUrl"];
	    }
	}
	export class RepoOverview {
	    description: string;
	    defaultBranch: string;
	    languages: LangShare[];
	    branches: string[];
	    runs: CIRun[];
	    docs: string[];
	    templateForms: string[];
	
	    static createFrom(source: any = {}) {
	        return new RepoOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.description = source["description"];
	        this.defaultBranch = source["defaultBranch"];
	        this.languages = this.convertValues(source["languages"], LangShare);
	        this.branches = source["branches"];
	        this.runs = this.convertValues(source["runs"], CIRun);
	        this.docs = source["docs"];
	        this.templateForms = source["templateForms"];
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
	export class UpdateInfo {
	    appLatest: string;
	    engineLatest: string;
	    appUpdate: boolean;
	    engineUpdate: boolean;
	    appUrl: string;
	    engineUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appLatest = source["appLatest"];
	        this.engineLatest = source["engineLatest"];
	        this.appUpdate = source["appUpdate"];
	        this.engineUpdate = source["engineUpdate"];
	        this.appUrl = source["appUrl"];
	        this.engineUrl = source["engineUrl"];
	    }
	}
	export class VersionInfo {
	    app: string;
	    engine: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app = source["app"];
	        this.engine = source["engine"];
	    }
	}
	export class appConfig {
	    lastParentDir: string;
	    defaultParentDir: string;
	    defaultOwner: string;
	    defaultPrivate: boolean;
	    defaultLicense: string;
	    registryUrl: string;
	    uiTheme: string;
	    uiAccent: string;
	    uiDensity: string;
	    uiScale: string;
	    uiLayout: string;
	    catalogs: CatalogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new appConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastParentDir = source["lastParentDir"];
	        this.defaultParentDir = source["defaultParentDir"];
	        this.defaultOwner = source["defaultOwner"];
	        this.defaultPrivate = source["defaultPrivate"];
	        this.defaultLicense = source["defaultLicense"];
	        this.registryUrl = source["registryUrl"];
	        this.uiTheme = source["uiTheme"];
	        this.uiAccent = source["uiAccent"];
	        this.uiDensity = source["uiDensity"];
	        this.uiScale = source["uiScale"];
	        this.uiLayout = source["uiLayout"];
	        this.catalogs = this.convertValues(source["catalogs"], CatalogEntry);
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

export namespace manifest {
	
	export class Patch {
	    file: string;
	    op: string;
	    path: string;
	    value?: any;
	
	    static createFrom(source: any = {}) {
	        return new Patch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.op = source["op"];
	        this.path = source["path"];
	        this.value = source["value"];
	    }
	}
	export class Feature {
	    key: string;
	    label?: string;
	    default?: boolean;
	    files?: string[];
	    patches?: Patch[];
	    requires?: string[];
	    conflicts?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Feature(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.default = source["default"];
	        this.files = source["files"];
	        this.patches = this.convertValues(source["patches"], Patch);
	        this.requires = source["requires"];
	        this.conflicts = source["conflicts"];
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
	export class Verify {
	    image: string;
	    run: string;
	
	    static createFrom(source: any = {}) {
	        return new Verify(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.image = source["image"];
	        this.run = source["run"];
	    }
	}
	export class Preset {
	    key: string;
	    label?: string;
	    features: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.features = source["features"];
	    }
	}
	export class Rename {
	    from: string;
	    to: string;
	
	    static createFrom(source: any = {}) {
	        return new Rename(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class Variable {
	    key: string;
	    label?: string;
	    type?: string;
	    pattern?: string;
	    options?: string[];
	    default?: string;
	
	    static createFrom(source: any = {}) {
	        return new Variable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.pattern = source["pattern"];
	        this.options = source["options"];
	        this.default = source["default"];
	    }
	}
	export class Manifest {
	    schema_version: number;
	    name: string;
	    description?: string;
	    platform?: string;
	    framework?: string;
	    variables?: Variable[];
	    identity?: Rename[];
	    features?: Feature[];
	    presets?: Preset[];
	    verify?: Verify;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.platform = source["platform"];
	        this.framework = source["framework"];
	        this.variables = this.convertValues(source["variables"], Variable);
	        this.identity = this.convertValues(source["identity"], Rename);
	        this.features = this.convertValues(source["features"], Feature);
	        this.presets = this.convertValues(source["presets"], Preset);
	        this.verify = this.convertValues(source["verify"], Verify);
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

export namespace update {
	
	export class Entry {
	    path: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	    }
	}
	export class Preview {
	    dir: string;
	    template: string;
	    oldCommit: string;
	    newCommit: string;
	    entries: Entry[];
	    unchanged: number;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.template = source["template"];
	        this.oldCommit = source["oldCommit"];
	        this.newCommit = source["newCommit"];
	        this.entries = this.convertValues(source["entries"], Entry);
	        this.unchanged = source["unchanged"];
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

