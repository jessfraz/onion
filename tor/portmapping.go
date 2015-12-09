package tor

import (
	"bytes"
	"errors"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
)

var (
	defaultBindingIP        = net.IPv4(0, 0, 0, 0)
	maxAllocatePortAttempts = 10
)

func (n *NetworkState) getEndpoint(eid string) (*torEndpoint, error) {
	n.Lock()
	defer n.Unlock()

	if eid == "" {
		return nil, InvalidEndpointIDError(eid)
	}

	if ep, ok := n.endpoints[eid]; ok {
		return ep, nil
	}

	return nil, nil
}

func (n *NetworkState) allocatePorts(epConfig *endpointConfiguration, ep *torEndpoint, reqDefBindIP net.IP, ulPxyEnabled bool) ([]types.PortBinding, error) {
	if epConfig == nil || epConfig.PortBindings == nil {
		return nil, nil
	}

	defHostIP := defaultBindingIP
	if reqDefBindIP != nil {
		defHostIP = reqDefBindIP
	}

	return n.allocatePortsInternal(epConfig.PortBindings, ep.addr.IP, defHostIP, ulPxyEnabled)
}

func (n *NetworkState) allocatePortsInternal(bindings []types.PortBinding, containerIP, defHostIP net.IP, ulPxyEnabled bool) ([]types.PortBinding, error) {
	bs := make([]types.PortBinding, 0, len(bindings))
	for _, c := range bindings {
		b := c.GetCopy()
		if err := n.allocatePort(&b, containerIP, defHostIP, ulPxyEnabled); err != nil {
			// On allocation failure, release previously allocated ports. On cleanup error, just log a warning message
			if cuErr := n.releasePortsInternal(bs); cuErr != nil {
				logrus.Warnf("Upon allocation failure for %v, failed to clear previously allocated port bindings: %v", b, cuErr)
			}
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, nil
}

func (n *NetworkState) allocatePort(bnd *types.PortBinding, containerIP, defHostIP net.IP, ulPxyEnabled bool) error {
	var (
		host net.Addr
		err  error
	)

	// Store the container interface address in the operational binding
	bnd.IP = containerIP

	// Adjust the host address in the operational binding
	if len(bnd.HostIP) == 0 {
		bnd.HostIP = defHostIP
	}

	// Adjust HostPortEnd if this is not a range.
	if bnd.HostPortEnd == 0 {
		bnd.HostPortEnd = bnd.HostPort
	}

	// Construct the container side transport address
	container, err := bnd.ContainerAddr()
	if err != nil {
		return err
	}

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	for i := 0; i < maxAllocatePortAttempts; i++ {
		if host, err = n.portMapper.MapRange(container, bnd.HostIP, int(bnd.HostPort), int(bnd.HostPortEnd), ulPxyEnabled); err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if bnd.HostPort != 0 {
			logrus.Warnf("Failed to allocate and map port %d-%d: %s", bnd.HostPort, bnd.HostPortEnd, err)
			break
		}
		logrus.Warnf("Failed to allocate and map port: %s, retry: %d", err, i+1)
	}
	if err != nil {
		return err
	}

	// Save the host port (regardless it was or not specified in the binding)
	switch netAddr := host.(type) {
	case *net.TCPAddr:
		bnd.HostPort = uint16(host.(*net.TCPAddr).Port)
		return nil
	case *net.UDPAddr:
		bnd.HostPort = uint16(host.(*net.UDPAddr).Port)
		return nil
	default:
		// For completeness
		return ErrUnsupportedAddressType(fmt.Sprintf("%T", netAddr))
	}
}

func (n *NetworkState) releasePorts(ep *torEndpoint) error {
	return n.releasePortsInternal(ep.portMapping)
}

func (n *NetworkState) releasePortsInternal(bindings []types.PortBinding) error {
	var errorBuf bytes.Buffer

	// Attempt to release all port bindings, do not stop on failure
	for _, m := range bindings {
		if err := n.releasePort(m); err != nil {
			errorBuf.WriteString(fmt.Sprintf("\ncould not release %v because of %v", m, err))
		}
	}

	if errorBuf.Len() != 0 {
		return errors.New(errorBuf.String())
	}
	return nil
}

func (n *NetworkState) releasePort(bnd types.PortBinding) error {
	// Construct the host side transport address
	host, err := bnd.HostAddr()
	if err != nil {
		return err
	}
	return n.portMapper.Unmap(host)
}

func parseEndpointOptions(epOptions map[string]interface{}) (*endpointConfiguration, error) {
	if epOptions == nil {
		return nil, nil
	}

	ec := &endpointConfiguration{}

	if opt, ok := epOptions[netlabel.PortMap]; ok {
		if bs, ok := opt.([]types.PortBinding); ok {
			ec.PortBindings = bs
		} else {
			logrus.Errorf("portbinding error: %#v", epOptions)
			//return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.ExposedPorts]; ok {
		if ports, ok := opt.([]types.TransportPort); ok {
			ec.ExposedPorts = ports
		} else {
			logrus.Errorf("transport port error: %#v", epOptions)
			//return nil, &ErrInvalidEndpointConfig{}
		}
	}

	return ec, nil
}
