package hcloud

import (
	"context"
	"time"

	"github.com/armanfeyzi/hcloud-operator/internal/metrics"
)

// instrumentedClient wraps an Interface and records Prometheus metrics for each API call.
type instrumentedClient struct {
	inner Interface
}

// Instrument returns c wrapped with API call metrics. Passing nil returns nil.
// Wrapping an already-instrumented client is a no-op.
func Instrument(c Interface) Interface {
	if c == nil {
		return nil
	}
	if _, ok := c.(*instrumentedClient); ok {
		return c
	}
	return &instrumentedClient{inner: c}
}

func recordAPI(op string, err error, start time.Time) {
	result := "success"
	if err != nil {
		result = "error"
	}
	metrics.RecordAPI(op, result, time.Since(start))
}

func (c *instrumentedClient) GetServer(ctx context.Context, id int64) (*ServerInfo, error) {
	start := time.Now()
	v, err := c.inner.GetServer(ctx, id)
	recordAPI("GetServer", err, start)
	return v, err
}

func (c *instrumentedClient) GetServerByName(ctx context.Context, name string) (*ServerInfo, error) {
	start := time.Now()
	v, err := c.inner.GetServerByName(ctx, name)
	recordAPI("GetServerByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateServer(ctx context.Context, opts ServerCreateOpts) (*ServerInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateServer(ctx, opts)
	recordAPI("CreateServer", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteServer(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteServer(ctx, id)
	recordAPI("DeleteServer", err, start)
	return err
}

func (c *instrumentedClient) PowerOffServer(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.PowerOffServer(ctx, id)
	recordAPI("PowerOffServer", err, start)
	return err
}

func (c *instrumentedClient) PowerOnServer(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.PowerOnServer(ctx, id)
	recordAPI("PowerOnServer", err, start)
	return err
}

func (c *instrumentedClient) ChangeServerType(ctx context.Context, id int64, serverType string, upgradeDisk bool) error {
	start := time.Now()
	err := c.inner.ChangeServerType(ctx, id, serverType, upgradeDisk)
	recordAPI("ChangeServerType", err, start)
	return err
}

func (c *instrumentedClient) AttachServerToNetwork(ctx context.Context, serverID int64, networkID int64) error {
	start := time.Now()
	err := c.inner.AttachServerToNetwork(ctx, serverID, networkID)
	recordAPI("AttachServerToNetwork", err, start)
	return err
}

func (c *instrumentedClient) DetachServerFromNetwork(ctx context.Context, serverID int64, networkID int64) error {
	start := time.Now()
	err := c.inner.DetachServerFromNetwork(ctx, serverID, networkID)
	recordAPI("DetachServerFromNetwork", err, start)
	return err
}

func (c *instrumentedClient) GetVolume(ctx context.Context, id int64) (*VolumeInfo, error) {
	start := time.Now()
	v, err := c.inner.GetVolume(ctx, id)
	recordAPI("GetVolume", err, start)
	return v, err
}

func (c *instrumentedClient) GetVolumeByName(ctx context.Context, name string) (*VolumeInfo, error) {
	start := time.Now()
	v, err := c.inner.GetVolumeByName(ctx, name)
	recordAPI("GetVolumeByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateVolume(ctx context.Context, opts VolumeCreateOpts) (*VolumeInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateVolume(ctx, opts)
	recordAPI("CreateVolume", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteVolume(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteVolume(ctx, id)
	recordAPI("DeleteVolume", err, start)
	return err
}

func (c *instrumentedClient) AttachVolume(ctx context.Context, volumeID int64, serverID int64) error {
	start := time.Now()
	err := c.inner.AttachVolume(ctx, volumeID, serverID)
	recordAPI("AttachVolume", err, start)
	return err
}

func (c *instrumentedClient) DetachVolume(ctx context.Context, volumeID int64) error {
	start := time.Now()
	err := c.inner.DetachVolume(ctx, volumeID)
	recordAPI("DetachVolume", err, start)
	return err
}

func (c *instrumentedClient) ResizeVolume(ctx context.Context, volumeID int64, sizeGB int) error {
	start := time.Now()
	err := c.inner.ResizeVolume(ctx, volumeID, sizeGB)
	recordAPI("ResizeVolume", err, start)
	return err
}

func (c *instrumentedClient) GetLoadBalancer(ctx context.Context, id int64) (*LoadBalancerInfo, error) {
	start := time.Now()
	v, err := c.inner.GetLoadBalancer(ctx, id)
	recordAPI("GetLoadBalancer", err, start)
	return v, err
}

func (c *instrumentedClient) GetLoadBalancerByName(ctx context.Context, name string) (*LoadBalancerInfo, error) {
	start := time.Now()
	v, err := c.inner.GetLoadBalancerByName(ctx, name)
	recordAPI("GetLoadBalancerByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateLoadBalancer(ctx context.Context, opts LoadBalancerCreateOpts) (*LoadBalancerInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateLoadBalancer(ctx, opts)
	recordAPI("CreateLoadBalancer", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteLoadBalancer(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteLoadBalancer(ctx, id)
	recordAPI("DeleteLoadBalancer", err, start)
	return err
}

func (c *instrumentedClient) AttachServerToLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error {
	start := time.Now()
	err := c.inner.AttachServerToLoadBalancer(ctx, loadBalancerID, serverID)
	recordAPI("AttachServerToLoadBalancer", err, start)
	return err
}

func (c *instrumentedClient) DetachServerFromLoadBalancer(ctx context.Context, loadBalancerID int64, serverID int64) error {
	start := time.Now()
	err := c.inner.DetachServerFromLoadBalancer(ctx, loadBalancerID, serverID)
	recordAPI("DetachServerFromLoadBalancer", err, start)
	return err
}

func (c *instrumentedClient) SyncLoadBalancerServices(ctx context.Context, loadBalancerID int64, services []LoadBalancerServiceInfo) error {
	start := time.Now()
	err := c.inner.SyncLoadBalancerServices(ctx, loadBalancerID, services)
	recordAPI("SyncLoadBalancerServices", err, start)
	return err
}

func (c *instrumentedClient) GetNetwork(ctx context.Context, id int64) (*NetworkInfo, error) {
	start := time.Now()
	v, err := c.inner.GetNetwork(ctx, id)
	recordAPI("GetNetwork", err, start)
	return v, err
}

func (c *instrumentedClient) GetNetworkByName(ctx context.Context, name string) (*NetworkInfo, error) {
	start := time.Now()
	v, err := c.inner.GetNetworkByName(ctx, name)
	recordAPI("GetNetworkByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateNetwork(ctx context.Context, opts NetworkCreateOpts) (*NetworkInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateNetwork(ctx, opts)
	recordAPI("CreateNetwork", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteNetwork(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteNetwork(ctx, id)
	recordAPI("DeleteNetwork", err, start)
	return err
}

func (c *instrumentedClient) AddNetworkCloudSubnet(ctx context.Context, networkID int64, zone string) error {
	start := time.Now()
	err := c.inner.AddNetworkCloudSubnet(ctx, networkID, zone)
	recordAPI("AddNetworkCloudSubnet", err, start)
	return err
}

func (c *instrumentedClient) GetFirewall(ctx context.Context, id int64) (*FirewallInfo, error) {
	start := time.Now()
	v, err := c.inner.GetFirewall(ctx, id)
	recordAPI("GetFirewall", err, start)
	return v, err
}

func (c *instrumentedClient) GetFirewallByName(ctx context.Context, name string) (*FirewallInfo, error) {
	start := time.Now()
	v, err := c.inner.GetFirewallByName(ctx, name)
	recordAPI("GetFirewallByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateFirewall(ctx context.Context, opts FirewallCreateOpts) (*FirewallInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateFirewall(ctx, opts)
	recordAPI("CreateFirewall", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteFirewall(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteFirewall(ctx, id)
	recordAPI("DeleteFirewall", err, start)
	return err
}

func (c *instrumentedClient) UpdateFirewallLabels(ctx context.Context, id int64, labels map[string]string) error {
	start := time.Now()
	err := c.inner.UpdateFirewallLabels(ctx, id, labels)
	recordAPI("UpdateFirewallLabels", err, start)
	return err
}

func (c *instrumentedClient) SetFirewallRules(ctx context.Context, id int64, rules []FirewallRuleInfo) error {
	start := time.Now()
	err := c.inner.SetFirewallRules(ctx, id, rules)
	recordAPI("SetFirewallRules", err, start)
	return err
}

func (c *instrumentedClient) ApplyFirewallResources(ctx context.Context, id int64, resources []FirewallApplyResource) error {
	start := time.Now()
	err := c.inner.ApplyFirewallResources(ctx, id, resources)
	recordAPI("ApplyFirewallResources", err, start)
	return err
}

func (c *instrumentedClient) RemoveFirewallResources(ctx context.Context, id int64, resources []FirewallApplyResource) error {
	start := time.Now()
	err := c.inner.RemoveFirewallResources(ctx, id, resources)
	recordAPI("RemoveFirewallResources", err, start)
	return err
}

func (c *instrumentedClient) GetPlacementGroup(ctx context.Context, id int64) (*PlacementGroupInfo, error) {
	start := time.Now()
	v, err := c.inner.GetPlacementGroup(ctx, id)
	recordAPI("GetPlacementGroup", err, start)
	return v, err
}

func (c *instrumentedClient) GetPlacementGroupByName(ctx context.Context, name string) (*PlacementGroupInfo, error) {
	start := time.Now()
	v, err := c.inner.GetPlacementGroupByName(ctx, name)
	recordAPI("GetPlacementGroupByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreatePlacementGroup(ctx context.Context, opts PlacementGroupCreateOpts) (*PlacementGroupInfo, error) {
	start := time.Now()
	v, err := c.inner.CreatePlacementGroup(ctx, opts)
	recordAPI("CreatePlacementGroup", err, start)
	return v, err
}

func (c *instrumentedClient) DeletePlacementGroup(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeletePlacementGroup(ctx, id)
	recordAPI("DeletePlacementGroup", err, start)
	return err
}

func (c *instrumentedClient) GetPrimaryIP(ctx context.Context, id int64) (*PrimaryIPInfo, error) {
	start := time.Now()
	v, err := c.inner.GetPrimaryIP(ctx, id)
	recordAPI("GetPrimaryIP", err, start)
	return v, err
}

func (c *instrumentedClient) GetPrimaryIPByName(ctx context.Context, name string) (*PrimaryIPInfo, error) {
	start := time.Now()
	v, err := c.inner.GetPrimaryIPByName(ctx, name)
	recordAPI("GetPrimaryIPByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreatePrimaryIP(ctx context.Context, opts PrimaryIPCreateOpts) (*PrimaryIPInfo, error) {
	start := time.Now()
	v, err := c.inner.CreatePrimaryIP(ctx, opts)
	recordAPI("CreatePrimaryIP", err, start)
	return v, err
}

func (c *instrumentedClient) DeletePrimaryIP(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeletePrimaryIP(ctx, id)
	recordAPI("DeletePrimaryIP", err, start)
	return err
}

func (c *instrumentedClient) UpdatePrimaryIP(ctx context.Context, id int64, opts PrimaryIPUpdateOpts) error {
	start := time.Now()
	err := c.inner.UpdatePrimaryIP(ctx, id, opts)
	recordAPI("UpdatePrimaryIP", err, start)
	return err
}

func (c *instrumentedClient) AssignPrimaryIP(ctx context.Context, id int64, assigneeID int64, assigneeType string) error {
	start := time.Now()
	err := c.inner.AssignPrimaryIP(ctx, id, assigneeID, assigneeType)
	recordAPI("AssignPrimaryIP", err, start)
	return err
}

func (c *instrumentedClient) UnassignPrimaryIP(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.UnassignPrimaryIP(ctx, id)
	recordAPI("UnassignPrimaryIP", err, start)
	return err
}

func (c *instrumentedClient) ChangePrimaryIPDNSPtr(ctx context.Context, id int64, ip string, dnsPtr string) error {
	start := time.Now()
	err := c.inner.ChangePrimaryIPDNSPtr(ctx, id, ip, dnsPtr)
	recordAPI("ChangePrimaryIPDNSPtr", err, start)
	return err
}

func (c *instrumentedClient) GetFloatingIP(ctx context.Context, id int64) (*FloatingIPInfo, error) {
	start := time.Now()
	v, err := c.inner.GetFloatingIP(ctx, id)
	recordAPI("GetFloatingIP", err, start)
	return v, err
}

func (c *instrumentedClient) GetFloatingIPByName(ctx context.Context, name string) (*FloatingIPInfo, error) {
	start := time.Now()
	v, err := c.inner.GetFloatingIPByName(ctx, name)
	recordAPI("GetFloatingIPByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateFloatingIP(ctx context.Context, opts FloatingIPCreateOpts) (*FloatingIPInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateFloatingIP(ctx, opts)
	recordAPI("CreateFloatingIP", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteFloatingIP(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteFloatingIP(ctx, id)
	recordAPI("DeleteFloatingIP", err, start)
	return err
}

func (c *instrumentedClient) UpdateFloatingIP(ctx context.Context, id int64, opts FloatingIPUpdateOpts) error {
	start := time.Now()
	err := c.inner.UpdateFloatingIP(ctx, id, opts)
	recordAPI("UpdateFloatingIP", err, start)
	return err
}

func (c *instrumentedClient) AssignFloatingIP(ctx context.Context, id int64, serverID int64) error {
	start := time.Now()
	err := c.inner.AssignFloatingIP(ctx, id, serverID)
	recordAPI("AssignFloatingIP", err, start)
	return err
}

func (c *instrumentedClient) UnassignFloatingIP(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.UnassignFloatingIP(ctx, id)
	recordAPI("UnassignFloatingIP", err, start)
	return err
}

func (c *instrumentedClient) ChangeFloatingIPDNSPtr(ctx context.Context, id int64, ip string, dnsPtr string) error {
	start := time.Now()
	err := c.inner.ChangeFloatingIPDNSPtr(ctx, id, ip, dnsPtr)
	recordAPI("ChangeFloatingIPDNSPtr", err, start)
	return err
}

func (c *instrumentedClient) GetCertificate(ctx context.Context, id int64) (*CertificateInfo, error) {
	start := time.Now()
	v, err := c.inner.GetCertificate(ctx, id)
	recordAPI("GetCertificate", err, start)
	return v, err
}

func (c *instrumentedClient) GetCertificateByName(ctx context.Context, name string) (*CertificateInfo, error) {
	start := time.Now()
	v, err := c.inner.GetCertificateByName(ctx, name)
	recordAPI("GetCertificateByName", err, start)
	return v, err
}

func (c *instrumentedClient) CreateCertificate(ctx context.Context, opts CertificateCreateOpts) (*CertificateInfo, error) {
	start := time.Now()
	v, err := c.inner.CreateCertificate(ctx, opts)
	recordAPI("CreateCertificate", err, start)
	return v, err
}

func (c *instrumentedClient) DeleteCertificate(ctx context.Context, id int64) error {
	start := time.Now()
	err := c.inner.DeleteCertificate(ctx, id)
	recordAPI("DeleteCertificate", err, start)
	return err
}

func (c *instrumentedClient) UpdateCertificate(ctx context.Context, id int64, opts CertificateUpdateOpts) error {
	start := time.Now()
	err := c.inner.UpdateCertificate(ctx, id, opts)
	recordAPI("UpdateCertificate", err, start)
	return err
}
