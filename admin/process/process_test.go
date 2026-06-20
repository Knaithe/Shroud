package process

import (
	"bytes"
	"testing"

	"Shroud/admin/initial"
	"Shroud/admin/topology"
)

func TestNewAdmin(t *testing.T) {
	opt := &initial.Options{
		Secret: []byte("testsecret"),
		Listen: "8080",
	}
	topo := topology.NewTopology()

	admin := NewAdmin(opt, topo)
	if admin == nil {
		t.Fatal("NewAdmin returned nil")
	}
	if admin.options != opt {
		t.Error("options field not set correctly")
	}
	if admin.topology != topo {
		t.Error("topology field not set correctly")
	}
	if admin.mgr != nil {
		t.Error("mgr should be nil before Run is called")
	}
}

func TestNewAdmin_NilOptions(t *testing.T) {
	topo := topology.NewTopology()
	admin := NewAdmin(nil, topo)
	if admin == nil {
		t.Fatal("NewAdmin returned nil with nil options")
	}
	if admin.options != nil {
		t.Error("expected nil options")
	}
}

func TestNewAdmin_NilTopology(t *testing.T) {
	opt := &initial.Options{
		Secret: []byte("test"),
	}
	admin := NewAdmin(opt, nil)
	if admin == nil {
		t.Fatal("NewAdmin returned nil with nil topology")
	}
	if admin.topology != nil {
		t.Error("expected nil topology")
	}
}

func TestNewAdmin_MultipleInstances(t *testing.T) {
	opt1 := &initial.Options{Secret: []byte("s1"), Listen: "8080"}
	opt2 := &initial.Options{Secret: []byte("s2"), Connect: "10.0.0.1:9090"}
	topo := topology.NewTopology()

	a1 := NewAdmin(opt1, topo)
	a2 := NewAdmin(opt2, topo)

	if a1 == a2 {
		t.Error("different NewAdmin calls should return different instances")
	}
	if bytes.Equal(a1.options.Secret, a2.options.Secret) {
		t.Error("instances should have independent options")
	}
}
