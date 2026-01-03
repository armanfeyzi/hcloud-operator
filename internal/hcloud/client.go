package hcloud

import (
	"context"
	"fmt"

	hcloudgo "github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// ServerInfo is a minimal, SDK-agnostic view of a Hetzner Cloud server.
// The controller uses this type so it doesn't import the hcloud-go SDK directly.
type ServerInfo struct {
	ID         int64
	Name       string
	State      string
	PublicIPv4 string
	PublicIPv6 string
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
	ID         int64
	Name       string
	State      string
	ServerID   int64  // 0 if unattached
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

// Interface defines the Hetzner Cloud operations required by the controller.
// Using an interface here allows the controller to be tested with a fake client
// without making real API calls.
type Interface interface {
	GetServer(ctx context.Context, id int64) (*ServerInfo, error)
	GetServerByName(ctx context.Context, name string) (*ServerInfo, error)
	CreateServer(ctx context.Context, opts ServerCreateOpts) (*ServerInfo, error)
	DeleteServer(ctx context.Context, id int64) error

	GetVolume(ctx context.Context, id int64) (*VolumeInfo, error)
	GetVolumeByName(ctx context.Context, name string) (*VolumeInfo, error)
	CreateVolume(ctx context.Context, opts VolumeCreateOpts) (*VolumeInfo, error)
	DeleteVolume(ctx context.Context, id int64) error
	AttachVolume(ctx context.Context, volumeID int64, serverID int64) error
	DetachVolume(ctx context.Context, volumeID int64) error
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

// toServerInfo converts a raw hcloud-go Server into our SDK-agnostic ServerInfo.
func toServerInfo(s *hcloudgo.Server) *ServerInfo {
	info := &ServerInfo{
		ID:    s.ID,
		Name:  s.Name,
		State: string(s.Status),
	}
	if s.PublicNet.IPv4.IP != nil {
		info.PublicIPv4 = s.PublicNet.IPv4.IP.String()
	}
	if s.PublicNet.IPv6.Network != nil {
		info.PublicIPv6 = s.PublicNet.IPv6.Network.String()
	}
	return info
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
