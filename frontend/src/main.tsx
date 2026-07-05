import { StrictMode, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Boxes,
  Code2,
  DatabaseZap,
  Download,
  GitBranch,
  Heart,
  RotateCcw,
  Search,
  ServerCog,
  ShieldCheck,
  Sparkles
} from 'lucide-react';
import { api } from './api';
import type { ClusterStatus, GenerateResponse, SnippetModule } from './types';
import './styles.css';

function App() {
  const [modules, setModules] = useState<SnippetModule[]>([]);
  const [cluster, setCluster] = useState<ClusterStatus | null>(null);
  const [selected, setSelected] = useState<string[]>(['jwt-auth', 'postgresql-setup', 'docker-config', 'logging-framework']);
  const [activeModule, setActiveModule] = useState<SnippetModule | null>(null);
  const [query, setQuery] = useState('');
  const [generated, setGenerated] = useState<GenerateResponse | null>(null);
  const [status, setStatus] = useState('Loading SnippetSync...');

  const refresh = async () => {
    const [moduleData, clusterData] = await Promise.all([query ? api.search(query) : api.modules(), api.cluster()]);
    setModules(moduleData);
    setCluster(clusterData);
    setActiveModule((current) => current ?? moduleData[0] ?? null);
    setStatus('Ready');
  };

  useEffect(() => {
    refresh().catch((error) => setStatus(error.message));
  }, []);

  useEffect(() => {
    const id = window.setTimeout(() => {
      refresh().catch((error) => setStatus(error.message));
    }, 250);
    return () => window.clearTimeout(id);
  }, [query]);

  const favorites = modules.filter((module) => module.favorite);
  const tagCounts = useMemo(() => {
    const counts = new Map<string, number>();
    modules.forEach((module) => module.tags.forEach((tag) => counts.set(tag, (counts.get(tag) ?? 0) + 1)));
    return Array.from(counts.entries()).sort((a, b) => b[1] - a[1]).slice(0, 10);
  }, [modules]);

  const toggleSelected = (id: string) => {
    setSelected((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]));
  };

  const generate = async () => {
    setStatus('Generating project from selected modules...');
    try {
      const response = await api.generate('flask-starter', selected);
      setGenerated(response);
      setStatus(`Generated ${response.files.length} files`);
      await refresh();
    } catch (error) {
      setStatus(error instanceof Error ? error.message : 'Generation failed');
    }
  };

  const clusterAction = async (action: 'failover' | 'snapshot' | 'sync') => {
    setStatus(`${action} requested...`);
    try {
      if (action === 'failover') await api.failover();
      if (action === 'snapshot') await api.snapshot();
      if (action === 'sync') await api.sync();
      await refresh();
    } catch (error) {
      setStatus(error instanceof Error ? error.message : `${action} failed`);
    }
  };

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <div className="brand-row">
            <DatabaseZap size={28} />
            <h1>SnippetSync</h1>
          </div>
          <p>Reusable software modules backed by a primary/backup SyncKV store.</p>
        </div>
        <div className="status-pill">
          <ShieldCheck size={16} />
          {status}
        </div>
      </header>

      <section className="metrics-grid">
        <Metric icon={<Boxes />} label="Modules" value={modules.length.toString()} />
        <Metric icon={<Heart />} label="Favorites" value={favorites.length.toString()} />
        <Metric icon={<GitBranch />} label="View" value={cluster ? `#${cluster.view.number}` : '-'} />
        <Metric icon={<ServerCog />} label="Primary" value={cluster?.view.primary ?? '-'} />
      </section>

      <section className="workbench">
        <aside className="module-pane">
          <div className="searchbar">
            <Search size={18} />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search modules, tags, frameworks" />
          </div>
          <div className="tag-strip">
            {tagCounts.map(([tag, count]) => (
              <button key={tag} onClick={() => setQuery(tag)}>{tag} {count}</button>
            ))}
          </div>
          <div className="module-list">
            {modules.map((module) => (
              <button className={activeModule?.id === module.id ? 'module-row active' : 'module-row'} key={module.id} onClick={() => setActiveModule(module)}>
                <span>{module.title}</span>
                <small>{module.language} / {module.framework}</small>
              </button>
            ))}
          </div>
        </aside>

        <section className="detail-pane">
          {activeModule && (
            <>
              <div className="section-heading">
                <div>
                  <h2>{activeModule.title}</h2>
                  <p>{activeModule.description}</p>
                </div>
                <button className={selected.includes(activeModule.id) ? 'primary selected' : 'primary'} onClick={() => toggleSelected(activeModule.id)}>
                  <Sparkles size={16} />
                  {selected.includes(activeModule.id) ? 'Selected' : 'Add'}
                </button>
              </div>
              <div className="chips">
                {activeModule.tags.map((tag) => <span key={tag}>{tag}</span>)}
              </div>
              <div className="file-grid">
                {activeModule.files.map((file) => (
                  <article key={file.path} className="file-card">
                    <div><Code2 size={16} />{file.path}</div>
                    <pre>{file.content}</pre>
                  </article>
                ))}
              </div>
            </>
          )}
        </section>

        <aside className="builder-pane">
          <div className="section-heading compact">
            <h2>Project Builder</h2>
            <button className="icon-button" onClick={generate} title="Generate project"><Download size={18} /></button>
          </div>
          <div className="selected-list">
            {selected.map((id) => {
              const module = modules.find((item) => item.id === id);
              return <button key={id} onClick={() => toggleSelected(id)}>{module?.title ?? id}</button>;
            })}
          </div>
          {generated && (
            <div className="generated">
              <strong>{generated.archive_name}</strong>
              <span>{generated.files.length} files</span>
              <span>{generated.dependency_summary.join(', ')}</span>
            </div>
          )}
        </aside>
      </section>

      {cluster && (
        <section className="cluster-panel">
          <div className="section-heading">
            <div>
              <h2>SyncKV Control Plane</h2>
              <p>View-service-inspired primary/backup replication with duplicate request protection, snapshots, and shard ownership.</p>
            </div>
            <div className="actions">
              <button onClick={() => clusterAction('sync')}><RotateCcw size={16} />Sync</button>
              <button onClick={() => clusterAction('snapshot')}>Snapshot</button>
              <button className="danger" onClick={() => clusterAction('failover')}>Failover</button>
            </div>
          </div>
          <div className="cluster-grid">
            {cluster.nodes.map((node) => (
              <article key={node.id} className={`node-card ${node.role}`}>
                <strong>{node.id}</strong>
                <span>{node.role} / {node.healthy ? 'healthy' : 'down'}</span>
                <small>log {node.log_index} / snapshot {node.snapshot_index}</small>
              </article>
            ))}
          </div>
          <div className="shard-grid">
            {cluster.shards.map((shard) => (
              <button key={shard.shard} onClick={() => api.reassignShard(shard.shard, cluster.view.primary).then(setCluster)}>
                S{shard.shard}<span>{shard.owner}</span>
              </button>
            ))}
          </div>
          <div className="log-strip">
            {cluster.log.slice(-8).map((entry) => (
              <span key={entry.index}>#{entry.index} {entry.operation.type} {entry.shard}</span>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}

function Metric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <article className="metric">
      {icon}
      <span>{label}</span>
      <strong>{value}</strong>
    </article>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
