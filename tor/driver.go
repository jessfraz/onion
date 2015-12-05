package tor

import (
	"fmt"

	"github.com/Sirupsen/logrus"
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
}

// NetworkState is filled in at network creation time
// it contains state that we wish to keep for each network
type NetworkState struct {
	BridgeName  string
	MTU         int
	Gateway     string
	GatewayMask string
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
	return nil
}

func (d *Driver) DeleteEndpoint(r *dknet.DeleteEndpointRequest) error {
	logrus.Debugf("Delete endpoint request: %+v", r)
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
