package tor

import (
	"fmt"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
)

// initBridge creates a bridge if it does not exist
func (d *Driver) initBridge(id string) error {
	// try to get bridge by name, if it already exists then just exit
	bridgeName := d.networks[id].BridgeName
	_, err := net.InterfaceByName(bridgeName)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// get the MTU for bridge
	mtu := d.networks[id].MTU

	// create *netlink.Bridge object
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	la.MTU = mtu
	br := &netlink.Bridge{la}
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("Bridge creation failed for bridge %s: %v", bridgeName, err)
	}

	// Set bridge IP
	gatewayIP := d.networks[id].Gateway + "/" + d.networks[id].GatewayMask
	if err := setInterfaceIP(bridgeName, gatewayIP); err != nil {
		return fmt.Errorf("Error assigning address: %s on bridge: %s with an error of: %v", gatewayIP, bridgeName, err)
	}

	// Validate that the IPAddress is there!
	_, err = getIfaceAddr(bridgeName)
	if err != nil {
		return fmt.Errorf("No IP address found on bridge %s: %v", bridgeName, err)
	}

	// Add NAT rules for iptables
	if err = natOut(gatewayIP, false); err != nil {
		return fmt.Errorf("Could not set NAT rules for bridge %s: %v", bridgeName, err)
	}

	// Bring the bridge up
	if err := interfaceUp(bridgeName); err != nil {
		return fmt.Errorf("Error enabling bridge for %s: %v", bridgeName, err)
	}

	return nil
}

// deleteBridge deletes the bridge
func (d *Driver) deleteBridge(id string) error {
	// get the link
	bridgeName := d.networks[id].BridgeName
	l, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("Getting link with name %s failed: %v", bridgeName, err)
	}

	// Delete NAT rules for iptables
	gatewayIP := d.networks[id].Gateway + "/" + d.networks[id].GatewayMask
	if err = natOut(gatewayIP, true); err != nil {
		return fmt.Errorf("Could not delete NAT rules for bridge %s: %v", bridgeName, err)
	}

	// delete the link
	if err := netlink.LinkDel(l); err != nil {
		return fmt.Errorf("Failed to remove bridge interface %s delete: %v", bridgeName, err)
	}

	return nil
}
