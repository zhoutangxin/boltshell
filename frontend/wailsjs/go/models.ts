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

