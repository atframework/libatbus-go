// Package libatbus_impl provides internal implementation details for libatbus.

package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"

	types "github.com/atframework/libatbus-go/types"
)

func makeTopologyData(pid int32, hostname string, labels map[string]string) *types.TopologyData {
	return &types.TopologyData{
		Pid:      pid,
		Hostname: hostname,
		Labels:   labels,
	}
}

func makeLabelValueSet(values ...string) map[string]struct{} {
	ret := make(map[string]struct{}, len(values))
	for _, v := range values {
		ret[v] = struct{}{}
	}
	return ret
}

func TestTopologyPeerBasic(t *testing.T) {
	t.Run("TopologyPeerBasic", func(t *testing.T) {
		// Arrange: create 2 peers
		peer1 := CreateTopologyPeer(0x12345678)
		peer2 := CreateTopologyPeer(0x22345678)

		// Act + Assert: basic getters
		assert.Equal(t, types.BusIdType(0x12345678), peer1.GetBusId())
		assert.Equal(t, types.BusIdType(0x22345678), peer2.GetBusId())

		assert.Nil(t, peer1.GetUpstream())
		assert.Nil(t, peer2.GetUpstream())

		// Act + Assert: downstream set is empty
		assert.False(t, peer1.ContainsDownstream(0x22345678))
		assert.False(t, peer2.ContainsDownstream(0x12345678))

		// Act: link peer1 -> peer2
		peer1.addDownstream(peer2)
		peer2.updateUpstream(peer1)

		// Assert: relationship is established
		assert.Equal(t, peer2.GetUpstream(), peer1)
		assert.True(t, peer1.ContainsDownstream(0x22345678))
		assert.False(t, peer2.ContainsDownstream(0x12345678))

		// Act + Assert: remove downstream with mismatched check should not remove
		peer1.removeDownstream(0x22345678, peer1)
		assert.True(t, peer1.ContainsDownstream(0x22345678))

		// Act + Assert: remove downstream with matching check should remove
		peer1.removeDownstream(0x22345678, peer2)
		assert.False(t, peer1.ContainsDownstream(0x22345678))

		data := types.TopologyData{
			Pid:      1234,
			Hostname: "test-host",
			Labels:   map[string]string{"key1": "value1", "key2": "value2"},
		}
		peer1.updateData(&data)
		td := peer1.GetTopologyData()
		assert.Equal(t, td.Pid, int32(1234))
		assert.Equal(t, td.Hostname, "test-host")
		assert.Equal(t, td.Labels["key1"], "value1")
		assert.Equal(t, td.Labels["key2"], "value2")
	})
}

func TestTopologyPeerNilReceiver(t *testing.T) {
	t.Run("NilReceiver", func(t *testing.T) {
		// Arrange: a nil peer pointer
		var p *TopologyPeer

		// Act + Assert: nil-safe behavior
		assert.Equal(t, types.BusIdType(0), p.GetBusId())
		assert.Nil(t, p.GetUpstream())
		assert.Nil(t, p.GetTopologyData())
		assert.False(t, p.ContainsDownstream(1))
	})
}

func TestTopologyPeerForeachDownstream(t *testing.T) {
	t.Run("ForeachDownstreamBasicAndCleanup", func(t *testing.T) {
		// Arrange: create a parent with two downstream peers and one invalid entry.
		parent := CreateTopologyPeer(1)
		peer2 := CreateTopologyPeer(2)
		peer3 := CreateTopologyPeer(3)
		parent.addDownstream(peer2)
		parent.addDownstream(peer3)
		parent.downstream[4] = nil

		// Act: iterate all downstream nodes.
		count := 0
		ok := parent.ForeachDownstream(func(peer types.TopologyPeer) bool {
			count++
			return true
		})

		// Assert: all valid downstreams visited, invalid entry cleaned.
		assert.True(t, ok)
		assert.Equal(t, 2, count)
		assert.False(t, parent.ContainsDownstream(4))
	})

	t.Run("ForeachDownstreamBreakEarly", func(t *testing.T) {
		// Arrange: create a parent with two downstream peers.
		parent := CreateTopologyPeer(10)
		parent.addDownstream(CreateTopologyPeer(20))
		parent.addDownstream(CreateTopologyPeer(30))

		// Act: break on first visit.
		count := 0
		ok := parent.ForeachDownstream(func(peer types.TopologyPeer) bool {
			count++
			return false
		})

		// Assert: early break returns false and only one callback runs.
		assert.False(t, ok)
		assert.Equal(t, 1, count)
	})

	t.Run("ForeachDownstreamNilReceiverAndEmpty", func(t *testing.T) {
		// Arrange: nil peer and empty downstream.
		var nilPeer *TopologyPeer
		emptyPeer := CreateTopologyPeer(100)

		// Act + Assert: nil or empty downstream returns true without calling.
		called := 0
		assert.True(t, nilPeer.ForeachDownstream(func(peer types.TopologyPeer) bool {
			called++
			return true
		}))
		assert.True(t, emptyPeer.ForeachDownstream(func(peer types.TopologyPeer) bool {
			called++
			return true
		}))
		assert.Equal(t, 0, called)
	})
}

func TestTopologyRegistryRelations(t *testing.T) {
	t.Run("Relations", func(t *testing.T) {
		// Arrange: build a small forest (same as C++ case)
		//   1
		//  / \
		// 2   4
		// |
		// 3
		//   10
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		registry.UpdatePeer(1, 0, makeTopologyData(1, "h1", nil))
		registry.UpdatePeer(2, 1, makeTopologyData(2, "h1", nil))
		registry.UpdatePeer(3, 2, makeTopologyData(3, "h1", nil))
		registry.UpdatePeer(4, 1, makeTopologyData(4, "h1", nil))
		registry.UpdatePeer(10, 0, makeTopologyData(10, "h2", nil))

		// Act + Assert: invalid inputs
		rel, nextHop := registry.GetRelation(0, 1)
		assert.Equal(t, types.TopologyRelationType_Invalid, rel)
		assert.Nil(t, nextHop)

		rel, nextHop = registry.GetRelation(1, 0)
		assert.Equal(t, types.TopologyRelationType_Invalid, rel)
		assert.Nil(t, nextHop)

		rel, nextHop = registry.GetRelation(1, 9999)
		assert.Equal(t, types.TopologyRelationType_Invalid, rel)
		assert.Nil(t, nextHop)

		// Act + Assert: self
		rel, nextHop = registry.GetRelation(1, 1)
		assert.Equal(t, types.TopologyRelationType_Self, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(1), nextHop.GetBusId())
		}

		// Act + Assert: immediate upstream/downstream
		rel, nextHop = registry.GetRelation(2, 1)
		assert.Equal(t, types.TopologyRelationType_ImmediateUpstream, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(1), nextHop.GetBusId())
		}

		rel, nextHop = registry.GetRelation(1, 2)
		assert.Equal(t, types.TopologyRelationType_ImmediateDownstream, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(2), nextHop.GetBusId())
		}

		// Act + Assert: transitive upstream/downstream (next hop is the first hop)
		rel, nextHop = registry.GetRelation(3, 1)
		assert.Equal(t, types.TopologyRelationType_TransitiveUpstream, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(2), nextHop.GetBusId())
		}

		rel, nextHop = registry.GetRelation(1, 3)
		assert.Equal(t, types.TopologyRelationType_TransitiveDownstream, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(2), nextHop.GetBusId())
		}

		// Act + Assert: same upstream peer
		rel, nextHop = registry.GetRelation(2, 4)
		assert.Equal(t, types.TopologyRelationType_SameUpstreamPeer, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(1), nextHop.GetBusId())
		}

		// Act + Assert: different roots
		// from=1 has no upstream, so next hop should fall back to "to".
		rel, nextHop = registry.GetRelation(1, 10)
		assert.Equal(t, types.TopologyRelationType_OtherUpstreamPeer, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(10), nextHop.GetBusId())
		}

		// from=2 has an upstream, so next hop should be from's upstream.
		rel, nextHop = registry.GetRelation(2, 10)
		assert.Equal(t, types.TopologyRelationType_OtherUpstreamPeer, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(1), nextHop.GetBusId())
		}
	})
}

func TestTopologyRegistryUpdateAndRemove(t *testing.T) {
	t.Run("UpdateAndRemove", func(t *testing.T) {
		// Arrange: build a chain 10 <- 2 <- 3
		// Note: peers created as targetBusId via UpdatePeer are "proactivelyAdded".
		// A proactivelyAdded peer must NOT be removed automatically unless RemovePeer is called.
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		assert.True(t, registry.UpdatePeer(10, 0, makeTopologyData(10, "h10", nil)))
		assert.True(t, registry.UpdatePeer(2, 10, makeTopologyData(2, "h2", nil)))
		assert.True(t, registry.UpdatePeer(3, 2, makeTopologyData(3, "h3", nil)))

		peer10 := registry.GetPeer(10)
		peer2 := registry.GetPeer(2)
		peer3 := registry.GetPeer(3)
		if assert.NotNil(t, peer10) && assert.NotNil(t, peer2) && assert.NotNil(t, peer3) {
			assert.True(t, peer10.ContainsDownstream(2))
			assert.True(t, peer2.ContainsDownstream(3))
			assert.Equal(t, types.BusIdType(10), peer2.GetUpstream().GetBusId())
			assert.Equal(t, types.BusIdType(2), peer3.GetUpstream().GetBusId())
		}

		// Act: try to remove peer2 while it still has downstream.
		// Assert: implementation keeps the node (cannot delete when it still has downstream).
		registry.RemovePeer(2)
		assert.NotNil(t, registry.GetPeer(2))

		// Act: remove leaf peer3, then remove peer2 again.
		// Assert: leaf can be deleted; then peer2 can be deleted once it has no downstream.
		registry.RemovePeer(3)
		assert.Nil(t, registry.GetPeer(3))
		assert.Nil(t, registry.GetPeer(2))

		// Act + Assert: proactivelyAdded upstream (peer10) should remain unless explicitly removed.
		assert.NotNil(t, registry.GetPeer(10))
		registry.RemovePeer(10)
		assert.Nil(t, registry.GetPeer(10))
	})
}

func TestTopologyRegistryPassivePeerAutoDelete(t *testing.T) {
	t.Run("PassiveUpstreamAutoDeleteAfterDownstreamRemoved", func(t *testing.T) {
		// Arrange: create peer2 with upstream=1 without proactively creating peer1.
		// This makes peer1 a passively-added peer (created by upstreamBusId).
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		assert.True(t, registry.UpdatePeer(2, 1, makeTopologyData(2, "h2", nil)))
		assert.NotNil(t, registry.GetPeer(2))
		assert.NotNil(t, registry.GetPeer(1))
		assert.True(t, registry.GetPeer(1).ContainsDownstream(2))

		// Act: remove the only downstream (peer2).
		registry.RemovePeer(2)

		// Assert: peer2 is removed, and peer1 is auto-removed because it was passive and has no downstream now.
		assert.Nil(t, registry.GetPeer(2))
		assert.Nil(t, registry.GetPeer(1))
	})

	t.Run("ProactiveUpstreamNotAutoDeleted", func(t *testing.T) {
		// Arrange: proactively create peer1 first, then attach peer2 under it.
		// peer1 must not be auto-deleted when peer2 is removed.
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		assert.True(t, registry.UpdatePeer(1, 0, makeTopologyData(1, "h1", nil)))
		assert.True(t, registry.UpdatePeer(2, 1, makeTopologyData(2, "h2", nil)))
		assert.NotNil(t, registry.GetPeer(1))
		assert.NotNil(t, registry.GetPeer(2))
		assert.True(t, registry.GetPeer(1).ContainsDownstream(2))

		// Act: remove peer2.
		registry.RemovePeer(2)

		// Assert: peer2 removed, peer1 remains because it was proactively added.
		assert.Nil(t, registry.GetPeer(2))
		assert.NotNil(t, registry.GetPeer(1))
		assert.False(t, registry.GetPeer(1).ContainsDownstream(2))
	})
}

func TestTopologyRegistryUpdatePeerCycle(t *testing.T) {
	t.Run("RejectSelfLoopOnNewPeer", func(t *testing.T) {
		// Arrange: create an empty registry
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		// Act: try to create a peer whose upstream is itself (cycle)
		ok := registry.UpdatePeer(1, 1, makeTopologyData(1, "h1", nil))

		// Assert: must reject because it forms a circle
		assert.False(t, ok)
		p := registry.GetPeer(1)
		// Implementation may or may not keep a partially-created node; but it must not form a self-loop.
		if p != nil {
			assert.Nil(t, p.GetUpstream())
			assert.False(t, p.ContainsDownstream(1))
		}
	})

	t.Run("RejectCycleByUpdatingExistingRoot", func(t *testing.T) {
		// Arrange: build a chain 1 <- 2 <- 3
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		assert.True(t, registry.UpdatePeer(1, 0, makeTopologyData(1, "h1", nil)))
		assert.True(t, registry.UpdatePeer(2, 1, makeTopologyData(2, "h1", nil)))
		assert.True(t, registry.UpdatePeer(3, 2, makeTopologyData(3, "h1", nil)))

		p1 := registry.GetPeer(1)
		p2 := registry.GetPeer(2)
		p3 := registry.GetPeer(3)
		if assert.NotNil(t, p1) && assert.NotNil(t, p2) && assert.NotNil(t, p3) {
			assert.Nil(t, p1.GetUpstream())
			assert.Equal(t, types.BusIdType(1), p2.GetUpstream().GetBusId())
			assert.Equal(t, types.BusIdType(2), p3.GetUpstream().GetBusId())
		}

		// Act: update peer1's upstream to peer3 (would create 1 -> 3 -> 2 -> 1)
		ok := registry.UpdatePeer(1, 3, makeTopologyData(1, "h1", nil))

		// Assert: reject update and keep previous structure intact
		assert.False(t, ok)
		p1 = registry.GetPeer(1)
		p2 = registry.GetPeer(2)
		p3 = registry.GetPeer(3)
		if assert.NotNil(t, p1) && assert.NotNil(t, p2) && assert.NotNil(t, p3) {
			assert.Nil(t, p1.GetUpstream())
			assert.Equal(t, types.BusIdType(1), p2.GetUpstream().GetBusId())
			assert.Equal(t, types.BusIdType(2), p3.GetUpstream().GetBusId())
			assert.False(t, p3.ContainsDownstream(1))
		}

		// And relation queries should still behave as the original chain
		rel, nextHop := registry.GetRelation(1, 3)
		assert.Equal(t, types.TopologyRelationType_TransitiveDownstream, rel)
		if assert.NotNil(t, nextHop) {
			assert.Equal(t, types.BusIdType(2), nextHop.GetBusId())
		}
	})

	t.Run("RejectCycleByUpdatingExistingMiddleNode", func(t *testing.T) {
		// Arrange: build a chain 1 <- 2 <- 3
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		assert.True(t, registry.UpdatePeer(1, 0, makeTopologyData(1, "h1", nil)))
		assert.True(t, registry.UpdatePeer(2, 1, makeTopologyData(2, "h1", nil)))
		assert.True(t, registry.UpdatePeer(3, 2, makeTopologyData(3, "h1", nil)))

		// Act: update peer2 upstream to 3 (would create 2 <-> 3)
		ok := registry.UpdatePeer(2, 3, makeTopologyData(2, "h1", nil))

		// Assert: reject and keep original wiring
		assert.False(t, ok)
		p2 := registry.GetPeer(2)
		p3 := registry.GetPeer(3)
		if assert.NotNil(t, p2) && assert.NotNil(t, p3) {
			if assert.NotNil(t, p2.GetUpstream()) {
				assert.Equal(t, types.BusIdType(1), p2.GetUpstream().GetBusId())
			}
			if assert.NotNil(t, p3.GetUpstream()) {
				assert.Equal(t, types.BusIdType(2), p3.GetUpstream().GetBusId())
			}
			assert.False(t, p3.ContainsDownstream(2))
		}
	})
}

func TestTopologyRegistryForeachAndPolicy(t *testing.T) {
	t.Run("ForeachAndPolicy", func(t *testing.T) {
		// Arrange
		var registry types.TopologyRegistry = CreateTopologyRegistry()
		assert.NotNil(t, registry)

		registry.UpdatePeer(1, 0, makeTopologyData(100, "host_a", nil))
		registry.UpdatePeer(2, 1, makeTopologyData(101, "host_a", nil))
		registry.UpdatePeer(3, 1, makeTopologyData(102, "host_b", nil))

		// Act + Assert: foreach all
		countAll := 0
		ok := registry.ForeachPeer(func(peer types.TopologyPeer) bool {
			countAll++
			return true
		})
		assert.True(t, ok)
		assert.Equal(t, 3, countAll)

		// Act + Assert: foreach break early
		countBreak := 0
		ok = registry.ForeachPeer(func(peer types.TopologyPeer) bool {
			countBreak++
			return false
		})
		assert.False(t, ok)
		assert.Equal(t, 1, countBreak)

		// Act + Assert: check policy (mirror C++ case)
		policy := types.TopologyPolicyRule{}
		fromData := makeTopologyData(1234, "host_a", nil)
		toDataOk := makeTopologyData(1234, "host_a", map[string]string{"zone": "1"})

		policy.RequireSameHostName = true
		policy.RequireSameProcess = true
		policy.RequireLabelValues = map[string]map[string]struct{}{
			"zone": makeLabelValueSet("1", "2"),
		}

		assert.True(t, registry.CheckPolicy(&policy, fromData, toDataOk))

		toDataBadHost := makeTopologyData(toDataOk.Pid, "host_b", map[string]string{"zone": "1"})
		assert.False(t, registry.CheckPolicy(&policy, fromData, toDataBadHost))

		toDataBadPid := makeTopologyData(5678, toDataOk.Hostname, map[string]string{"zone": "1"})
		assert.False(t, registry.CheckPolicy(&policy, fromData, toDataBadPid))

		toDataBadLabel := makeTopologyData(toDataOk.Pid, toDataOk.Hostname, map[string]string{"zone": "3"})
		assert.False(t, registry.CheckPolicy(&policy, fromData, toDataBadLabel))

		toDataMissingLabel := makeTopologyData(toDataOk.Pid, toDataOk.Hostname, nil)
		assert.False(t, registry.CheckPolicy(&policy, fromData, toDataMissingLabel))

		// Boundary behaviors
		assert.True(t, registry.CheckPolicy(nil, fromData, toDataOk))
		assert.False(t, registry.CheckPolicy(&policy, nil, toDataOk))
		assert.False(t, registry.CheckPolicy(&policy, fromData, nil))

		t.Run("PolicySameHostNameTrueSameProcessFalse", func(t *testing.T) {
			// Arrange: require same hostname only; pid is NOT required to match
			policy := types.TopologyPolicyRule{}
			policy.RequireSameHostName = true
			policy.RequireSameProcess = false
			policy.RequireLabelValues = map[string]map[string]struct{}{
				"zone": makeLabelValueSet("1"),
			}

			fromData := makeTopologyData(1234, "host_a", nil)
			toDataSameHostDifferentPid := makeTopologyData(5678, "host_a", map[string]string{"zone": "1"})
			toDataDifferentHost := makeTopologyData(1234, "host_b", map[string]string{"zone": "1"})

			// Act + Assert: same host should pass even if pid differs
			assert.True(t, registry.CheckPolicy(&policy, fromData, toDataSameHostDifferentPid))
			// Act + Assert: different host should fail
			assert.False(t, registry.CheckPolicy(&policy, fromData, toDataDifferentHost))
		})

		t.Run("PolicySameHostNameFalseSameProcessTrue", func(t *testing.T) {
			// Arrange: require same process; implementation implies same hostname as well
			policy := types.TopologyPolicyRule{}
			policy.RequireSameHostName = false
			policy.RequireSameProcess = true
			policy.RequireLabelValues = map[string]map[string]struct{}{
				"zone": makeLabelValueSet("1"),
			}

			fromData := makeTopologyData(1234, "host_a", nil)
			toDataOk := makeTopologyData(1234, "host_a", map[string]string{"zone": "1"})
			toDataBadPid := makeTopologyData(5678, "host_a", map[string]string{"zone": "1"})
			toDataBadHost := makeTopologyData(1234, "host_b", map[string]string{"zone": "1"})

			// Act + Assert: exact same process+host should pass
			assert.True(t, registry.CheckPolicy(&policy, fromData, toDataOk))
			// Act + Assert: same host but different pid should fail
			assert.False(t, registry.CheckPolicy(&policy, fromData, toDataBadPid))
			// Act + Assert: different host should fail even if pid matches
			assert.False(t, registry.CheckPolicy(&policy, fromData, toDataBadHost))
		})

		t.Run("PolicySameHostNameFalseSameProcessFalse", func(t *testing.T) {
			// Arrange: neither same-host nor same-process is required; only labels (if configured) are checked
			policy := types.TopologyPolicyRule{}
			policy.RequireSameHostName = false
			policy.RequireSameProcess = false
			policy.RequireLabelValues = map[string]map[string]struct{}{
				"zone": makeLabelValueSet("1", "2"),
			}

			fromData := makeTopologyData(1234, "host_a", nil)
			toDataDifferentHostPidButLabelOk := makeTopologyData(5678, "host_b", map[string]string{"zone": "1"})
			toDataLabelMissing := makeTopologyData(5678, "host_b", nil)

			// Act + Assert: host/pid mismatch should NOT matter when both flags are false
			assert.True(t, registry.CheckPolicy(&policy, fromData, toDataDifferentHostPidButLabelOk))
			// Act + Assert: label still required (missing label should fail)
			assert.False(t, registry.CheckPolicy(&policy, fromData, toDataLabelMissing))

			// Arrange: no label constraints => should pass as long as from/to are non-nil
			policy.RequireLabelValues = nil
			assert.True(t, registry.CheckPolicy(&policy, fromData, toDataLabelMissing))
		})
	})
}

func TestTopologyRegistryNilReceiverBehaviors(t *testing.T) {
	t.Run("NilReceiver", func(t *testing.T) {
		// Arrange: nil registry pointer
		var r *TopologyRegistry

		// Act + Assert: nil-safe behaviors follow implementation contract
		assert.Nil(t, r.GetPeer(1))
		rel, nextHop := r.GetRelation(1, 2)
		assert.Equal(t, types.TopologyRelationType_Invalid, rel)
		assert.Nil(t, nextHop)

		// ForeachPeer returns true when registry is nil (nothing to iterate)
		called := 0
		ok := r.ForeachPeer(func(peer types.TopologyPeer) bool {
			called++
			return true
		})
		assert.True(t, ok)
		assert.Equal(t, 0, called)

		// UpdatePeer / RemovePeer should not panic
		r.UpdatePeer(1, 0, makeTopologyData(1, "h1", nil))
		r.RemovePeer(1)
	})
}
