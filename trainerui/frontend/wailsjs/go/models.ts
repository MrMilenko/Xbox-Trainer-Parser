export namespace parser {
	
	export class Trainer {
	    isXBTF: boolean;
	    path: string;
	    creationKey: string;
	    scroller: string;
	    name: string;
	    labels: string[];
	    titleIDs: number[];
	    textStart: number;
	    optStart: number;
	    iconURL: string;
	
	    static createFrom(source: any = {}) {
	        return new Trainer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isXBTF = source["isXBTF"];
	        this.path = source["path"];
	        this.creationKey = source["creationKey"];
	        this.scroller = source["scroller"];
	        this.name = source["name"];
	        this.labels = source["labels"];
	        this.titleIDs = source["titleIDs"];
	        this.textStart = source["textStart"];
	        this.optStart = source["optStart"];
	        this.iconURL = source["iconURL"];
	    }
	}

}

