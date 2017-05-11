// Copyright (c) 2017 Pani Networks
// All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package provides tools for romana CNI plugin to interact with other
// Romana services.
package cni

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/romana/core/common"
	log "github.com/romana/rlog"
	"github.com/vishvananda/netlink"
	"net"
)

// RomanaAddresManager describes functions that allow allocating and deallocating
// IP addresses from Romana.
type RomanaAddresManager interface {
	Allocate(NetConf, RomanaAllocatorPodDescription) (*net.IPNet, error)
	Deallocate(NetConf, string) error
}

// NewRomanaAddressManager returns structure that satisfies RomanaAddresManager,
// it allows multiple implementations.
func NewRomanaAddressManager(provider RomanaAddressManagerProvider) (RomanaAddresManager, error) {
	if provider == DefaultProvider {
		return DefaultAddressManager{}, nil
	}

	return nil, fmt.Errorf("Unknown provider type %s", provider)
}

type RomanaAddressManagerProvider string

// DefaultProvider allocates and deallocates IP addresses using rest requests
// to Romana IPAM.
const DefaultProvider RomanaAddressManagerProvider = "default"

// RomanaAllocatorPodDescription represents collection of parameters used to allocate IP address.
type RomanaAllocatorPodDescription struct {
	Name        string
	Hostname    string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
}

// NetConf represents parameters CNI plugin receives via stdin.
type NetConf struct {
	FeatureIP6TW
	types.NetConf
	KubernetesConfig string `json:"kubernetes_config"`
	RomanaRoot       string `json:"romana_root"`

	// Name of a current host in romana.
	// If omitted, current hostname will be used.
	RomanaHostName   string `json:"romana_host_name"`
	SegmentLabelName string `json:"segment_label_name"`
	TenantLabelName  string `json:"tenant_label_name"` // TODO for stas, we don't use it. May be it should go away.
	UseAnnotations   bool   `json:"use_annotattions"`
}

// The structure should be in separate file.
type FeatureIP6TW struct {
	// Fields relative to the feature.
}

type DefaultAddressManager struct{}

func (DefaultAddressManager) Allocate(config NetConf, pod RomanaAllocatorPodDescription) (*net.IPNet, error) {
	// Discover pod segment.
	var segmentLabel string
	var ok bool
	if config.UseAnnotations {
		segmentLabel, ok = pod.Annotations[config.SegmentLabelName]
	} else {
		segmentLabel, ok = pod.Labels[config.SegmentLabelName]
	}
	if !ok {
		return nil, fmt.Errorf("Failed to discover segment label for a pod")
	}
	log.Infof("Discovered segment %s for a pod", segmentLabel)

	// Rest client config
	clientConfig := common.GetDefaultRestClientConfig(config.RomanaRoot, nil)
	client, err := common.NewRestClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to reach romana root at %s, err=(%s)", config.RomanaRoot, err)
	}
	log.Infof("Created romana client %v", client)

	// Topology, find host id.
	hosts, err := client.ListHosts()
	if err != nil {
		return nil, fmt.Errorf("Failed to list romana hosts err=(%s)", err)
	}
	var currentHost common.Host
	for hostNum, host := range hosts {
		if host.Name == config.RomanaHostName {
			currentHost = hosts[hostNum]
			break
		}
	}
	if currentHost.Name == "" {
		return nil, fmt.Errorf("Failed to find romana host with name %s in romana database", config.RomanaHostName)
	}

	// Tenant and segemnt
	tenantUrl, err := client.GetServiceUrl("tenant")
	if err != nil {
		return nil, fmt.Errorf("Failed to discover tenant url from romana root err=(%s)", err)
	}
	tenantUrl += "/tenants"
	var tenants []common.Tenant
	err = client.Get(tenantUrl, &tenants)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch romana tenants from %s, err=(%s)", tenantUrl, err)
	}
	var currentTenant common.Tenant
	for tenantNum, tenant := range tenants {
		if tenant.Name == pod.Namespace {
			currentTenant = tenants[tenantNum]
			break
		}
	}
	if currentTenant.Name == "" {
		return nil, fmt.Errorf("Failed to find romana tenant with name %s, err=(%s)", pod.Namespace, err)
	}
	var segments []common.Segment
	segmentsUrl := fmt.Sprintf("%s/%d/segments", tenantUrl, currentTenant.ID)
	err = client.Get(segmentsUrl, &segments)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch segments from %s, err=(%s)", segmentsUrl, err)
	}
	var currentSegment common.Segment
	for segmentNum, segment := range segments {
		if segment.Name == segmentLabel {
			currentSegment = segments[segmentNum]
			break
		}
	}
	if currentSegment.Name == "" {
		return nil, fmt.Errorf("Failed to discover segment %s within tenant %s", pod.Namespace, segmentLabel)
	}
	log.Infof("Discovered tenant=%d, segment=%d, host=%d", currentTenant.ID, currentSegment.ID, currentHost.ID)

	// IPAM allocate
	ipamUrl, err := client.GetServiceUrl("ipam")
	if err != nil {
		return nil, fmt.Errorf("Failed to discover ipam url from romana root err=(%s)", err)
	}
	ipamUrl += "/endpoints"
	var ipamReq, ipamResp common.IPAMEndpoint
	ipamReq = common.IPAMEndpoint{
		TenantID:  fmt.Sprintf("%d", currentTenant.ID),
		SegmentID: fmt.Sprintf("%d", currentSegment.ID),
		HostId:    fmt.Sprintf("%d", currentHost.ID),
		Name:      pod.Name,
	}
	err = client.Post(ipamUrl, &ipamReq, &ipamResp)
	if err != nil {
		return nil, fmt.Errorf("Failed to allocate IP address for a pod %s.%s, err=(%s)", pod.Namespace, currentTenant.Name, err)
	}
	log.Infof("Allocated IP address %s", ipamResp.Ip)
	ipamIP, err := netlink.ParseIPNet(ipamResp.Ip + "/32")
	if err != nil {
		return nil, fmt.Errorf("Failed to parse IP address %s, err=(%s)", ipamResp.Ip, err)
	}

	return ipamIP, nil
}

func (DefaultAddressManager) Deallocate(config NetConf, targetName string) error {
	// Rest client config
	clientConfig := common.GetDefaultRestClientConfig(config.RomanaRoot, nil)
	client, err := common.NewRestClient(clientConfig)
	if err != nil {
		return fmt.Errorf("Failed to reach romana root at %s, err=(%s)", config.RomanaRoot, err)
	}
	log.Infof("Created romana client %v", client)

	ipamUrl, err := client.GetServiceUrl("ipam")
	if err != nil {
		return fmt.Errorf("Failed to discover ipam url from romana root err=(%s)", err)
	}
	ipamUrl += "/endpoints"

	var endpoints, podEndpoints []common.IPAMEndpoint
	err = client.Get(ipamUrl, &endpoints)
	if err != nil {
		return fmt.Errorf("Failed to fetch ipam endpoints, err=(%s)", err)
	}

	for eNum, endpoint := range endpoints {
		if endpoint.Name == targetName && endpoint.InUse {
			podEndpoints = append(podEndpoints, endpoints[eNum])
		}
	}

	if len(podEndpoints) == 0 {
		return fmt.Errorf("No IPAM endpoints found for pod %s", targetName)
	}

	if len(podEndpoints) > 1 {
		return fmt.Errorf("Multiple IPAM endpoints found for pod %s, not supported", targetName)
	}

	endpointDeleteUrl := fmt.Sprintf("%s/%s", ipamUrl, podEndpoints[0].Ip)
	err = client.Delete(endpointDeleteUrl, nil, nil)
	if err != nil {
		return fmt.Errorf("Failed to delete IPAM endpoint %v", podEndpoints[0])
	}

	return nil
}
