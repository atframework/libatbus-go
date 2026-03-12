package libatbus_types

// TopologyRelationType represents the relation type between two peers in the topology registry.
// The relation is evaluated based on the upstream chain (parent links).
type TopologyRelationType uint8

const (
	// TopologyRelationType_Invalid indicates invalid input or one/both peers not found.
	TopologyRelationType_Invalid TopologyRelationType = 0
	// TopologyRelationType_Self indicates from == to.
	TopologyRelationType_Self TopologyRelationType = 1
	// TopologyRelationType_ImmediateUpstream indicates to is the direct upstream(parent) of from.
	TopologyRelationType_ImmediateUpstream TopologyRelationType = 2
	// TopologyRelationType_TransitiveUpstream indicates to is an ancestor of from, but not the direct upstream.
	TopologyRelationType_TransitiveUpstream TopologyRelationType = 3
	// TopologyRelationType_ImmediateDownstream indicates to is the direct downstream(child) of from.
	TopologyRelationType_ImmediateDownstream TopologyRelationType = 4
	// TopologyRelationType_TransitiveDownstream indicates to is a descendant of from, but not the direct downstream.
	TopologyRelationType_TransitiveDownstream TopologyRelationType = 5
	// TopologyRelationType_SameUpstreamPeer indicates from and to share the same direct upstream.
	TopologyRelationType_SameUpstreamPeer TopologyRelationType = 6
	// TopologyRelationType_OtherUpstreamPeer indicates from and to do not fall into any of the above categories.
	TopologyRelationType_OtherUpstreamPeer TopologyRelationType = 7
)

// TopologyPolicyRule is a policy rule used by TopologyRegistry.CheckPolicy.
// It describes constraints that the "to" peer must satisfy.
type TopologyPolicyRule struct {
	// RequireSameProcess requires same pid and same hostname if true.
	RequireSameProcess bool
	// RequireSameHostName requires same hostname if true.
	RequireSameHostName bool
	// RequireLabelValues contains label constraints.
	// Key: label name.
	// Value: allowed label values (set).
	RequireLabelValues map[string]map[string]struct{}
}

// TopologyData is the runtime topology data associated with a peer.
type TopologyData struct {
	// Pid is the process id of the peer.
	Pid int32
	// Hostname is the hostname of the peer.
	Hostname string
	// Labels contains arbitrary labels for policy matching (e.g. region/zone/group/version).
	Labels map[string]string
}

// TopologyPeer is a topology node (peer).
// A peer has:
//   - A stable bus id.
//   - An optional upstream peer.
//   - A set of downstream peers.
//   - Associated TopologyData.
type TopologyPeer interface {
	// GetBusId returns the bus id of this peer.
	GetBusId() BusIdType

	// GetUpstream returns the upstream (parent) peer, or nil if this peer is a root.
	GetUpstream() TopologyPeer

	// GetTopologyData returns current topology data of this peer.
	GetTopologyData() *TopologyData

	// ContainsDownstream checks whether a downstream(peer) with given bus id exists.
	ContainsDownstream(busId BusIdType) bool

	// ForeachDownstream iterates all downstream peers.
	ForeachDownstream(fn func(peer TopologyPeer) bool) bool
}

// TopologyRegistry provides CRUD for peers and relation querying.
// This is an in-process data structure. It is NOT thread-safe.
type TopologyRegistry interface {
	// GetPeer returns peer by bus id, or nil if not found.
	GetPeer(busId BusIdType) TopologyPeer

	// RemovePeer removes a peer and fixes relationships.
	// If the peer has an upstream, it will be removed from the upstream's downstream set.
	// For each downstream peer whose upstream is this peer, its upstream will be cleared.
	RemovePeer(targetBusId BusIdType)

	// UpdatePeer creates or updates a peer.
	//   - If targetBusId is 0, this call is ignored.
	//   - If upstreamBusId is 0, the peer becomes a root (no upstream).
	//   - If peer exists and upstream changed, downstream links will be updated accordingly.
	// true on success, false on failure (e.g. invalid target_bus_id or there will be a circle).
	UpdatePeer(targetBusId BusIdType, upstreamBusId BusIdType, data *TopologyData) bool

	// GetRelation returns the topology relation between two peers.
	// The first return value is the relation type.
	// The second return value is the next-hop peer (may be nil):
	//   - If the relation is TransitiveUpstream or TransitiveDownstream, it's to.
	//   - If the relation is SameUpstreamPeer, it's the upstream of both from and to.
	//   - If the relation is OtherUpstreamPeer, it prefers the upstream of from;
	//     if from has no upstream then returns to.
	GetRelation(from BusIdType, to BusIdType) (TopologyRelationType, TopologyPeer)

	// ForeachPeer iterates all peers in registry.
	// The callback function returns false to stop iteration early.
	// Returns true if iterated all peers, false if stopped by callback.
	ForeachPeer(fn func(peer TopologyPeer) bool) bool

	// CheckPolicy checks whether toData satisfies the fromPolicy.
	// The checks include:
	//   - same hostname (optional)
	//   - same process (optional, implies same hostname)
	//   - required labels (optional)
	CheckPolicy(rule *TopologyPolicyRule, fromData *TopologyData, toData *TopologyData) bool
}
