export namespace db {
	
	export class Connection {
	    ID: string;
	    Name: string;
	    Host: string;
	    Port: number;
	    User: string;
	    Password: string;
	    GroupName: string;
	    Enabled: number;
	    Deleted: number;
	    CreatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Host = source["Host"];
	        this.Port = source["Port"];
	        this.User = source["User"];
	        this.Password = source["Password"];
	        this.GroupName = source["GroupName"];
	        this.Enabled = source["Enabled"];
	        this.Deleted = source["Deleted"];
	        this.CreatedAt = source["CreatedAt"];
	    }
	}

}

export namespace main {
	
	export class SessionInfo {
	    ID: string;
	    Title: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Title = source["Title"];
	    }
	}
	export class SysProcInfo {
	    MemKB: number;
	    CPUPct: number;
	    Command: string;
	
	    static createFrom(source: any = {}) {
	        return new SysProcInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MemKB = source["MemKB"];
	        this.CPUPct = source["CPUPct"];
	        this.Command = source["Command"];
	    }
	}
	export class SysInfo {
	    CPUPercent: number;
	    MemTotal: number;
	    MemUsed: number;
	    SwapTotal: number;
	    SwapUsed: number;
	    DiskTotal: number;
	    DiskUsed: number;
	    DiskFree: number;
	    DiskPath: string;
	    Processes: SysProcInfo[];
	
	    static createFrom(source: any = {}) {
	        return new SysInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CPUPercent = source["CPUPercent"];
	        this.MemTotal = source["MemTotal"];
	        this.MemUsed = source["MemUsed"];
	        this.SwapTotal = source["SwapTotal"];
	        this.SwapUsed = source["SwapUsed"];
	        this.DiskTotal = source["DiskTotal"];
	        this.DiskUsed = source["DiskUsed"];
	        this.DiskFree = source["DiskFree"];
	        this.DiskPath = source["DiskPath"];
	        this.Processes = this.convertValues(source["Processes"], SysProcInfo);
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

export namespace sftpclient {
	
	export class RemoteEntry {
	    Name: string;
	    Path: string;
	    Size: number;
	    IsDir: boolean;
	    ModTime: number;
	    Mode: string;
	    Owner: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Path = source["Path"];
	        this.Size = source["Size"];
	        this.IsDir = source["IsDir"];
	        this.ModTime = source["ModTime"];
	        this.Mode = source["Mode"];
	        this.Owner = source["Owner"];
	    }
	}

}

