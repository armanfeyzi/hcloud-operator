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

// Interface defines the Hetzner Cloud operations required by the controller.
// Using an interface here allows the controller to be tested with a fake client
// without making real API calls.
type Interface interface {
	GetServer(ctx context.Context, id int64) (*ServerInfo, error)
	GetServerByName(ctx context.Context, name string) (*ServerInfo, error)
	CreateServer(ctx context.Context, opts ServerCreateOpts) (*ServerInfo, error)
	DeleteServer(ctx context.Context, id int64) error
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
