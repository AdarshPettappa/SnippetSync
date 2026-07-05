import type { ClusterStatus, GenerateResponse, SnippetModule } from './types';

const API_BASE = import.meta.env.VITE_API_BASE ?? '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Request-ID': crypto.randomUUID(),
      ...(init?.headers ?? {})
    }
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error ?? `Request failed with ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export const api = {
  modules: () => request<SnippetModule[]>('/api/modules'),
  search: (q: string) => request<SnippetModule[]>(`/api/search?q=${encodeURIComponent(q)}`),
  generate: (projectName: string, moduleIds: string[]) =>
    request<GenerateResponse>('/api/generate', {
      method: 'POST',
      body: JSON.stringify({
        project_name: projectName,
        language: 'Python',
        framework: 'Flask',
        module_ids: moduleIds
      })
    }),
  cluster: () => request<ClusterStatus>('/api/cluster/status'),
  failover: () => request('/api/cluster/failover', { method: 'POST' }),
  snapshot: () => request('/api/cluster/snapshot', { method: 'POST' }),
  sync: () => request('/api/sync', { method: 'POST' }),
  reassignShard: (shard: number, owner: string) =>
    request<ClusterStatus>('/api/cluster/reassign-shard', {
      method: 'POST',
      body: JSON.stringify({ shard, owner })
    })
};
