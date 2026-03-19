export namespace chatlog {
	
	export class ChatStats {
	    input_tokens: number;
	    output_tokens: number;
	    tokens_per_sec: number;
	    time_to_first_sec: number;
	    response_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.tokens_per_sec = source["tokens_per_sec"];
	        this.time_to_first_sec = source["time_to_first_sec"];
	        this.response_id = source["response_id"];
	    }
	}
	export class ChatEvent {
	    type: string;
	    session_id: string;
	    // Go type: time
	    ts: any;
	    content?: string;
	    stats?: ChatStats;
	    tool?: string;
	    arguments?: string;
	    phase?: string;
	    progress?: number;
	    success?: boolean;
	    output?: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.session_id = source["session_id"];
	        this.ts = this.convertValues(source["ts"], null);
	        this.content = source["content"];
	        this.stats = this.convertValues(source["stats"], ChatStats);
	        this.tool = source["tool"];
	        this.arguments = source["arguments"];
	        this.phase = source["phase"];
	        this.progress = source["progress"];
	        this.success = source["success"];
	        this.output = source["output"];
	        this.reason = source["reason"];
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
	
	export class IntegrationDTO {
	    label: string;
	    description: string;
	    type: string;
	    id?: string;
	    server_label?: string;
	    server_url?: string;
	    allowed_tools?: string[];
	    headers?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new IntegrationDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.id = source["id"];
	        this.server_label = source["server_label"];
	        this.server_url = source["server_url"];
	        this.allowed_tools = source["allowed_tools"];
	        this.headers = source["headers"];
	    }
	}
	export class ProfileDTO {
	    label: string;
	    description: string;
	    system_prompt: string;
	    model: string;
	    temperature: number;
	    context_length: number;
	    top_p: number;
	    top_k: number;
	    min_p: number;
	    repeat_penalty: number;
	    max_output_tokens: number;
	    reasoning: string;
	    integrations: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProfileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.description = source["description"];
	        this.system_prompt = source["system_prompt"];
	        this.model = source["model"];
	        this.temperature = source["temperature"];
	        this.context_length = source["context_length"];
	        this.top_p = source["top_p"];
	        this.top_k = source["top_k"];
	        this.min_p = source["min_p"];
	        this.repeat_penalty = source["repeat_penalty"];
	        this.max_output_tokens = source["max_output_tokens"];
	        this.reasoning = source["reasoning"];
	        this.integrations = source["integrations"];
	    }
	}
	export class ConfigDTO {
	    shared_system_prompt: string;
	    profiles: Record<string, ProfileDTO>;
	    integrations: Record<string, IntegrationDTO>;
	
	    static createFrom(source: any = {}) {
	        return new ConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shared_system_prompt = source["shared_system_prompt"];
	        this.profiles = this.convertValues(source["profiles"], ProfileDTO, true);
	        this.integrations = this.convertValues(source["integrations"], IntegrationDTO, true);
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
	
	
	export class SessionDTO {
	    id: string;
	    task: string;
	    profile: string;
	    model: string;
	    status: string;
	    tokensUsed: number;
	    tokensMax: number;
	    tokensPercent: number;
	    exchanges: number;
	    integrationKeys: string[];
	    createdAt: string;
	    lastActiveAt: string;
	    hasChatLog: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SessionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.task = source["task"];
	        this.profile = source["profile"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.tokensUsed = source["tokensUsed"];
	        this.tokensMax = source["tokensMax"];
	        this.tokensPercent = source["tokensPercent"];
	        this.exchanges = source["exchanges"];
	        this.integrationKeys = source["integrationKeys"];
	        this.createdAt = source["createdAt"];
	        this.lastActiveAt = source["lastActiveAt"];
	        this.hasChatLog = source["hasChatLog"];
	    }
	}
	export class SettingsDTO {
	    apiBase: string;
	    apiToken: string;
	    model: string;
	    contextLength: number;
	    requestTimeout: number;
	    maxSessionTokens: number;
	    tokenWarningThreshold: number;
	    tokenCriticalThreshold: number;
	    sessionsDir: string;
	    progressDir: string;
	    chatlogDir: string;
	    configFile: string;
	    logFile: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiBase = source["apiBase"];
	        this.apiToken = source["apiToken"];
	        this.model = source["model"];
	        this.contextLength = source["contextLength"];
	        this.requestTimeout = source["requestTimeout"];
	        this.maxSessionTokens = source["maxSessionTokens"];
	        this.tokenWarningThreshold = source["tokenWarningThreshold"];
	        this.tokenCriticalThreshold = source["tokenCriticalThreshold"];
	        this.sessionsDir = source["sessionsDir"];
	        this.progressDir = source["progressDir"];
	        this.chatlogDir = source["chatlogDir"];
	        this.configFile = source["configFile"];
	        this.logFile = source["logFile"];
	    }
	}

}

