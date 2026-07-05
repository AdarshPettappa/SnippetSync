package synckv

import "testing"

func TestDuplicateRequestIsNotAppliedTwice(t *testing.T) {
	cluster := NewCluster()
	op := KVOperation{Type: OpAppend, Key: "module:a", Value: "x", RequestID: "r1", ClientID: "c1"}
	first, err := cluster.Append(op)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	second, err := cluster.Append(op)
	if err != nil {
		t.Fatalf("duplicate append failed: %v", err)
	}
	value, _ := cluster.Get("module:a")
	if value != "x" {
		t.Fatalf("duplicate append changed value: %q", value)
	}
	if !second.Duplicate || second.LogIndex != first.LogIndex {
		t.Fatalf("duplicate result not cached: %#v", second)
	}
}

func TestFailoverPromotesBackupAndPreservesData(t *testing.T) {
	cluster := NewCluster()
	_, err := cluster.Put(KVOperation{Type: OpPut, Key: "module:a", Value: "payload", RequestID: "r1"})
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	view, err := cluster.Failover()
	if err != nil {
		t.Fatalf("failover failed: %v", err)
	}
	if view.Primary != "node-b" {
		t.Fatalf("expected node-b primary, got %s", view.Primary)
	}
	for _, shard := range cluster.Status().Shards {
		if shard.Owner == "node-a" {
			t.Fatalf("failed primary still owns shard %d", shard.Shard)
		}
	}
	value, ok := cluster.Get("module:a")
	if !ok || value != "payload" {
		t.Fatalf("lost committed data after failover: %q", value)
	}
}

func TestShardReassignment(t *testing.T) {
	cluster := NewCluster()
	if err := cluster.ReassignShard(3, "node-c"); err != nil {
		t.Fatalf("reassign failed: %v", err)
	}
	status := cluster.Status()
	if status.Shards[3].Owner != "node-c" {
		t.Fatalf("expected shard 3 owner node-c, got %s", status.Shards[3].Owner)
	}
}
