export type ModuleFile = {
  path: string;
  content: string;
  language: string;
};

export type ModuleVersion = {
  version: string;
  message: string;
  files: ModuleFile[];
  created_at: string;
};

export type SnippetModule = {
  id: string;
  title: string;
  language: string;
  framework: string;
  description: string;
  tags: string[];
  dependencies: string[];
  files: ModuleFile[];
  versions: ModuleVersion[];
  owner: string;
  favorite: boolean;
  created_at: string;
  updated_at: string;
};

export type ClusterStatus = {
  view: {
    number: number;
    primary: string;
    backup: string;
    idle: string[];
    acknowledged: boolean;
    updated_at: string;
  };
  nodes: Array<{
    id: string;
    role: string;
    healthy: boolean;
    store: Record<string, string>;
    log_index: number;
    snapshot_index: number;
    last_applied: string;
  }>;
  shards: Array<{ shard: number; owner: string }>;
  log: Array<{
    index: number;
    view: number;
    node: string;
    shard: number;
    operation: {
      type: string;
      key: string;
      value: string;
      request_id: string;
      client_id: string;
    };
    at: string;
  }>;
  snapshot_at: string;
  snapshot_key: string;
};

export type GenerateResponse = {
  project_name: string;
  files: Array<{ path: string; content: string }>;
  dependency_summary: string[];
  archive_name: string;
};
