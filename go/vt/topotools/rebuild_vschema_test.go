package topotools

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/proto"
	"github.com/youtube/vitess/go/vt/logutil"
	"github.com/youtube/vitess/go/vt/zktopo/zktestserver"

	topodatapb "github.com/youtube/vitess/go/vt/proto/topodata"
	vschemapb "github.com/youtube/vitess/go/vt/proto/vschema"
)

func TestRebuildVSchema(t *testing.T) {
	ctx := context.Background()
	logger := logutil.NewConsoleLogger()
	emptySrvVSchema := &vschemapb.SrvVSchema{}

	// Set up topology.
	cells := []string{"cell1", "cell2"}
	ts := zktestserver.New(t, cells)

	// Rebuild with no keyspace / no vschema
	if err := RebuildVSchema(ctx, logger, ts, cells); err != nil {
		t.Errorf("RebuildVSchema failed: %v", err)
	}
	for _, cell := range cells {
		if v, err := ts.GetSrvVSchema(ctx, cell); err != nil || !proto.Equal(v, emptySrvVSchema) {
			t.Errorf("unexpected GetSrvVSchema(%v) result: %v %v", cell, v, err)
		}
	}

	// create a keyspace, rebuild, should see an empty entry
	emptyKs1SrvVSchema := &vschemapb.SrvVSchema{
		Keyspaces: map[string]*vschemapb.Keyspace{
			"ks1": {},
		},
	}
	if err := ts.CreateKeyspace(ctx, "ks1", &topodatapb.Keyspace{}); err != nil {
		t.Fatalf("CreateKeyspace(ks1) failed: %v", err)
	}
	if err := RebuildVSchema(ctx, logger, ts, cells); err != nil {
		t.Errorf("RebuildVSchema failed: %v", err)
	}
	for _, cell := range cells {
		if v, err := ts.GetSrvVSchema(ctx, cell); err != nil || !proto.Equal(v, emptyKs1SrvVSchema) {
			t.Errorf("unexpected GetSrvVSchema(%v) result: %v %v", cell, v, err)
		}
	}

	// save a vschema for the keyspace, rebuild, should see it
	keyspace1 := &vschemapb.Keyspace{
		Sharded: true,
	}
	if err := ts.SaveVSchema(ctx, "ks1", keyspace1); err != nil {
		t.Fatalf("SaveVSchema(ks1) failed: %v", err)
	}
	if err := RebuildVSchema(ctx, logger, ts, cells); err != nil {
		t.Errorf("RebuildVSchema failed: %v", err)
	}
	wanted1 := &vschemapb.SrvVSchema{
		Keyspaces: map[string]*vschemapb.Keyspace{
			"ks1": keyspace1,
		},
	}
	for _, cell := range cells {
		if v, err := ts.GetSrvVSchema(ctx, cell); err != nil || !proto.Equal(v, wanted1) {
			t.Errorf("unexpected GetSrvVSchema(%v) result: %v %v", cell, v, err)
		}
	}

	// save a vschema for a new keyspace, rebuild in one cell only
	if err := ts.CreateKeyspace(ctx, "ks2", &topodatapb.Keyspace{}); err != nil {
		t.Fatalf("CreateKeyspace(ks2) failed: %v", err)
	}
	keyspace2 := &vschemapb.Keyspace{
		Sharded: true,
		Vindexes: map[string]*vschemapb.Vindex{
			"name1": {
				Type: "hash",
			},
		},
		Tables: map[string]*vschemapb.Table{
			"table1": {
				Type: "sequence",
				ColumnVindexes: []*vschemapb.ColumnVindex{
					{
						Column: "column1",
						Name:   "name1",
					},
				},
			},
		},
	}
	if err := ts.SaveVSchema(ctx, "ks2", keyspace2); err != nil {
		t.Fatalf("SaveVSchema(ks1) failed: %v", err)
	}
	if err := RebuildVSchema(ctx, logger, ts, []string{"cell1"}); err != nil {
		t.Errorf("RebuildVSchema failed: %v", err)
	}
	wanted2 := &vschemapb.SrvVSchema{
		Keyspaces: map[string]*vschemapb.Keyspace{
			"ks1": keyspace1,
			"ks2": keyspace2,
		},
	}
	if v, err := ts.GetSrvVSchema(ctx, "cell1"); err != nil || !proto.Equal(v, wanted2) {
		t.Errorf("unexpected GetSrvVSchema result: %v %v", v, err)
	}
	if v, err := ts.GetSrvVSchema(ctx, "cell2"); err != nil || !proto.Equal(v, wanted1) {
		t.Errorf("unexpected GetSrvVSchema result: %v %v", v, err)
	}

	// now rebuild everywhere
	if err := RebuildVSchema(ctx, logger, ts, nil); err != nil {
		t.Errorf("RebuildVSchema failed: %v", err)
	}
	for _, cell := range cells {
		if v, err := ts.GetSrvVSchema(ctx, cell); err != nil || !proto.Equal(v, wanted2) {
			t.Errorf("unexpected GetSrvVSchema(%v) result: %v %v", cell, v, err)
		}
	}

	// delete a keyspace, checks vschema entry in map goes
	if err := ts.DeleteKeyspace(ctx, "ks2"); err != nil {
		t.Fatalf("DeleteKeyspace failed: %v", err)
	}
	if err := RebuildVSchema(ctx, logger, ts, nil); err != nil {
		t.Errorf("RebuildVSchema failed: %v", err)
	}
	for _, cell := range cells {
		if v, err := ts.GetSrvVSchema(ctx, cell); err != nil || !proto.Equal(v, wanted1) {
			t.Errorf("unexpected GetSrvVSchema(%v) result: %v %v", cell, v, err)
		}
	}
}
