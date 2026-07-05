package synckv

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const ShardCount = 8

var (
	ErrWrongServer = errors.New("wrong server")
	ErrWrongGroup  = errors.New("wrong shard owner")
	ErrNoBackup    = errors.New("no healthy backup")
)

type OperationType string

const (
	OpPut    OperationType = "put"
	OpAppend OperationType = "append"
	OpDelete OperationType = "delete"
)

type KVOperation struct {
	Type      OperationType `json:"type"`
	Key       string        `json:"key"`
	Value     string        `json:"value"`
	RequestID string        `json:"request_id"`
	ClientID  string        `json:"client_id"`
}

type OperationResult struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	LogIndex   int    `json:"log_index"`
	Duplicate  bool   `json:"duplicate"`
	Shard      int    `json:"shard"`
	Committed  bool   `json:"committed"`
	Primary    string `json:"primary"`
	Backup     string `json:"backup"`
	ViewNumber int    `json:"view_number"`
}

type View struct {
	Number       int       `json:"number"`
	Primary      string    `json:"primary"`
	Backup       string    `json:"backup"`
	Idle         []string  `json:"idle"`
	Acknowledged bool      `json:"acknowledged"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Node struct {
	ID            string            `json:"id"`
	Role          string            `json:"role"`
	Healthy       bool              `json:"healthy"`
	Store         map[string]string `json:"store"`
	LogIndex      int               `json:"log_index"`
	SnapshotIndex int               `json:"snapshot_index"`
	LastApplied   string            `json:"last_applied"`
}

type ShardAssignment struct {
	Shard int    `json:"shard"`
	Owner string `json:"owner"`
}

type LogEntry struct {
	Index     int         `json:"index"`
	View      int         `json:"view"`
	Node      string      `json:"node"`
	Operation KVOperation `json:"operation"`
	Shard     int         `json:"shard"`
	At        time.Time   `json:"at"`
}

type ClusterStatus struct {
	View        View              `json:"view"`
	Nodes       []Node            `json:"nodes"`
	Shards      []ShardAssignment `json:"shards"`
	Log         []LogEntry        `json:"log"`
	SnapshotAt  time.Time         `json:"snapshot_at"`
	SnapshotKey string            `json:"snapshot_key"`
}

type Cluster struct {
	mu          sync.Mutex
	nodes       map[string]*Node
	view        View
	shards      map[int]string
	requests    map[string]OperationResult
	log         []LogEntry
	snapshotAt  time.Time
	snapshotKey string
}

func NewCluster() *Cluster {
	nodes := map[string]*Node{
		"node-a": newNode("node-a", "primary"),
		"node-b": newNode("node-b", "backup"),
		"node-c": newNode("node-c", "idle"),
	}
	shards := make(map[int]string, ShardCount)
	for i := 0; i < ShardCount; i++ {
		if i%2 == 0 {
			shards[i] = "node-a"
		} else {
			shards[i] = "node-b"
		}
	}
	return &Cluster{
		nodes:    nodes,
		shards:   shards,
		requests: map[string]OperationResult{},
		view: View{
			Number:       1,
			Primary:      "node-a",
			Backup:       "node-b",
			Idle:         []string{"node-c"},
			Acknowledged: true,
			UpdatedAt:    time.Now().UTC(),
		},
	}
}

func newNode(id, role string) *Node {
	return &Node{
		ID:      id,
		Role:    role,
		Healthy: true,
		Store:   map[string]string{},
	}
}

func (c *Cluster) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.nodes[c.view.Primary].Store[key]
	return value, ok
}

func (c *Cluster) Put(op KVOperation) (OperationResult, error) {
	return c.apply(op)
}

func (c *Cluster) Append(op KVOperation) (OperationResult, error) {
	op.Type = OpAppend
	return c.apply(op)
}

func (c *Cluster) Delete(op KVOperation) (OperationResult, error) {
	op.Type = OpDelete
	return c.apply(op)
}

func (c *Cluster) apply(op KVOperation) (OperationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if op.RequestID != "" {
		if cached, ok := c.requests[op.RequestID]; ok {
			cached.Duplicate = true
			return cached, nil
		}
	}

	primary := c.nodes[c.view.Primary]
	backup := c.nodes[c.view.Backup]
	if primary == nil || !primary.Healthy {
		return OperationResult{}, ErrWrongServer
	}
	if backup == nil || !backup.Healthy {
		return OperationResult{}, ErrNoBackup
	}

	shard := KeyToShard(op.Key)
	owner := c.shards[shard]
	if owner != primary.ID && owner != backup.ID {
		return OperationResult{}, ErrWrongGroup
	}

	entry := LogEntry{
		Index:     len(c.log) + 1,
		View:      c.view.Number,
		Node:      primary.ID,
		Operation: op,
		Shard:     shard,
		At:        time.Now().UTC(),
	}

	applyToNode(primary, op, entry.Index)
	applyToNode(backup, op, entry.Index)
	c.log = append(c.log, entry)

	result := OperationResult{
		Key:        op.Key,
		Value:      primary.Store[op.Key],
		LogIndex:   entry.Index,
		Shard:      shard,
		Committed:  true,
		Primary:    primary.ID,
		Backup:     backup.ID,
		ViewNumber: c.view.Number,
	}
	if op.RequestID != "" {
		c.requests[op.RequestID] = result
	}
	return result, nil
}

func applyToNode(node *Node, op KVOperation, index int) {
	switch op.Type {
	case OpAppend:
		node.Store[op.Key] += op.Value
	case OpDelete:
		delete(node.Store, op.Key)
	default:
		node.Store[op.Key] = op.Value
	}
	node.LogIndex = index
	node.LastApplied = op.RequestID
}

func (c *Cluster) Failover() (View, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldPrimary := c.nodes[c.view.Primary]
	oldBackup := c.nodes[c.view.Backup]
	if oldBackup == nil || !oldBackup.Healthy {
		return c.view, ErrNoBackup
	}
	if oldPrimary != nil {
		oldPrimary.Healthy = false
		oldPrimary.Role = "down"
		for shard, owner := range c.shards {
			if owner == oldPrimary.ID {
				c.shards[shard] = oldBackup.ID
			}
		}
	}
	oldBackup.Role = "primary"

	newBackupID := ""
	for _, id := range sortedNodeIDs(c.nodes) {
		node := c.nodes[id]
		if id != oldBackup.ID && node.Healthy {
			newBackupID = id
			break
		}
	}
	if newBackupID != "" {
		next := c.nodes[newBackupID]
		next.Role = "backup"
		next.Store = cloneStore(oldBackup.Store)
		next.LogIndex = oldBackup.LogIndex
	}

	c.view.Number++
	c.view.Primary = oldBackup.ID
	c.view.Backup = newBackupID
	c.view.Acknowledged = true
	c.view.UpdatedAt = time.Now().UTC()
	c.view.Idle = idleNodes(c.nodes, c.view.Primary, c.view.Backup)
	return c.view, nil
}

func (c *Cluster) Snapshot(path string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if path == "" {
		path = filepath.Join("data", "synckv-snapshot.json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	payload := c.statusLocked()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	c.snapshotAt = time.Now().UTC()
	c.snapshotKey = path
	for _, node := range c.nodes {
		node.SnapshotIndex = node.LogIndex
	}
	return path, nil
}

func (c *Cluster) ReassignShard(shard int, owner string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if shard < 0 || shard >= ShardCount {
		return fmt.Errorf("shard must be between 0 and %d", ShardCount-1)
	}
	node := c.nodes[owner]
	if node == nil || !node.Healthy {
		return fmt.Errorf("owner %q is not a healthy node", owner)
	}
	c.shards[shard] = owner
	return nil
}

func (c *Cluster) Status() ClusterStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked()
}

func (c *Cluster) statusLocked() ClusterStatus {
	nodes := make([]Node, 0, len(c.nodes))
	for _, id := range sortedNodeIDs(c.nodes) {
		node := c.nodes[id]
		copyNode := *node
		copyNode.Store = cloneStore(node.Store)
		nodes = append(nodes, copyNode)
	}
	shards := make([]ShardAssignment, 0, len(c.shards))
	for i := 0; i < ShardCount; i++ {
		shards = append(shards, ShardAssignment{Shard: i, Owner: c.shards[i]})
	}
	logCopy := append([]LogEntry(nil), c.log...)
	return ClusterStatus{
		View:        c.view,
		Nodes:       nodes,
		Shards:      shards,
		Log:         logCopy,
		SnapshotAt:  c.snapshotAt,
		SnapshotKey: c.snapshotKey,
	}
}

func (c *Cluster) Sync() {
	c.mu.Lock()
	defer c.mu.Unlock()

	primary := c.nodes[c.view.Primary]
	backup := c.nodes[c.view.Backup]
	if primary == nil || backup == nil {
		return
	}
	backup.Store = cloneStore(primary.Store)
	backup.LogIndex = primary.LogIndex
	c.view.Acknowledged = true
	c.view.UpdatedAt = time.Now().UTC()
}

func KeyToShard(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(key)))
	return int(h.Sum32() % ShardCount)
}

func sortedNodeIDs(nodes map[string]*Node) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func idleNodes(nodes map[string]*Node, primary, backup string) []string {
	var idle []string
	for _, id := range sortedNodeIDs(nodes) {
		node := nodes[id]
		if node.Healthy && id != primary && id != backup {
			node.Role = "idle"
			idle = append(idle, id)
		}
	}
	return idle
}

func cloneStore(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
