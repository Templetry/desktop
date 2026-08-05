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

