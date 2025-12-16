import {
  AgentConfig,
  SubAgentInfo,
  Tool,
  MemoryEntry,
  MemoryResponse,
  RunRequest,
  StreamRequest,
  RunResponse,
  StreamResponse,
  StreamEventData,
  UITrace,
  TracesResponse,
  TraceStats
} from '@/types/agent';

const API_BASE_URL = '/api/v1';

class AgentAPI {
  private baseUrl: string;
  private authToken: string | null = null;

  constructor(baseUrl = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  // 设置/清除当前使用的认证 token
  setAuthToken(token: string | null) {
    this.authToken = token;
  }

  // 为请求构建通用 Header（自动附加 Authorization）
  private buildHeaders(extra?: HeadersInit): HeadersInit {
    const headers: HeadersInit = {
      ...(extra || {}),
    };
    if (this.authToken) {
      (headers as Record<string, string>)['Authorization'] = `Bearer ${this.authToken}`;
    }
    return headers;
  }

  async get<T>(endpoint: string): Promise<T> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      headers: this.buildHeaders(),
    });
    if (!response.ok) {
      throw new Error(`API Error: ${response.status} ${response.statusText}`);
    }
    return response.json();
  }

  async post<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'POST',
      headers: this.buildHeaders({
        'Content-Type': 'application/json',
      }),
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      throw new Error(`API Error: ${response.status} ${response.statusText}`);
    }
    return response.json();
  }

  // Agent configuration endpoints
  async getAgentConfig(): Promise<AgentConfig> {
    return this.get<AgentConfig>('/agent/config');
  }

  async getAgentMetadata(): Promise<AgentConfig> {
    return this.get<AgentConfig>('/agent/metadata');
  }

  // Sub-agents endpoints
  async getSubAgents(): Promise<{ sub_agents: SubAgentInfo[]; count: number }> {
    return this.get<{ sub_agents: SubAgentInfo[]; count: number }>('/agent/subagents');
  }

  async getSubAgent(id: string): Promise<SubAgentInfo> {
    return this.get<SubAgentInfo>(`/agent/subagents/${id}`);
  }

  async delegateToSubAgent(data: {
    sub_agent_id: string;
    task: string;
    context?: Record<string, string>;
    conversation_id?: string;
  }): Promise<{
    status: string;
    sub_agent_id: string;
    task: string;
    result: string;
  }> {
    return this.post('/agent/delegate', data);
  }

  // Tools endpoints
  async getTools(): Promise<{ tools: Tool[]; count: number }> {
    return this.get<{ tools: Tool[]; count: number }>('/tools');
  }

  // Memory endpoints
  async getMemory(limit = 100, offset = 0): Promise<MemoryResponse> {
    return this.get(`/memory?limit=${limit}&offset=${offset}`);
  }

  async getConversationMessages(conversationId: string, limit = 100, offset = 0): Promise<MemoryResponse> {
    return this.get(`/memory?conversation_id=${conversationId}&limit=${limit}&offset=${offset}`);
  }

  async searchMemory(query: string): Promise<{
    query: string;
    results: MemoryEntry[];
    count: number;
  }> {
    return this.get(`/memory/search?q=${encodeURIComponent(query)}`);
  }

  // Chat endpoints
  async runAgent(data: RunRequest): Promise<RunResponse> {
    return this.post<RunResponse>('/agent/run', data);
  }

  // Streaming chat with SSE
  async *streamAgent(data: StreamRequest): AsyncGenerator<StreamEventData, void, unknown> {
    const response = await fetch(`${this.baseUrl}/agent/stream`, {
      method: 'POST',
      headers: this.buildHeaders({
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
      }),
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      throw new Error(`Stream Error: ${response.status} ${response.statusText}`);
    }

    const reader = response.body?.getReader();
    const decoder = new TextDecoder();

    if (!reader) {
      throw new Error('No response body reader available');
    }

    try {
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');

        // Keep the last line in buffer as it might be incomplete
        buffer = lines.pop() || '';

        let currentEvent: Partial<StreamResponse> = {};

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent.event = line.slice(7);
          } else if (line.startsWith('data: ')) {
            const dataStr = line.slice(6);
            if (dataStr === '[DONE]') {
              return;
            }
            try {
              currentEvent.data = JSON.parse(dataStr) as StreamEventData;
            } catch {
              console.warn('Failed to parse SSE data:', dataStr);
            }
          } else if (line.startsWith('id: ')) {
            currentEvent.id = line.slice(4);
          } else if (line === '') {
            // End of event, yield if we have data
            if (currentEvent.data) {
              yield currentEvent.data;
              currentEvent = {};
            }
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  // Health check
  async health(): Promise<{ status: string; timestamp: number }> {
    const response = await fetch('/health');
    if (!response.ok) {
      throw new Error(`Health check failed: ${response.status}`);
    }
    return response.json();
  }

  // Trace endpoints
  async getTraces(limit = 50, offset = 0): Promise<TracesResponse> {
    return this.get<TracesResponse>(`/traces?limit=${limit}&offset=${offset}`);
  }

  async getTrace(id: string): Promise<UITrace> {
    return this.get<UITrace>(`/traces/${id}`);
  }

  async deleteTrace(id: string): Promise<{ status: string; id: string }> {
    const response = await fetch(`${this.baseUrl}/traces/${id}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      throw new Error(`Delete trace failed: ${response.status} ${response.statusText}`);
    }
    return response.json();
  }

  async getTraceStats(): Promise<TraceStats> {
    return this.get<TraceStats>('/traces/stats');
  }

  // File upload
  async uploadFile(file: File): Promise<{
    status: string;
    file: string;
    path: string;
    size: number;
    message: string;
    abs_path?: string;
  }> {
    const formData = new FormData();
    formData.append('file', file);

    const response = await fetch(`${this.baseUrl}/files/upload`, {
      method: 'POST',
      headers: this.buildHeaders(),
      body: formData,
    });

    if (!response.ok) {
      throw new Error(`Upload Error: ${response.status} ${response.statusText}`);
    }

    return response.json();
  }

  // File download (returns Blob; caller can create object URL or save)
  async downloadFile(name: string): Promise<Blob> {
    const response = await fetch(
      `${this.baseUrl}/files/download?name=${encodeURIComponent(name)}`,
      {
        headers: this.buildHeaders(),
      }
    );

    if (!response.ok) {
      throw new Error(`Download Error: ${response.status} ${response.statusText}`);
    }

    return response.blob();
  }

  // Auth: 登录，返回 token，并自动写入 AgentAPI 的 authToken
  async login(username: string, password: string): Promise<{ token: string; expires?: number }> {
    const response = await fetch(`${this.baseUrl}/auth/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      throw new Error(`Login failed: ${response.status} ${response.statusText}`);
    }

    const data: { token: string; expires?: number } = await response.json();
    this.setAuthToken(data.token || '');
    return data;
  }
}

export const agentAPI = new AgentAPI();
export default agentAPI;
