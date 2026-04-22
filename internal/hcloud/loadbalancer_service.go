package hcloud

import (
	"context"
	"fmt"
	"time"

	hcloudgo "github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// LoadBalancerHealthCheckHTTPInfo holds HTTP(S) health check settings.
type LoadBalancerHealthCheckHTTPInfo struct {
	Domain      *string
	Path        *string
	Response    *string
	StatusCodes []string
	TLS         *bool
}

// LoadBalancerHealthCheckInfo holds health check settings for a service.
type LoadBalancerHealthCheckInfo struct {
	Protocol string
	Port     *int
	Interval *time.Duration
	Timeout  *time.Duration
	Retries  *int
	HTTP     *LoadBalancerHealthCheckHTTPInfo
}

// LoadBalancerServiceInfo is a portable load balancer service definition.
type LoadBalancerServiceInfo struct {
	Protocol        string
	ListenPort      int
	DestinationPort int
	Proxyprotocol   bool
	HealthCheck     *LoadBalancerHealthCheckInfo
}

// SyncLoadBalancerServices converges Hetzner load balancer services to the desired set (keyed by listen port).
func (c *Client) SyncLoadBalancerServices(ctx context.Context, loadBalancerID int64, desired []LoadBalancerServiceInfo) error {
	lb, _, err := c.hc.LoadBalancer.GetByID(ctx, loadBalancerID)
	if err != nil {
		return fmt.Errorf("hcloud: GetLoadBalancer(%d): %w", loadBalancerID, err)
	}
	if lb == nil {
		return fmt.Errorf("hcloud: load balancer %d not found", loadBalancerID)
	}

	desiredByPort := map[int]LoadBalancerServiceInfo{}
	for _, svc := range desired {
		desiredByPort[svc.ListenPort] = svc
	}

	currentByPort := map[int]LoadBalancerServiceInfo{}
	for _, svc := range lb.Services {
		info := serviceInfoFromSDK(svc)
		currentByPort[info.ListenPort] = info
	}

	for listenPort := range currentByPort {
		if _, ok := desiredByPort[listenPort]; ok {
			continue
		}
		if _, _, err := c.hc.LoadBalancer.DeleteService(ctx, lb, listenPort); err != nil {
			return fmt.Errorf("hcloud: DeleteService(%d, %d): %w", loadBalancerID, listenPort, err)
		}
	}

	for listenPort, want := range desiredByPort {
		have, exists := currentByPort[listenPort]
		if !exists {
			if _, _, err := c.hc.LoadBalancer.AddService(ctx, lb, addServiceOptsFromInfo(want)); err != nil {
				return fmt.Errorf("hcloud: AddService(%d, %d): %w", loadBalancerID, listenPort, err)
			}
			continue
		}
		if servicesEqual(have, want) {
			continue
		}
		if _, _, err := c.hc.LoadBalancer.UpdateService(ctx, lb, listenPort, updateServiceOptsFromInfo(want)); err != nil {
			return fmt.Errorf("hcloud: UpdateService(%d, %d): %w", loadBalancerID, listenPort, err)
		}
	}

	return nil
}

func serviceInfoFromSDK(svc hcloudgo.LoadBalancerService) LoadBalancerServiceInfo {
	info := LoadBalancerServiceInfo{
		Protocol:        string(svc.Protocol),
		ListenPort:      svc.ListenPort,
		DestinationPort: svc.DestinationPort,
		Proxyprotocol:   svc.Proxyprotocol,
	}
	if svc.HealthCheck.Protocol != "" {
		hc := healthCheckInfoFromSDK(svc.HealthCheck)
		info.HealthCheck = &hc
	}
	return info
}

func healthCheckInfoFromSDK(hc hcloudgo.LoadBalancerServiceHealthCheck) LoadBalancerHealthCheckInfo {
	info := LoadBalancerHealthCheckInfo{
		Protocol: string(hc.Protocol),
	}
	if hc.Port > 0 {
		port := hc.Port
		info.Port = &port
	}
	if hc.Interval > 0 {
		interval := hc.Interval
		info.Interval = &interval
	}
	if hc.Timeout > 0 {
		timeout := hc.Timeout
		info.Timeout = &timeout
	}
	if hc.Retries > 0 {
		retries := hc.Retries
		info.Retries = &retries
	}
	if hc.HTTP != nil {
		httpInfo := LoadBalancerHealthCheckHTTPInfo{
			StatusCodes: append([]string{}, hc.HTTP.StatusCodes...),
		}
		if hc.HTTP.Domain != "" {
			domain := hc.HTTP.Domain
			httpInfo.Domain = &domain
		}
		if hc.HTTP.Path != "" {
			path := hc.HTTP.Path
			httpInfo.Path = &path
		}
		if hc.HTTP.Response != "" {
			response := hc.HTTP.Response
			httpInfo.Response = &response
		}
		if hc.HTTP.TLS {
			tls := hc.HTTP.TLS
			httpInfo.TLS = &tls
		}
		info.HTTP = &httpInfo
	}
	return info
}

func addServiceOptsFromInfo(svc LoadBalancerServiceInfo) hcloudgo.LoadBalancerAddServiceOpts {
	opts := hcloudgo.LoadBalancerAddServiceOpts{
		Protocol:        hcloudgo.LoadBalancerServiceProtocol(svc.Protocol),
		ListenPort:      intPtr(svc.ListenPort),
		DestinationPort: intPtr(svc.DestinationPort),
	}
	if svc.Proxyprotocol {
		opts.Proxyprotocol = boolPtr(true)
	}
	if svc.HealthCheck != nil {
		hc := addHealthCheckOptsFromInfo(*svc.HealthCheck)
		opts.HealthCheck = &hc
	}
	return opts
}

func updateServiceOptsFromInfo(svc LoadBalancerServiceInfo) hcloudgo.LoadBalancerUpdateServiceOpts {
	opts := hcloudgo.LoadBalancerUpdateServiceOpts{
		Protocol:        hcloudgo.LoadBalancerServiceProtocol(svc.Protocol),
		DestinationPort: intPtr(svc.DestinationPort),
	}
	if svc.Proxyprotocol {
		opts.Proxyprotocol = boolPtr(true)
	} else {
		opts.Proxyprotocol = boolPtr(false)
	}
	if svc.HealthCheck != nil {
		hc := updateHealthCheckOptsFromInfo(*svc.HealthCheck)
		opts.HealthCheck = &hc
	}
	return opts
}

func addHealthCheckOptsFromInfo(hc LoadBalancerHealthCheckInfo) hcloudgo.LoadBalancerAddServiceOptsHealthCheck {
	opts := hcloudgo.LoadBalancerAddServiceOptsHealthCheck{
		Protocol: hcloudgo.LoadBalancerServiceProtocol(hc.Protocol),
		Port:     hc.Port,
		Retries:  hc.Retries,
	}
	if hc.Interval != nil {
		opts.Interval = hc.Interval
	}
	if hc.Timeout != nil {
		opts.Timeout = hc.Timeout
	}
	if hc.HTTP != nil {
		opts.HTTP = addHealthCheckHTTPOptsFromInfo(*hc.HTTP)
	}
	return opts
}

func updateHealthCheckOptsFromInfo(hc LoadBalancerHealthCheckInfo) hcloudgo.LoadBalancerUpdateServiceOptsHealthCheck {
	opts := hcloudgo.LoadBalancerUpdateServiceOptsHealthCheck{
		Protocol: hcloudgo.LoadBalancerServiceProtocol(hc.Protocol),
		Port:     hc.Port,
		Retries:  hc.Retries,
	}
	if hc.Interval != nil {
		opts.Interval = hc.Interval
	}
	if hc.Timeout != nil {
		opts.Timeout = hc.Timeout
	}
	if hc.HTTP != nil {
		opts.HTTP = updateHealthCheckHTTPOptsFromInfo(*hc.HTTP)
	}
	return opts
}

func addHealthCheckHTTPOptsFromInfo(http LoadBalancerHealthCheckHTTPInfo) *hcloudgo.LoadBalancerAddServiceOptsHealthCheckHTTP {
	return &hcloudgo.LoadBalancerAddServiceOptsHealthCheckHTTP{
		Domain:      http.Domain,
		Path:        http.Path,
		Response:    http.Response,
		StatusCodes: append([]string{}, http.StatusCodes...),
		TLS:         http.TLS,
	}
}

func updateHealthCheckHTTPOptsFromInfo(http LoadBalancerHealthCheckHTTPInfo) *hcloudgo.LoadBalancerUpdateServiceOptsHealthCheckHTTP {
	return &hcloudgo.LoadBalancerUpdateServiceOptsHealthCheckHTTP{
		Domain:      http.Domain,
		Path:        http.Path,
		Response:    http.Response,
		StatusCodes: append([]string{}, http.StatusCodes...),
		TLS:         http.TLS,
	}
}

func servicesEqual(a, b LoadBalancerServiceInfo) bool {
	if a.Protocol != b.Protocol ||
		a.ListenPort != b.ListenPort ||
		a.DestinationPort != b.DestinationPort ||
		a.Proxyprotocol != b.Proxyprotocol {
		return false
	}
	if b.HealthCheck == nil {
		return true
	}
	return healthChecksEqual(a.HealthCheck, b.HealthCheck)
}

func healthChecksEqual(a, b *LoadBalancerHealthCheckInfo) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Protocol != b.Protocol ||
		!intPtrEqual(a.Port, b.Port) ||
		!durationPtrEqual(a.Interval, b.Interval) ||
		!durationPtrEqual(a.Timeout, b.Timeout) ||
		!intPtrEqual(a.Retries, b.Retries) {
		return false
	}
	return healthCheckHTTPEqual(a.HTTP, b.HTTP)
}

func healthCheckHTTPEqual(a, b *LoadBalancerHealthCheckHTTPInfo) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !stringPtrEqual(a.Domain, b.Domain) ||
		!stringPtrEqual(a.Path, b.Path) ||
		!stringPtrEqual(a.Response, b.Response) ||
		!boolPtrEqual(a.TLS, b.TLS) {
		return false
	}
	if len(a.StatusCodes) != len(b.StatusCodes) {
		return false
	}
	for i := range a.StatusCodes {
		if a.StatusCodes[i] != b.StatusCodes[i] {
			return false
		}
	}
	return true
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func durationPtrEqual(a, b *time.Duration) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
