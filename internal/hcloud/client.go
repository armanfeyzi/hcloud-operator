package hcloud

import (
	"context"
	"fmt"
	"net"

	hcloudgo "github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// ServerInfo is a minimal, SDK-agnostic view of a Hetzner Cloud server.
// The controller uses this type so it doesn't import the hcloud-go SDK directly.
type ServerInfo struct {
	ID         int64
	Name       string
	ServerType string
	State      string
	PublicIPv4 string
	PublicIPv6 string
	NetworkIDs []int64
}

// ServerCreateOpts holds the parameters for creating a Hetzner Cloud server.
type ServerCreateOpts struct {
	Name       string
	ServerType string
	Image      string
	Location   string
	Labels     map[string]string
	SSHKeys    []string
	UserData   string
}

// VolumeInfo is a minimal, SDK-agnostic view of a Hetzner Cloud volume.
type VolumeInfo struct {
	ID          int64
	Name        string
	State       string
	ServerID    int64 // 0 if unattached
	LinuxDevice string
}

// VolumeCreateOpts holds the parameters for creating a Hetzner Cloud volume.
type VolumeCreateOpts struct {
	Name      string
	Size      int
	ServerID  int64  // optional, if > 0 the volume is attached to this server during creation
	Location  string // required if ServerID is 0
	Format    string // optional filesystem format
	Automount bool   // if true, Hetzner attempts to mount it automatically
	Labels    map[string]string
}

// LoadBalancerInfo is a minimal, SDK-agnostic view of a Hetzner Cloud load balancer.
type LoadBalancerInfo struct {
	ID         int64
	Name       string
	PublicIPv4 string
	PublicIPv6 string
	Targets    []int64
}

// LoadBalancerCreateOpts holds the parameters for creating a Hetzner Cloud load balancer.
type LoadBalancerCreateOpts struct {
	Name             string
	LoadBalancerType string
	Location         string
	NetworkZone      string
	Algorithm        string
	Labels           map[string]string
}

// NetworkInfo is a minimal view of a Hetzner Cloud private network.
type NetworkInfo struct {
	ID           int64
	Name         string
	IPRange      string
	SubnetZones  []string
	Labels       map[string]string
	ExposeRoutes bool
}

// NetworkCreateOpts holds parameters for creating a private network.
type NetworkCreateOpts struct {
	Name                  string
	IPRange               string
	NetworkZones          []string
	Labels                map[string]string
	ExposeRoutesToVSwitch bool
}

// Interface defines the Hetzner Cloud operations required by the controller.
// Using an interface here allows the controller to be tested with a fake client
// without making real API calls.
type Interface interface {
	GetServer(ctx context.Context, id int64) (*ServerInfo, error)
	GetServerByName(ctx context.Context, name string) (*ServerInfo, error)
	CreateServer(ctx context.Context, opts ServerCreateOpts) (*ServerInfo, error)
	DeleteServer(ctx context.Context, id int64) error
	PowerOffServer(ctx context.Context, id int64) error
	PowerOnServer(ctx context.Context, id int64) error
	ChangeServerType(ctx context.Context, id int64, serverType string, upgradeDisk bool) error
	AttachServerToNetwork(ctx context.Context, serverID int64, networkID int64) error

	GetVolume(ctx context.Context, id int64) (*VolumeInfo, error)
	GetVolumeByName(ctx context.Context, name string) (*VolumeInfo, error)
	CreateVolume(ctx context.Context, opts VolumeCreateOpts) (*VolumeInfo, error)
	DeleteVolume(ctx context.Context, id int64) error
	AttachVolume(ctx context.Context, volumeID int64, serverID int64) error
	DetachVolume(ctx context.Context, volumeID int64) error

	GetLoadBalancer(ctx context.Context, id int64) (*LoadBalancerInfo, error)
	GetLoadBalancerByName(ctx context.Context, name string) (*LoadBalancerInfo, error)
	CreateLoadBalancer(ctx context.Context, opts LoadBalancerCreateOpts) (*LoadBalancerInfo, error)
	DeleteLoadBalancer(ctx context.Context, id int64) error
	AttachServerToLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error
	DetachServerFromLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error

	GetNetwork(ctx context.Context, id int64) (*NetworkInfo, error)
	GetNetworkByName(ctx context.Context, name string) (*NetworkInfo, error)
	CreateNetwork(ctx context.Context, opts NetworkCreateOpts) (*NetworkInfo, error)
	DeleteNetwork(ctx context.Context, id int64) error
	AddNetworkCloudSubnet(ctx context.Context, networkID int64, zone string) error
}

// Client wraps the Hetzner Cloud API client with idempotent helpers.
// It implements Interface.
type Client struct {
	hc *hcloudgo.Client
}

// NewClient creates a new Hetzner Cloud API client authenticated with the given token.
func NewClient(token string) *Client {
	return &Client{
		hc: hcloudgo.NewClient(hcloudgo.WithToken(token)),
	}
}

// GetServer fetches a server by its Hetzner Cloud ID.
// Returns nil, nil if the server does not exist.
func (c *Client) GetServer(ctx context.Context, id int64) (*ServerInfo, error) {
	s, _, err := c.hc.Server.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetServer(%d): %w", id, err)
	}
	if s == nil {
		return nil, nil
	}
	return toServerInfo(s), nil
}

// GetServerByName fetches a server by its name.
// Returns nil, nil if the server does not exist.
func (c *Client) GetServerByName(ctx context.Context, name string) (*ServerInfo, error) {
	s, _, err := c.hc.Server.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetServerByName(%q): %w", name, err)
	}
	if s == nil {
		return nil, nil
	}
	return toServerInfo(s), nil
}

// CreateServer provisions a new Hetzner Cloud server.
func (c *Client) CreateServer(ctx context.Context, opts ServerCreateOpts) (*ServerInfo, error) {
	serverType, _, err := c.hc.ServerType.GetByName(ctx, opts.ServerType)
	if err != nil {
		return nil, fmt.Errorf("hcloud: resolve server type %q: %w", opts.ServerType, err)
	}
	if serverType == nil {
		return nil, fmt.Errorf("hcloud: server type %q not found", opts.ServerType)
	}

	image, _, err := c.hc.Image.GetByName(ctx, opts.Image)
	if err != nil {
		return nil, fmt.Errorf("hcloud: resolve image %q: %w", opts.Image, err)
	}
	if image == nil {
		return nil, fmt.Errorf("hcloud: image %q not found", opts.Image)
	}

	location, _, err := c.hc.Location.GetByName(ctx, opts.Location)
	if err != nil {
		return nil, fmt.Errorf("hcloud: resolve location %q: %w", opts.Location, err)
	}
	if location == nil {
		return nil, fmt.Errorf("hcloud: location %q not found", opts.Location)
	}

	var sshKeys []*hcloudgo.SSHKey
	for _, key := range opts.SSHKeys {
		k, _, err := c.hc.SSHKey.GetByName(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("hcloud: resolve ssh key %q: %w", key, err)
		}
		if k != nil {
			sshKeys = append(sshKeys, k)
		}
	}

	result, _, err := c.hc.Server.Create(ctx, hcloudgo.ServerCreateOpts{
		Name:       opts.Name,
		ServerType: serverType,
		Image:      image,
		Location:   location,
		Labels:     opts.Labels,
		SSHKeys:    sshKeys,
		UserData:   opts.UserData,
	})
	if err != nil {
		return nil, fmt.Errorf("hcloud: CreateServer %q: %w", opts.Name, err)
	}

	return toServerInfo(result.Server), nil
}

// DeleteServer destroys a Hetzner Cloud server by ID.
// Idempotent — returns nil if the server does not exist.
func (c *Client) DeleteServer(ctx context.Context, id int64) error {
	s, _, err := c.hc.Server.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetServer(%d): %w", id, err)
	}
	if s == nil {
		return nil
	}
	if _, _, err = c.hc.Server.DeleteWithResult(ctx, s); err != nil {
		return fmt.Errorf("hcloud: DeleteServer(%d): %w", id, err)
	}
	return nil
}

// PowerOffServer requests a graceful power-off for the server. Idempotent if already off.
func (c *Client) PowerOffServer(ctx context.Context, id int64) error {
	s, _, err := c.hc.Server.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetServer(%d): %w", id, err)
	}
	if s == nil {
		return fmt.Errorf("hcloud: server %d not found", id)
	}
	if s.Status == hcloudgo.ServerStatusOff || s.Status == hcloudgo.ServerStatusStopping {
		return nil
	}
	if _, _, err := c.hc.Server.Poweroff(ctx, s); err != nil {
		return fmt.Errorf("hcloud: PowerOffServer(%d): %w", id, err)
	}
	return nil
}

// PowerOnServer powers on a stopped server. Idempotent if already running.
func (c *Client) PowerOnServer(ctx context.Context, id int64) error {
	s, _, err := c.hc.Server.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetServer(%d): %w", id, err)
	}
	if s == nil {
		return fmt.Errorf("hcloud: server %d not found", id)
	}
	if s.Status == hcloudgo.ServerStatusRunning || s.Status == hcloudgo.ServerStatusStarting {
		return nil
	}
	if _, _, err := c.hc.Server.Poweron(ctx, s); err != nil {
		return fmt.Errorf("hcloud: PowerOnServer(%d): %w", id, err)
	}
	return nil
}

// ChangeServerType changes a server's type. The server must be off. Idempotent if already the requested type.
func (c *Client) ChangeServerType(ctx context.Context, id int64, serverTypeName string, upgradeDisk bool) error {
	s, _, err := c.hc.Server.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetServer(%d): %w", id, err)
	}
	if s == nil {
		return fmt.Errorf("hcloud: server %d not found", id)
	}
	if s.ServerType != nil && s.ServerType.Name == serverTypeName {
		return nil
	}
	serverType, _, err := c.hc.ServerType.GetByName(ctx, serverTypeName)
	if err != nil {
		return fmt.Errorf("hcloud: resolve server type %q: %w", serverTypeName, err)
	}
	if serverType == nil {
		return fmt.Errorf("hcloud: server type %q not found", serverTypeName)
	}
	if _, _, err := c.hc.Server.ChangeType(ctx, s, hcloudgo.ServerChangeTypeOpts{
		ServerType:  serverType,
		UpgradeDisk: upgradeDisk,
	}); err != nil {
		return fmt.Errorf("hcloud: ChangeServerType(%d, %q): %w", id, serverTypeName, err)
	}
	return nil
}

// toServerInfo converts a raw hcloud-go Server into our SDK-agnostic ServerInfo.
func toServerInfo(s *hcloudgo.Server) *ServerInfo {
	info := &ServerInfo{
		ID:    s.ID,
		Name:  s.Name,
		State: string(s.Status),
	}
	if s.ServerType != nil {
		info.ServerType = s.ServerType.Name
	}
	if s.PublicNet.IPv4.IP != nil {
		info.PublicIPv4 = s.PublicNet.IPv4.IP.String()
	}
	if s.PublicNet.IPv6.Network != nil {
		info.PublicIPv6 = s.PublicNet.IPv6.Network.String()
	}
	for _, pn := range s.PrivateNet {
		if pn.Network != nil {
			info.NetworkIDs = append(info.NetworkIDs, pn.Network.ID)
		}
	}
	return info
}

// AttachServerToNetwork attaches an existing server to an existing private network.
func (c *Client) AttachServerToNetwork(ctx context.Context, serverID int64, networkID int64) error {
	server, _, err := c.hc.Server.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch server %d: %w", serverID, err)
	}
	if server == nil {
		return fmt.Errorf("hcloud: server %d not found", serverID)
	}
	for _, pn := range server.PrivateNet {
		if pn.Network != nil && pn.Network.ID == networkID {
			return nil
		}
	}

	network, _, err := c.hc.Network.GetByID(ctx, networkID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch network %d: %w", networkID, err)
	}
	if network == nil {
		return fmt.Errorf("hcloud: network %d not found", networkID)
	}

	_, _, err = c.hc.Server.AttachToNetwork(ctx, server, hcloudgo.ServerAttachToNetworkOpts{
		Network: network,
	})
	if err != nil {
		return fmt.Errorf("hcloud: AttachServerToNetwork(%d, %d): %w", serverID, networkID, err)
	}
	return nil
}

// GetVolume fetches a volume by its Hetzner Cloud ID.
func (c *Client) GetVolume(ctx context.Context, id int64) (*VolumeInfo, error) {
	v, _, err := c.hc.Volume.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetVolume(%d): %w", id, err)
	}
	if v == nil {
		return nil, nil
	}
	return toVolumeInfo(v), nil
}

// GetVolumeByName fetches a volume by its name.
func (c *Client) GetVolumeByName(ctx context.Context, name string) (*VolumeInfo, error) {
	v, _, err := c.hc.Volume.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetVolumeByName(%q): %w", name, err)
	}
	if v == nil {
		return nil, nil
	}
	return toVolumeInfo(v), nil
}

// CreateVolume provisions a new Hetzner Cloud volume.
func (c *Client) CreateVolume(ctx context.Context, opts VolumeCreateOpts) (*VolumeInfo, error) {
	createOpts := hcloudgo.VolumeCreateOpts{
		Name:      opts.Name,
		Size:      opts.Size,
		Automount: hcloudgo.Ptr(opts.Automount),
		Labels:    opts.Labels,
	}

	if opts.Format != "" {
		createOpts.Format = hcloudgo.Ptr(opts.Format)
	}

	if opts.ServerID > 0 {
		server, _, err := c.hc.Server.GetByID(ctx, opts.ServerID)
		if err != nil {
			return nil, fmt.Errorf("hcloud: resolve server for volume attachment %d: %w", opts.ServerID, err)
		}
		if server == nil {
			return nil, fmt.Errorf("hcloud: server %d not found for attachment", opts.ServerID)
		}
		createOpts.Server = server
	} else if opts.Location != "" {
		location, _, err := c.hc.Location.GetByName(ctx, opts.Location)
		if err != nil {
			return nil, fmt.Errorf("hcloud: resolve location %q: %w", opts.Location, err)
		}
		if location == nil {
			return nil, fmt.Errorf("hcloud: location %q not found", opts.Location)
		}
		createOpts.Location = location
	} else {
		return nil, fmt.Errorf("hcloud: either ServerID or Location must be provided")
	}

	result, _, err := c.hc.Volume.Create(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("hcloud: CreateVolume %q: %w", opts.Name, err)
	}

	// The Create API returns an Action. Wait for it if we're attaching immediately.
	// But hcloud-go action waiting could block. We'll let the controller loop handle state.
	return toVolumeInfo(result.Volume), nil
}

// DeleteVolume destroys a Hetzner Cloud volume by ID.
func (c *Client) DeleteVolume(ctx context.Context, id int64) error {
	v, _, err := c.hc.Volume.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetVolume(%d): %w", id, err)
	}
	if v == nil {
		return nil
	}
	if _, err = c.hc.Volume.Delete(ctx, v); err != nil {
		return fmt.Errorf("hcloud: DeleteVolume(%d): %w", id, err)
	}
	return nil
}

// AttachVolume attaches an existing volume to a server.
func (c *Client) AttachVolume(ctx context.Context, volumeID int64, serverID int64) error {
	v, _, err := c.hc.Volume.GetByID(ctx, volumeID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch volume %d: %w", volumeID, err)
	}
	if v == nil {
		return fmt.Errorf("hcloud: volume %d not found", volumeID)
	}

	s, _, err := c.hc.Server.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch server %d: %w", serverID, err)
	}
	if s == nil {
		return fmt.Errorf("hcloud: server %d not found", serverID)
	}

	action, _, err := c.hc.Volume.Attach(ctx, v, s)
	if err != nil {
		return fmt.Errorf("hcloud: AttachVolume(%d, %d): %w", volumeID, serverID, err)
	}
	// Note: We don't wait for the action here, the controller will observe state changes
	_ = action
	return nil
}

// DetachVolume detaches an existing volume from any server.
func (c *Client) DetachVolume(ctx context.Context, volumeID int64) error {
	v, _, err := c.hc.Volume.GetByID(ctx, volumeID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch volume %d: %w", volumeID, err)
	}
	if v == nil {
		return fmt.Errorf("hcloud: volume %d not found", volumeID)
	}

	action, _, err := c.hc.Volume.Detach(ctx, v)
	if err != nil {
		return fmt.Errorf("hcloud: DetachVolume(%d): %w", volumeID, err)
	}
	_ = action
	return nil
}

func toVolumeInfo(v *hcloudgo.Volume) *VolumeInfo {
	info := &VolumeInfo{
		ID:          v.ID,
		Name:        v.Name,
		State:       string(v.Status),
		LinuxDevice: v.LinuxDevice,
	}
	if v.Server != nil {
		info.ServerID = v.Server.ID
	}
	return info
}

// GetLoadBalancer fetches a load balancer by its Hetzner Cloud ID.
func (c *Client) GetLoadBalancer(ctx context.Context, id int64) (*LoadBalancerInfo, error) {
	lb, _, err := c.hc.LoadBalancer.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetLoadBalancer(%d): %w", id, err)
	}
	if lb == nil {
		return nil, nil
	}
	return toLoadBalancerInfo(lb), nil
}

// GetLoadBalancerByName fetches a load balancer by its name.
func (c *Client) GetLoadBalancerByName(ctx context.Context, name string) (*LoadBalancerInfo, error) {
	lb, _, err := c.hc.LoadBalancer.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetLoadBalancerByName(%q): %w", name, err)
	}
	if lb == nil {
		return nil, nil
	}
	return toLoadBalancerInfo(lb), nil
}

// CreateLoadBalancer provisions a new Hetzner Cloud load balancer.
func (c *Client) CreateLoadBalancer(ctx context.Context, opts LoadBalancerCreateOpts) (*LoadBalancerInfo, error) {
	lbType, _, err := c.hc.LoadBalancerType.GetByName(ctx, opts.LoadBalancerType)
	if err != nil {
		return nil, fmt.Errorf("hcloud: resolve load balancer type %q: %w", opts.LoadBalancerType, err)
	}
	if lbType == nil {
		return nil, fmt.Errorf("hcloud: load balancer type %q not found", opts.LoadBalancerType)
	}

	createOpts := hcloudgo.LoadBalancerCreateOpts{
		Name:             opts.Name,
		LoadBalancerType: lbType,
		Labels:           opts.Labels,
	}

	if opts.Algorithm != "" {
		createOpts.Algorithm = &hcloudgo.LoadBalancerAlgorithm{Type: hcloudgo.LoadBalancerAlgorithmType(opts.Algorithm)}
	}
	if opts.Location != "" {
		location, _, err := c.hc.Location.GetByName(ctx, opts.Location)
		if err != nil {
			return nil, fmt.Errorf("hcloud: resolve location %q: %w", opts.Location, err)
		}
		if location == nil {
			return nil, fmt.Errorf("hcloud: location %q not found", opts.Location)
		}
		createOpts.Location = location
	}
	if opts.NetworkZone != "" {
		createOpts.NetworkZone = hcloudgo.NetworkZone(opts.NetworkZone)
	}

	result, _, err := c.hc.LoadBalancer.Create(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("hcloud: CreateLoadBalancer %q: %w", opts.Name, err)
	}
	return toLoadBalancerInfo(result.LoadBalancer), nil
}

// DeleteLoadBalancer destroys a Hetzner Cloud load balancer by ID.
func (c *Client) DeleteLoadBalancer(ctx context.Context, id int64) error {
	lb, _, err := c.hc.LoadBalancer.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetLoadBalancer(%d): %w", id, err)
	}
	if lb == nil {
		return nil
	}
	if _, err := c.hc.LoadBalancer.Delete(ctx, lb); err != nil {
		return fmt.Errorf("hcloud: DeleteLoadBalancer(%d): %w", id, err)
	}
	return nil
}

// AttachServerToLoadBalancer attaches an existing server target to a load balancer.
func (c *Client) AttachServerToLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error {
	lb, _, err := c.hc.LoadBalancer.GetByID(ctx, loadBalancerID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch load balancer %d: %w", loadBalancerID, err)
	}
	if lb == nil {
		return fmt.Errorf("hcloud: load balancer %d not found", loadBalancerID)
	}

	server, _, err := c.hc.Server.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch server %d: %w", serverID, err)
	}
	if server == nil {
		return fmt.Errorf("hcloud: server %d not found", serverID)
	}

	action, _, err := c.hc.LoadBalancer.AddServerTarget(ctx, lb, hcloudgo.LoadBalancerAddServerTargetOpts{Server: server})
	if err != nil {
		return fmt.Errorf("hcloud: AttachServerToLoadBalancer(%d, %d): %w", loadBalancerID, serverID, err)
	}
	_ = action
	return nil
}

// DetachServerFromLoadBalancer detaches a server target from a load balancer.
func (c *Client) DetachServerFromLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error {
	lb, _, err := c.hc.LoadBalancer.GetByID(ctx, loadBalancerID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch load balancer %d: %w", loadBalancerID, err)
	}
	if lb == nil {
		return fmt.Errorf("hcloud: load balancer %d not found", loadBalancerID)
	}

	server, _, err := c.hc.Server.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("hcloud: fetch server %d: %w", serverID, err)
	}
	if server == nil {
		return nil
	}

	action, _, err := c.hc.LoadBalancer.RemoveServerTarget(ctx, lb, server)
	if err != nil {
		return fmt.Errorf("hcloud: DetachServerFromLoadBalancer(%d, %d): %w", loadBalancerID, serverID, err)
	}
	_ = action
	return nil
}

// GetNetwork fetches a private network by Hetzner ID.
func (c *Client) GetNetwork(ctx context.Context, id int64) (*NetworkInfo, error) {
	n, _, err := c.hc.Network.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetNetwork(%d): %w", id, err)
	}
	if n == nil {
		return nil, nil
	}
	return toNetworkInfo(n), nil
}

// GetNetworkByName fetches a private network by name.
func (c *Client) GetNetworkByName(ctx context.Context, name string) (*NetworkInfo, error) {
	n, _, err := c.hc.Network.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetNetworkByName(%q): %w", name, err)
	}
	if n == nil {
		return nil, nil
	}
	return toNetworkInfo(n), nil
}

// CreateNetwork creates a private network with the given IPv4 CIDR.
func (c *Client) CreateNetwork(ctx context.Context, opts NetworkCreateOpts) (*NetworkInfo, error) {
	_, ipNet, err := net.ParseCIDR(opts.IPRange)
	if err != nil {
		return nil, fmt.Errorf("hcloud: parse ipRange %q: %w", opts.IPRange, err)
	}
	createOpts := hcloudgo.NetworkCreateOpts{
		Name:                  opts.Name,
		IPRange:               ipNet,
		Labels:                opts.Labels,
		ExposeRoutesToVSwitch: opts.ExposeRoutesToVSwitch,
	}
	n, _, err := c.hc.Network.Create(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("hcloud: CreateNetwork %q: %w", opts.Name, err)
	}
	return toNetworkInfo(n), nil
}

// DeleteNetwork deletes a private network by ID.
func (c *Client) DeleteNetwork(ctx context.Context, id int64) error {
	n, _, err := c.hc.Network.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetNetwork(%d): %w", id, err)
	}
	if n == nil {
		return nil
	}
	if _, err := c.hc.Network.Delete(ctx, n); err != nil {
		return fmt.Errorf("hcloud: DeleteNetwork(%d): %w", id, err)
	}
	return nil
}

// AddNetworkCloudSubnet adds a Cloud subnet in the given Hetzner network zone.
func (c *Client) AddNetworkCloudSubnet(ctx context.Context, networkID int64, zone string) error {
	n, _, err := c.hc.Network.GetByID(ctx, networkID)
	if err != nil {
		return fmt.Errorf("hcloud: GetNetwork(%d): %w", networkID, err)
	}
	if n == nil {
		return fmt.Errorf("hcloud: network %d not found", networkID)
	}
	for _, sn := range n.Subnets {
		if sn.Type == hcloudgo.NetworkSubnetTypeCloud && string(sn.NetworkZone) == zone {
			return nil
		}
	}
	_, _, err = c.hc.Network.AddSubnet(ctx, n, hcloudgo.NetworkAddSubnetOpts{
		Subnet: hcloudgo.NetworkSubnet{
			Type:        hcloudgo.NetworkSubnetTypeCloud,
			NetworkZone: hcloudgo.NetworkZone(zone),
		},
	})
	if err != nil {
		return fmt.Errorf("hcloud: AddNetworkCloudSubnet(%d, %q): %w", networkID, zone, err)
	}
	return nil
}

func toNetworkInfo(n *hcloudgo.Network) *NetworkInfo {
	if n == nil {
		return nil
	}
	info := &NetworkInfo{
		ID:           n.ID,
		Name:         n.Name,
		ExposeRoutes: n.ExposeRoutesToVSwitch,
	}
	if n.IPRange != nil {
		info.IPRange = n.IPRange.String()
	}
	if n.Labels != nil {
		info.Labels = make(map[string]string, len(n.Labels))
		for k, v := range n.Labels {
			info.Labels[k] = v
		}
	}
	for _, sn := range n.Subnets {
		if sn.Type == hcloudgo.NetworkSubnetTypeCloud {
			info.SubnetZones = append(info.SubnetZones, string(sn.NetworkZone))
		}
	}
	return info
}

func toLoadBalancerInfo(lb *hcloudgo.LoadBalancer) *LoadBalancerInfo {
	info := &LoadBalancerInfo{
		ID:   lb.ID,
		Name: lb.Name,
	}
	if lb.PublicNet.IPv4.IP != nil {
		info.PublicIPv4 = lb.PublicNet.IPv4.IP.String()
	}
	if lb.PublicNet.IPv6.IP != nil {
		info.PublicIPv6 = lb.PublicNet.IPv6.IP.String()
	}
	for _, target := range lb.Targets {
		if target.Type == hcloudgo.LoadBalancerTargetTypeServer && target.Server != nil && target.Server.Server != nil {
			info.Targets = append(info.Targets, target.Server.Server.ID)
		}
	}
	return info
}
