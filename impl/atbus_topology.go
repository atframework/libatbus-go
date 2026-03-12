// Package libatbus_impl provides internal implementation details for libatbus.

package libatbus_impl

import (
	types "github.com/atframework/libatbus-go/types"
)

type TopologyPeer struct {
	busId            types.BusIdType
	proactivelyAdded bool

	upstream   *TopologyPeer
	downstream map[types.BusIdType]*TopologyPeer

	topologyData types.TopologyData
}

var _ types.TopologyPeer = (*TopologyPeer)(nil)

type TopologyRegistry struct {
	data map[types.BusIdType]*TopologyPeer
}

var _ types.TopologyRegistry = (*TopologyRegistry)(nil)

func CreateTopologyPeer(busId types.BusIdType) *TopologyPeer {
	return &TopologyPeer{
		busId: busId,
	}
}

func (p *TopologyPeer) GetBusId() types.BusIdType {
	if p == nil {
		return 0
	}

	return p.busId
}

func (p *TopologyPeer) GetUpstream() types.TopologyPeer {
	if p == nil {
		return nil
	}

	// Avoid returning a typed-nil interface (which is != nil and may break relation checks).
	if p.upstream == nil {
		return nil
	}

	return p.upstream
}

func (p *TopologyPeer) GetTopologyData() *types.TopologyData {
	if p == nil {
		return nil
	}

	return &p.topologyData
}

func (p *TopologyPeer) ContainsDownstream(busId types.BusIdType) bool {
	if p != nil && p.downstream != nil {
		s, ok := p.downstream[busId]
		return ok && s != nil
	}

	return false
}

func (p *TopologyPeer) ForeachDownstream(fn func(peer types.TopologyPeer) bool) bool {
	if p == nil || p.downstream == nil {
		return true
	}

	ret := true

	invalidKeys := make([]types.BusIdType, 0)
	for key, peer := range p.downstream {
		if peer == nil {
			invalidKeys = append(invalidKeys, key)
			continue
		}

		if !fn(peer) {
			ret = false
			break
		}
	}

	for _, k := range invalidKeys {
		delete(p.downstream, k)
	}

	return ret
}

func (p *TopologyPeer) setProactivelyAdded(v bool) {
	if p == nil {
		return
	}

	p.proactivelyAdded = v
}

func (p *TopologyPeer) getProactivelyAdded() bool {
	if p == nil {
		return false
	}

	return p.proactivelyAdded
}

func (p *TopologyPeer) updateUpstream(upstream *TopologyPeer) {
	if p == nil {
		return
	}

	p.upstream = upstream
}

func (p *TopologyPeer) updateData(data *types.TopologyData) {
	if p == nil || data == nil {
		return
	}

	p.topologyData = *data
}

func (p *TopologyPeer) addDownstream(downstream *TopologyPeer) {
	if p == nil || downstream == nil {
		return
	}

	if p.downstream == nil {
		p.downstream = make(map[types.BusIdType]*TopologyPeer)
	}

	p.downstream[downstream.busId] = downstream
}

func (p *TopologyPeer) removeDownstream(downStreamBusId types.BusIdType, check *TopologyPeer) {
	if p == nil {
		return
	}

	if p.downstream == nil {
		return
	}

	s, ok := p.downstream[downStreamBusId]
	if ok && (check == nil || s == check) {
		delete(p.downstream, downStreamBusId)
	}
}

func CreateTopologyRegistry() *TopologyRegistry {
	return &TopologyRegistry{
		data: make(map[types.BusIdType]*TopologyPeer),
	}
}

func (r *TopologyRegistry) GetPeerInstance(busId types.BusIdType) *TopologyPeer {
	if r == nil {
		return nil
	}

	peer, ok := r.data[busId]
	if !ok {
		return nil
	}

	return peer
}

func (r *TopologyRegistry) GetPeer(busId types.BusIdType) types.TopologyPeer {
	ret := r.GetPeerInstance(busId)
	if ret == nil {
		return nil
	}

	return ret
}

func (r *TopologyRegistry) RemovePeer(targetBusId types.BusIdType) {
	if r == nil {
		return
	}

	p, exists := r.data[targetBusId]
	if !exists {
		return
	}

	if p == nil {
		delete(r.data, targetBusId)
		return
	}

	p.setProactivelyAdded(false)
	// 如果还有下游节点，不能删除
	if len(p.downstream) == 0 {
		delete(r.data, targetBusId)
	}

	// remove from upstream
	if p.upstream != nil {
		p.upstream.removeDownstream(targetBusId, p)

		// 如果上游是被动添加的，且没有下游了，则递归删除
		if !p.upstream.getProactivelyAdded() && len(p.upstream.downstream) == 0 {
			r.RemovePeer(p.upstream.busId)
		}
	}
}

func (r *TopologyRegistry) UpdatePeer(targetBusId types.BusIdType, upstreamBusId types.BusIdType, data *types.TopologyData) bool {
	if r == nil || targetBusId == 0 {
		return false
	}

	// Reject trivial self-loop (including the case where the peer does not exist yet).
	if targetBusId == upstreamBusId {
		return false
	}

	peer := r.GetPeerInstance(targetBusId)
	var upstream *TopologyPeer = nil
	if upstreamBusId != 0 {
		upstream = r.mutablePeer(upstreamBusId)
	}

	// 更新老的关系
	if peer != nil {
		peer.setProactivelyAdded(true)

		// 数据更新
		if data != nil {
			peer.updateData(data)
		}

		// 路由关系未更新，不用走后面的流程
		if peer.GetUpstream() == upstream {
			return true
		}

		// 检查成环
		cur := upstream
		for cur != nil {
			if cur.busId == targetBusId {
				// 成环，拒绝更新
				return false
			}
			cur = cur.upstream
		}

		// 解除旧的上游关系
		if peer.upstream != nil {
			peer.upstream.removeDownstream(targetBusId, peer)
		}
		peer.updateUpstream(upstream)

		if upstream != nil {
			upstream.addDownstream(peer)
		}
		return true
	}

	// 创建新的节点
	peer = r.mutablePeer(targetBusId)
	peer.setProactivelyAdded(true)
	// 数据更新
	if data != nil {
		peer.updateData(data)
	}
	peer.updateUpstream(upstream)
	if upstream != nil {
		upstream.addDownstream(peer)
	}

	return true
}

func (r *TopologyRegistry) GetRelation(from types.BusIdType, to types.BusIdType) (types.TopologyRelationType, types.TopologyPeer) {
	if from == 0 || to == 0 || r == nil {
		return types.TopologyRelationType_Invalid, nil
	}

	fromPeer := r.GetPeerInstance(from)
	toPeer := r.GetPeerInstance(to)
	if fromPeer == nil || toPeer == nil {
		return types.TopologyRelationType_Invalid, nil
	}

	// check upstream
	if from == to {
		return types.TopologyRelationType_Self, toPeer
	}

	fromPeerUpstream := fromPeer.GetUpstream()
	if fromPeerUpstream == toPeer {
		return types.TopologyRelationType_ImmediateUpstream, toPeer
	}

	toPeerUpstream := toPeer.GetUpstream()
	if toPeerUpstream == fromPeer {
		return types.TopologyRelationType_ImmediateDownstream, toPeer
	}

	if fromPeerUpstream != nil && fromPeerUpstream == toPeerUpstream {
		return types.TopologyRelationType_SameUpstreamPeer, fromPeerUpstream
	}

	// check TransitiveUpstream
	for fromPeerUpstream != nil {
		fromPeerUpstream = fromPeerUpstream.GetUpstream()
		if fromPeerUpstream == toPeer {
			return types.TopologyRelationType_TransitiveUpstream, fromPeer.GetUpstream()
		}
	}

	// check TransitiveDownstream
	var previousToPeerUpstream types.TopologyPeer
	for toPeerUpstream != nil {
		previousToPeerUpstream = toPeerUpstream
		toPeerUpstream = toPeerUpstream.GetUpstream()
		if toPeerUpstream == fromPeer {
			return types.TopologyRelationType_TransitiveDownstream, previousToPeerUpstream
		}
	}

	if fromPeer.GetUpstream() != nil {
		return types.TopologyRelationType_OtherUpstreamPeer, fromPeer.GetUpstream()
	}

	return types.TopologyRelationType_OtherUpstreamPeer, toPeer
}

func (r *TopologyRegistry) ForeachPeer(fn func(peer types.TopologyPeer) bool) bool {
	if r == nil {
		return true
	}

	for _, peer := range r.data {
		if !fn(peer) {
			return false
		}
	}

	return true
}

func (r *TopologyRegistry) CheckPolicy(rule *types.TopologyPolicyRule, fromData *types.TopologyData, toData *types.TopologyData) bool {
	if rule == nil {
		return true
	}

	if fromData == nil || toData == nil {
		return false
	}

	if rule.RequireSameProcess || rule.RequireSameHostName {
		if fromData.Hostname != toData.Hostname {
			return false
		}

		if rule.RequireSameProcess && fromData.Pid != toData.Pid {
			return false
		}
	}

	for labelKey, labelValues := range rule.RequireLabelValues {
		toValue, toOk := toData.Labels[labelKey]
		if !toOk {
			return false
		}

		_, exists := labelValues[toValue]
		if !exists {
			return false
		}
	}

	return true
}

func (r *TopologyRegistry) mutablePeer(busId types.BusIdType) *TopologyPeer {
	if r == nil {
		return nil
	}

	peer := r.GetPeerInstance(busId)
	if peer == nil {
		peer = CreateTopologyPeer(busId)
		r.data[busId] = peer
	}

	return peer
}
