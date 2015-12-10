package tor

import (
	"fmt"
	"net"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// initBridge creates a bridge if it does not exist
func (ns *NetworkState) initBridge() error {
	// try to get bridge by name, if it already exists then just exit
	bridgeName := ns.BridgeName
	_, err := net.InterfaceByName(bridgeName)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// create *netlink.Bridge object
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	la.MTU = ns.MTU
	br := &netlink.Bridge{la}
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("Bridge creation failed for bridge %s: %v", bridgeName, err)
	}

	// Set bridge IP
	gatewayIP := ns.Gateway + "/" + ns.GatewayMask
	if err := setInterfaceIP(bridgeName, gatewayIP); err != nil {
		return fmt.Errorf("Error assigning address: %s on bridge: %s with an error of: %v", gatewayIP, bridgeName, err)
	}

	// Validate that the IPAddress is there!
	_, err = getIfaceAddr(bridgeName)
	if err != nil {
		return fmt.Errorf("No IP address found on bridge %s: %v", bridgeName, err)
	}

	// Bring the bridge up
	if err := interfaceUp(bridgeName); err != nil {
		return fmt.Errorf("Error enabling bridge for %s: %v", bridgeName, err)
	}

	// Setup iptables
	if err := ns.setupIPTables(); err != nil {
		return fmt.Errorf("Error setting up iptables for %s: %v", bridgeName, err)
	}

	return nil
}

// deleteBridge deletes the bridge
func (ns *NetworkState) deleteBridge(id string) error {
	bridgeName := ns.BridgeName

	// get the link
	l, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("Getting link with name %s failed: %v", bridgeName, err)
	}

	// delete the link
	if err := netlink.LinkDel(l); err != nil {
		return fmt.Errorf("Failed to remove bridge interface %s delete: %v", bridgeName, err)
	}

	// delete all relevant iptables rules
	for _, cleanFunc := range ns.iptCleanFuncs {
		if err := cleanFunc(); err != nil {
			logrus.Warnf("Failed to clean iptables rules for bridge %s: %v", bridgeName, err)
		}
	}

	return nil
}
