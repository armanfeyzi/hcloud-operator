package hcloud

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sync"
)

// FakeClient is an in-memory implementation of Interface for use in tests.
// All operations are goroutine-safe and stored in a simple map keyed by server ID.
type FakeClient struct {
	mu            sync.Mutex
	servers       map[int64]*ServerInfo
	volumes       map[int64]*VolumeInfo
	loadBalancers map[int64]*LoadBalancerInfo
	networks      map[int64]*NetworkInfo
	nextID        int64

	// CreateErr, if non-nil, is returned by every CreateServer call.
	CreateErr error
	// GetErr, if non-nil, is returned by every GetServer call.
	GetErr error
	// DeleteErr, if non-nil, is returned by every DeleteServer call.
	DeleteErr error
}

// NewFakeClient returns an empty FakeClient ready for use in tests.
func NewFakeClient() *FakeClient {
	return &FakeClient{
		servers:       make(map[int64]*ServerInfo),
		volumes:       make(map[int64]*VolumeInfo),
		loadBalancers: make(map[int64]*LoadBalancerInfo),
		networks:      make(map[int64]*NetworkInfo),
		nextID:        1,
	}
}

// Reset clears all stored resources and injected errors. Use between Ginkgo specs when sharing one FakeClient.
func (f *FakeClient) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.servers = make(map[int64]*ServerInfo)
	f.volumes = make(map[int64]*VolumeInfo)
	f.loadBalancers = make(map[int64]*LoadBalancerInfo)
	f.networks = make(map[int64]*NetworkInfo)
	f.nextID = 1
	f.CreateErr = nil
	f.GetErr = nil
	f.DeleteErr = nil
}

func (f *FakeClient) GetServer(ctx context.Context, id int64) (*ServerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	s, ok := f.servers[id]
	if !ok {
		return nil, nil
	}
	return copyServerInfo(s), nil
}

func (f *FakeClient) GetServerByName(ctx context.Context, name string) (*ServerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	for _, s := range f.servers {
		if s.Name == name {
			return copyServerInfo(s), nil
		}
	}
	return nil, nil
}

func (f *FakeClient) CreateServer(ctx context.Context, opts ServerCreateOpts) (*ServerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.CreateErr != nil {
		return nil, f.CreateErr
	}

	// Simulate uniqueness constraint.
	for _, s := range f.servers {
		if s.Name == opts.Name {
			return nil, fmt.Errorf("fake: server with name %q already exists", opts.Name)
		}
	}

	id := f.nextID
	f.nextID++

	info := &ServerInfo{
		ID:         id,
		Name:       opts.Name,
		ServerType: opts.ServerType,
		State:      "running",
		PublicIPv4: fmt.Sprintf("1.2.3.%d", id),
		PublicIPv6: fmt.Sprintf("2001:db8::%d/64", id),
		NetworkIDs: nil,
	}
	f.servers[id] = info
	return copyServerInfo(info), nil
}

func (f *FakeClient) DeleteServer(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	delete(f.servers, id)
	return nil
}

// PowerOffServer simulates an immediate transition to off.
func (f *FakeClient) PowerOffServer(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.servers[id]
	if !ok {
		return fmt.Errorf("fake: server %d not found", id)
	}
	if s.State == "off" || s.State == "stopping" {
		return nil
	}
	s.State = "off"
	return nil
}

// PowerOnServer simulates an immediate transition to running.
func (f *FakeClient) PowerOnServer(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.servers[id]
	if !ok {
		return fmt.Errorf("fake: server %d not found", id)
	}
	if s.State == "running" || s.State == "starting" {
		return nil
	}
	s.State = "running"
	return nil
}

// ChangeServerType updates the stored type when the fake server is off.
func (f *FakeClient) ChangeServerType(ctx context.Context, id int64, serverType string, upgradeDisk bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.servers[id]
	if !ok {
		return fmt.Errorf("fake: server %d not found", id)
	}
	if s.ServerType == serverType {
		return nil
	}
	if s.State != "off" {
		return fmt.Errorf("fake: server %d must be off to change type (state=%s)", id, s.State)
	}
	s.ServerType = serverType
	return nil
}

func (f *FakeClient) AttachServerToNetwork(ctx context.Context, serverID int64, networkID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, ok := f.servers[serverID]
	if !ok {
		return fmt.Errorf("fake: server %d not found", serverID)
	}
	if _, ok := f.networks[networkID]; !ok {
		return fmt.Errorf("fake: network %d not found", networkID)
	}
	for _, existing := range s.NetworkIDs {
		if existing == networkID {
			return nil
		}
	}
	s.NetworkIDs = append(s.NetworkIDs, networkID)
	slices.Sort(s.NetworkIDs)
	return nil
}

// Len returns the number of servers currently tracked by the fake.
func (f *FakeClient) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.servers)
}

// LenVolumes returns the number of volumes currently tracked by the fake.
func (f *FakeClient) LenVolumes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.volumes)
}

// LenLoadBalancers returns the number of load balancers currently tracked by the fake.
func (f *FakeClient) LenLoadBalancers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.loadBalancers)
}

func (f *FakeClient) GetVolume(ctx context.Context, id int64) (*VolumeInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	v, ok := f.volumes[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (f *FakeClient) GetVolumeByName(ctx context.Context, name string) (*VolumeInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	for _, v := range f.volumes {
		if v.Name == name {
			cp := *v
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *FakeClient) CreateVolume(ctx context.Context, opts VolumeCreateOpts) (*VolumeInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.CreateErr != nil {
		return nil, f.CreateErr
	}

	for _, v := range f.volumes {
		if v.Name == opts.Name {
			return nil, fmt.Errorf("fake: volume with name %q already exists", opts.Name)
		}
	}

	id := f.nextID
	f.nextID++

	info := &VolumeInfo{
		ID:          id,
		Name:        opts.Name,
		State:       "available",
		LinuxDevice: fmt.Sprintf("/dev/disk/by-id/scsi-0HC_Volume_%d", id),
	}

	if opts.ServerID > 0 {
		info.ServerID = opts.ServerID
	}

	f.volumes[id] = info
	cp := *info
	return &cp, nil
}

func (f *FakeClient) DeleteVolume(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	delete(f.volumes, id)
	return nil
}

func (f *FakeClient) AttachVolume(ctx context.Context, volumeID int64, serverID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	v, ok := f.volumes[volumeID]
	if !ok {
		return fmt.Errorf("fake: volume %d not found", volumeID)
	}
	if v.ServerID > 0 {
		return fmt.Errorf("fake: volume %d already attached", volumeID)
	}

	v.ServerID = serverID
	return nil
}

func (f *FakeClient) DetachVolume(ctx context.Context, volumeID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	v, ok := f.volumes[volumeID]
	if !ok {
		return fmt.Errorf("fake: volume %d not found", volumeID)
	}

	v.ServerID = 0
	return nil
}

func (f *FakeClient) GetLoadBalancer(ctx context.Context, id int64) (*LoadBalancerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	lb, ok := f.loadBalancers[id]
	if !ok {
		return nil, nil
	}
	cp := *lb
	cp.Targets = append([]int64{}, lb.Targets...)
	return &cp, nil
}

func (f *FakeClient) GetLoadBalancerByName(ctx context.Context, name string) (*LoadBalancerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	for _, lb := range f.loadBalancers {
		if lb.Name == name {
			cp := *lb
			cp.Targets = append([]int64{}, lb.Targets...)
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *FakeClient) CreateLoadBalancer(ctx context.Context, opts LoadBalancerCreateOpts) (*LoadBalancerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.CreateErr != nil {
		return nil, f.CreateErr
	}

	for _, lb := range f.loadBalancers {
		if lb.Name == opts.Name {
			return nil, fmt.Errorf("fake: load balancer with name %q already exists", opts.Name)
		}
	}

	id := f.nextID
	f.nextID++

	info := &LoadBalancerInfo{
		ID:         id,
		Name:       opts.Name,
		PublicIPv4: fmt.Sprintf("5.6.7.%d", id),
		PublicIPv6: fmt.Sprintf("2001:db8:1::%d", id),
		Targets:    []int64{},
	}

	f.loadBalancers[id] = info
	cp := *info
	cp.Targets = append([]int64{}, info.Targets...)
	return &cp, nil
}

func (f *FakeClient) DeleteLoadBalancer(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	delete(f.loadBalancers, id)
	return nil
}

func (f *FakeClient) AttachServerToLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	lb, ok := f.loadBalancers[loadBalancerID]
	if !ok {
		return fmt.Errorf("fake: load balancer %d not found", loadBalancerID)
	}
	if _, ok := f.servers[serverID]; !ok {
		return fmt.Errorf("fake: server %d not found", serverID)
	}
	for _, existing := range lb.Targets {
		if existing == serverID {
			return nil
		}
	}
	lb.Targets = append(lb.Targets, serverID)
	return nil
}

func (f *FakeClient) DetachServerFromLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	lb, ok := f.loadBalancers[loadBalancerID]
	if !ok {
		return fmt.Errorf("fake: load balancer %d not found", loadBalancerID)
	}
	filtered := lb.Targets[:0]
	for _, id := range lb.Targets {
		if id != serverID {
			filtered = append(filtered, id)
		}
	}
	lb.Targets = filtered
	return nil
}

func (f *FakeClient) GetNetwork(ctx context.Context, id int64) (*NetworkInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	n, ok := f.networks[id]
	if !ok {
		return nil, nil
	}
	return copyNetworkInfo(n), nil
}

func (f *FakeClient) GetNetworkByName(ctx context.Context, name string) (*NetworkInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	for _, n := range f.networks {
		if n.Name == name {
			return copyNetworkInfo(n), nil
		}
	}
	return nil, nil
}

func (f *FakeClient) CreateNetwork(ctx context.Context, opts NetworkCreateOpts) (*NetworkInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.CreateErr != nil {
		return nil, f.CreateErr
	}
	if _, _, err := net.ParseCIDR(opts.IPRange); err != nil {
		return nil, fmt.Errorf("fake: invalid ipRange %q: %w", opts.IPRange, err)
	}
	for _, n := range f.networks {
		if n.Name == opts.Name {
			return nil, fmt.Errorf("fake: network with name %q already exists", opts.Name)
		}
	}

	id := f.nextID
	f.nextID++

	info := &NetworkInfo{
		ID:           id,
		Name:         opts.Name,
		IPRange:      opts.IPRange,
		SubnetZones:  nil,
		Labels:       cloneStringMap(opts.Labels),
		ExposeRoutes: opts.ExposeRoutesToVSwitch,
	}
	f.networks[id] = info
	return copyNetworkInfo(info), nil
}

func (f *FakeClient) DeleteNetwork(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	delete(f.networks, id)
	return nil
}

func (f *FakeClient) AddNetworkCloudSubnet(ctx context.Context, networkID int64, zone string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	n, ok := f.networks[networkID]
	if !ok {
		return fmt.Errorf("fake: network %d not found", networkID)
	}
	for _, z := range n.SubnetZones {
		if z == zone {
			return nil
		}
	}
	n.SubnetZones = append(n.SubnetZones, zone)
	slices.Sort(n.SubnetZones)
	return nil
}

func copyNetworkInfo(n *NetworkInfo) *NetworkInfo {
	if n == nil {
		return nil
	}
	cp := *n
	cp.SubnetZones = append([]string{}, n.SubnetZones...)
	cp.Labels = cloneStringMap(n.Labels)
	return &cp
}

func copyServerInfo(s *ServerInfo) *ServerInfo {
	if s == nil {
		return nil
	}
	cp := *s
	cp.NetworkIDs = append([]int64{}, s.NetworkIDs...)
	return &cp
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
