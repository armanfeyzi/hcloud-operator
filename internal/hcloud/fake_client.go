package hcloud

import (
	"context"
	"fmt"
	"sync"
)

// FakeClient is an in-memory implementation of Interface for use in tests.
// All operations are goroutine-safe and stored in a simple map keyed by server ID.
type FakeClient struct {
	mu      sync.Mutex
	servers map[int64]*ServerInfo
	volumes map[int64]*VolumeInfo
	nextID  int64

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
		servers: make(map[int64]*ServerInfo),
		volumes: make(map[int64]*VolumeInfo),
		nextID:  1,
	}
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
	cp := *s
	return &cp, nil
}

func (f *FakeClient) GetServerByName(ctx context.Context, name string) (*ServerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.GetErr != nil {
		return nil, f.GetErr
	}
	for _, s := range f.servers {
		if s.Name == name {
			cp := *s
			return &cp, nil
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
		State:      "running",
		PublicIPv4: fmt.Sprintf("1.2.3.%d", id),
		PublicIPv6: fmt.Sprintf("2001:db8::%d/64", id),
	}
	f.servers[id] = info
	cp := *info
	return &cp, nil
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

