export namespace config {
	
	export class KnownPeer {
	    hostname: string;
	    display_name: string;
	    addr: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new KnownPeer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.display_name = source["display_name"];
	        this.addr = source["addr"];
	        this.port = source["port"];
	    }
	}
	export class Subscription {
	    peer_hostname: string;
	    remote_folder: string;
	    folder_id: string;
	    local_dest: string;
	    last_synced_at: number;
	
	    static createFrom(source: any = {}) {
	        return new Subscription(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peer_hostname = source["peer_hostname"];
	        this.remote_folder = source["remote_folder"];
	        this.folder_id = source["folder_id"];
	        this.local_dest = source["local_dest"];
	        this.last_synced_at = source["last_synced_at"];
	    }
	}
	export class Folder {
	    id: string;
	    path: string;
	    disabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.disabled = source["disabled"];
	    }
	}
	export class Config {
	    device_id: string;
	    display_name: string;
	    port: number;
	    base_storage: string;
	    folders: Folder[];
	    subscriptions: Subscription[];
	    known_peers: KnownPeer[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.display_name = source["display_name"];
	        this.port = source["port"];
	        this.base_storage = source["base_storage"];
	        this.folders = this.convertValues(source["folders"], Folder);
	        this.subscriptions = this.convertValues(source["subscriptions"], Subscription);
	        this.known_peers = this.convertValues(source["known_peers"], KnownPeer);
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
	
	export class Peer {
	    Hostname: string;
	    DisplayName: string;
	    Addr: string;
	    Port: number;
	
	    static createFrom(source: any = {}) {
	        return new Peer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Hostname = source["Hostname"];
	        this.DisplayName = source["DisplayName"];
	        this.Addr = source["Addr"];
	        this.Port = source["Port"];
	    }
	}

}

