export namespace db {
	
	export class Connection {
	    ID: string;
	    Name: string;
	    Host: string;
	    Port: number;
	    User: string;
	    Password: string;
	    GroupID: string;
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
	        this.GroupID = source["GroupID"];
	        this.GroupName = source["GroupName"];
	        this.Enabled = source["Enabled"];
	        this.Deleted = source["Deleted"];
	        this.CreatedAt = source["CreatedAt"];
	    }
	}
	export class ConnectionGroup {
	    ID: string;
	    Name: string;
	    Status: string;
	    CreatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Status = source["Status"];
	        this.CreatedAt = source["CreatedAt"];
	    }
	}

}

export namespace main {
	
	export class ConnectionsImportResult {
	    GroupsAdded: number;
	    ConnectionsAdded: number;
	    ConnectionsSkip: number;
	    ConnectionsUpdated: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionsImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.GroupsAdded = source["GroupsAdded"];
	        this.ConnectionsAdded = source["ConnectionsAdded"];
	        this.ConnectionsSkip = source["ConnectionsSkip"];
	        this.ConnectionsUpdated = source["ConnectionsUpdated"];
	    }
	}
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
	export class SponsorSlotView {
	    SlotID: string;
	    enabled: boolean;
	    type: string;
	    badge: string;
	    title: string;
	    desc: string;
	    linkUrl: string;
	    imageUrl?: string;
	    dismissDays?: number;
	
	    static createFrom(source: any = {}) {
	        return new SponsorSlotView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SlotID = source["SlotID"];
	        this.enabled = source["enabled"];
	        this.type = source["type"];
	        this.badge = source["badge"];
	        this.title = source["title"];
	        this.desc = source["desc"];
	        this.linkUrl = source["linkUrl"];
	        this.imageUrl = source["imageUrl"];
	        this.dismissDays = source["dismissDays"];
	    }
	}
	export class SponsorConfigView {
	    Version: number;
	    UpdatedAt: string;
	    CacheTTLSeconds: number;
	    ProUpgradeURL: string;
	    IsPro: boolean;
	    Slots: SponsorSlotView[];
	    DismissedUntil: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new SponsorConfigView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Version = source["Version"];
	        this.UpdatedAt = source["UpdatedAt"];
	        this.CacheTTLSeconds = source["CacheTTLSeconds"];
	        this.ProUpgradeURL = source["ProUpgradeURL"];
	        this.IsPro = source["IsPro"];
	        this.Slots = this.convertValues(source["Slots"], SponsorSlotView);
	        this.DismissedUntil = source["DismissedUntil"];
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

export namespace updater {
	
	export class CheckResult {
	    CurrentVersion: string;
	    LatestVersion: string;
	    HasUpdate: boolean;
	    ReleaseNotes: string;
	    DownloadURL: string;
	    Mandatory: boolean;
	    PublishedAt: string;
	    CheckError?: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CurrentVersion = source["CurrentVersion"];
	        this.LatestVersion = source["LatestVersion"];
	        this.HasUpdate = source["HasUpdate"];
	        this.ReleaseNotes = source["ReleaseNotes"];
	        this.DownloadURL = source["DownloadURL"];
	        this.Mandatory = source["Mandatory"];
	        this.PublishedAt = source["PublishedAt"];
	        this.CheckError = source["CheckError"];
	    }
	}

}

