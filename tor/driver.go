package tor

import (
	"fmt"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/portmapper"
	"github.com/docker/libnetwork/types"
	"github.com/jfrazelle/onion/pkg/dknet"
	"github.com/samalba/dockerclient"
	"github.com/vishvananda/netlink"
)

const (
	defaultRoute     = "0.0.0.0/0"
	torPortPrefix    = "tor-veth0-"
	bridgePrefix     = "torbr-"
	containerEthName = "eth"

	mtuOption        = "net.jfrazelle.tor.bridge.mtu"
	bridgeNameOption = "net.jfrazelle.tor.bridge.name"

	defaultMTU = 1500
)

type Driver struct {
	dknet.Driver
	dockerer
	networks map[string]*NetworkState
	sync.Mutex
}

// endpointConfiguration represents the user specified configuration for the sandbox endpoint
type endpointConfiguration struct {
	PortBindings []types.PortBinding
	ExposedPorts []types.TransportPort
}

// containerConfiguration represents the user specified configuration for a container
type containerConfiguration struct {
	ParentEndpoints []string
	ChildEndpoints  []string
}

type torEndpoint struct {
	id              string
	srcName         string
	addr            *net.IPNet
	addrv6          *net.IPNet
	macAddress      net.HardwareAddr
	config          *endpointConfiguration // User specified parameters
	containerConfig *containerConfiguration
	portMapping     []types.PortBinding // Operation port bindings
}

// NetworkState is filled in at network creation time
// it contains state that we wish to keep for each network
type NetworkState struct {
	BridgeName  string
	MTU         int
	Gateway     string
	GatewayMask string
	endpoints   map[string]*torEndpoint // key: endpoint id
	portMapper  *portmapper.PortMapper
	sync.Mutex
}

func (d *Driver) CreateNetwork(r *dknet.CreateNetworkRequest) error {
	logrus.Debugf("Create network request: %+v", r)

	bridgeName, err := getBridgeName(r.NetworkID, r.Options)
	if err != nil {
		return err
	}

	mtu, err := getBridgeMTU(r.Options)
	if err != nil {
		return err
	}

	gateway, mask, err := getGatewayIP(r)
	if err != nil {
		return err
	}

	ns := &NetworkState{
		BridgeName:  bridgeName,
		MTU:         mtu,
		Gateway:     gateway,
		GatewayMask: mask,
	}
	d.networks[r.NetworkID] = ns

	logrus.Debugf("Initializing bridge for network %s", r.NetworkID)
	if err := d.initBridge(r.NetworkID); err != nil {
		logrus.Errorf("Init bridge %s failed: %s", bridgeName, err)
		delete(d.networks, r.NetworkID)
		return err
	}
	return nil
}

func (d *Driver) DeleteNetwork(r *dknet.DeleteNetworkRequest) error {
	logrus.Debugf("Delete network request: %+v", r)
	bridgeName := d.networks[r.NetworkID].BridgeName
	logrus.Debugf("Deleting Bridge %s", bridgeName)
	err := d.deleteBridge(bridgeName)
	if err != nil {
		logrus.Errorf("Deleting bridge %s failed: %s", bridgeName, err)
		return err
	}
	delete(d.networks, r.NetworkID)
	return nil
}

func (d *Driver) CreateEndpoint(r *dknet.CreateEndpointRequest) error {
	logrus.Debugf("Create endpoint request: %+v", r)

	// Get the network handler and make sure it exists
	d.Lock()
	ns, ok := d.networks[r.NetworkID]
	d.Unlock()
	if !ok {
		return types.InternalMaskableErrorf("network %s does not exist", r.NetworkID)
	}

	if ns == nil {
		return driverapi.ErrNoNetwork(r.NetworkID)
	}

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := ns.getEndpoint(r.EndpointID)
	if err != nil {
		return err
	}

	// Endpoint with that id exists either on desired or other sandbox
	if ep != nil {
		return driverapi.ErrEndpointExists(r.EndpointID)
	}

	// Try to convert the options to endpoint configuration
	epConfig, err := parseEndpointOptions(r.Options)
	if err != nil {
		return err
	}

	// Create and add the endpoint
	ns.Lock()
	endpoint := &torEndpoint{id: r.EndpointID, config: epConfig}
	ns.endpoints[r.EndpointID] = endpoint
	ns.Unlock()

	// On failure make sure to remove the endpoint
	defer func() {
		if err != nil {
			ns.Lock()
			delete(ns.endpoints, r.EndpointID)
			ns.Unlock()
		}
	}()

	endpoint.macAddress, err = net.ParseMAC(r.Interface.MacAddress)
	if err != nil {
		return fmt.Errorf("Parsing %s as Mac failed: %v", r.Interface.MacAddress, err)
	}
	_, endpoint.addr, err = net.ParseCIDR(r.Interface.Address)
	if err != nil {
		return fmt.Errorf("Parsing %s as CIDR failed: %v", r.Interface.Address, err)
	}
	_, endpoint.addrv6, err = net.ParseCIDR(r.Interface.AddressIPv6)
	if err != nil {
		return fmt.Errorf("Parsing %s as CIDR failed: %v", r.Interface.AddressIPv6, err)
	}

	// Program any required port mapping and store them in the endpoint
	endpoint.portMapping, err = ns.allocatePorts(epConfig, endpoint, defaultBindingIP, false)
	if err != nil {
		return err
	}

	return nil
}

func (d *Driver) DeleteEndpoint(r *dknet.DeleteEndpointRequest) error {
	logrus.Debugf("Delete endpoint request: %+v", r)

	// Get the network handler and make sure it exists
	d.Lock()
	ns, ok := d.networks[r.NetworkID]
	d.Unlock()
	if !ok {
		return types.InternalMaskableErrorf("network %s does not exist", r.NetworkID)
	}

	if ns == nil {
		return driverapi.ErrNoNetwork(r.NetworkID)
	}

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := ns.getEndpoint(r.EndpointID)
	if err != nil {
		return err
	}

	if ep == nil {
		return EndpointNotFoundError(r.EndpointID)
	}

	// Remove it
	ns.Lock()
	delete(ns.endpoints, r.EndpointID)
	ns.Unlock()

	// On failure make sure to set back ep in n.endpoints, but only
	// if it hasn't been taken over already by some other thread.
	defer func() {
		if err != nil {
			ns.Lock()
			if _, ok := ns.endpoints[r.EndpointID]; !ok {
				ns.endpoints[r.EndpointID] = ep
			}
			ns.Unlock()
		}
	}()

	// Remove port mappings. Do not stop endpoint delete on unmap failure
	if err := ns.releasePorts(ep); err != nil {
		return err
	}

	return nil
}

func (d *Driver) EndpointInfo(r *dknet.InfoRequest) (*dknet.InfoResponse, error) {
	res := &dknet.InfoResponse{
		Value: make(map[string]string),
	}
	return res, nil
}

func (d *Driver) Join(r *dknet.JoinRequest) (*dknet.JoinResponse, error) {
	bridgeName := d.networks[r.NetworkID].BridgeName

	// create and attach local name to the bridge
	localVethPair, err := vethPair(truncateID(r.EndpointID), bridgeName)
	if err != nil {
		logrus.Errorf("getting vethpair failed: %v", err)
		return nil, err
	}

	if err := netlink.LinkAdd(localVethPair); err != nil {
		logrus.Errorf("failed to create the veth pair named: [ %v ] error: [ %s ] ", localVethPair, err)
		return nil, err
	}
	// Bring the veth pair up
	if err := netlink.LinkSetUp(localVethPair); err != nil {
		logrus.Errorf("Error enabling Veth local iface: [ %v ]: %v", localVethPair, err)
		return nil, err
	}
	logrus.Infof("Attached veth [ %s ] to bridge [ %s ]", localVethPair.Name, bridgeName)

	// SrcName gets renamed to DstPrefix + ID on the container iface
	res := &dknet.JoinResponse{
		InterfaceName: dknet.InterfaceName{
			SrcName:   localVethPair.PeerName,
			DstPrefix: containerEthName,
		},
		Gateway: d.networks[r.NetworkID].Gateway,
	}
	logrus.Debugf("Join endpoint %s:%s to %s", r.NetworkID, r.EndpointID, r.SandboxKey)
	return res, nil
}

func (d *Driver) Leave(r *dknet.LeaveRequest) error {
	// make sure the network exists, like if the plugin stopped without cleaning up
	if _, ok := d.networks[r.NetworkID]; !ok {
		return fmt.Errorf("network id %s does not exist", r.NetworkID)
	}
	bridgeName := d.networks[r.NetworkID].BridgeName

	logrus.Debugf("Leave request: %+v", r)
	localVethPair, err := vethPair(truncateID(r.EndpointID), bridgeName)
	if err != nil {
		logrus.Errorf("getting vethpair failed: %v", err)
		return err
	}

	if err := netlink.LinkDel(localVethPair); err != nil {
		logrus.Errorf("unable to delete veth on leave: %s", err)
	}

	logrus.Debugf("Leave %s:%s", r.NetworkID, r.EndpointID)
	return nil
}

func NewDriver() (*Driver, error) {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %s", err)
	}

	d := &Driver{
		dockerer: dockerer{
			client: docker,
		},
		networks: make(map[string]*NetworkState),
	}
	return d, nil
}
