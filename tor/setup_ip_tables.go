// +build ignore

package tor

import (
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netutils"
)

// TorChain: TOR iptable chain name
const (
	TorChain    = "TOR"
	hairpinMode = false
)

func setupIPChains(config *configuration) (*iptables.ChainInfo, *iptables.ChainInfo, error) {
	natChain, err := iptables.NewChain(TorChain, iptables.Nat, hairpinMode)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create NAT chain: %s", err.Error())
	}
	defer func() {
		if err != nil {
			if err := iptables.RemoveExistingChain(TorChain, iptables.Nat); err != nil {
				logrus.Warnf("Failed on removing iptables NAT chain on cleanup: %v", err)
			}
		}
	}()

	filterChain, err := iptables.NewChain(TorChain, iptables.Filter, hairpinMode)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create FILTER chain: %s", err.Error())
	}

	return natChain, filterChain, nil
}

func (n *NetworkState) setupIPTables(bridgeName string) error {
	addrv4, _, err := netutils.GetIfaceAddr(bridgeName)
	if err != nil {
		return fmt.Errorf("Failed to setup IP tables, cannot acquire interface address for bridge %s: %v", bridgeName, err)
	}
	ipnet := addrv4.(*net.IPNet)
	maskedAddrv4 := &net.IPNet{
		IP:   ipnet.IP.Mask(ipnet.Mask),
		Mask: ipnet.Mask,
	}

	if err = setupIPTablesInternal(bridgeName, maskedAddrv4, hairpinMode, true); err != nil {
		return fmt.Errorf("setup iptables failed for bridge %s: %v", bridgeName, err)
	}
	n.registerIptCleanFunc(func() error {
		return setupIPTablesInternal(bridgeName, maskedAddrv4, hairpinMode, false)
	})

	natChain, filterChain, err := n.getDriverChains()
	if err != nil {
		return fmt.Errorf("Failed to setup IP tables, cannot acquire chain info %s", err.Error())
	}

	err = iptables.ProgramChain(natChain, bridgeName, hairpinMode, true)
	if err != nil {
		return fmt.Errorf("Failed to program NAT chain: %s", err.Error())
	}

	err = iptables.ProgramChain(filterChain, bridgeName, hairpinMode, true)
	if err != nil {
		return fmt.Errorf("Failed to program FILTER chain: %s", err.Error())
	}
	n.registerIptCleanFunc(func() error {
		return iptables.ProgramChain(filterChain, bridgeName, hairpinMode, false)
	})

	n.portMapper.SetIptablesChain(filterChain, n.getNetworkBridgeName())

	return nil
}

type iptRule struct {
	table   iptables.Table
	chain   string
	preArgs []string
	args    []string
}

func setupIPTablesInternal(bridgeIface string, addr net.Addr, hairpin, enable bool) error {

	var (
		address   = addr.String()
		natRule   = iptRule{table: iptables.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: []string{"-s", address, "!", "-o", bridgeIface, "-j", "MASQUERADE"}}
		hpNatRule = iptRule{table: iptables.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", bridgeIface, "-j", "MASQUERADE"}}
		outRule   = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}}
		inRule    = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-o", bridgeIface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}}
	)

	// Set NAT.
	if err := programChainRule(natRule, "NAT", enable); err != nil {
		return err
	}

	// In hairpin mode, masquerade traffic from localhost
	if hairpin {
		if err := programChainRule(hpNatRule, "MASQ LOCAL HOST", enable); err != nil {
			return err
		}
	}

	// Set Inter Container Communication.
	if err := setIcc(bridgeIface, true, enable); err != nil {
		return err
	}

	// Set Accept on all non-intercontainer outgoing packets.
	if err := programChainRule(outRule, "ACCEPT NON_ICC OUTGOING", enable); err != nil {
		return err
	}

	// Set Accept on incoming packets for existing connections.
	if err := programChainRule(inRule, "ACCEPT INCOMING", enable); err != nil {
		return err
	}

	return nil
}

func programChainRule(rule iptRule, ruleDescr string, insert bool) error {
	var (
		prefix    []string
		operation string
		condition bool
		doesExist = iptables.Exists(rule.table, rule.chain, rule.args...)
	)

	if insert {
		condition = !doesExist
		prefix = []string{"-I", rule.chain}
		operation = "enable"
	} else {
		condition = doesExist
		prefix = []string{"-D", rule.chain}
		operation = "disable"
	}
	if rule.preArgs != nil {
		prefix = append(rule.preArgs, prefix...)
	}

	if condition {
		if output, err := iptables.Raw(append(prefix, rule.args...)...); err != nil {
			return fmt.Errorf("Unable to %s %s rule: %v", operation, ruleDescr, err)
		} else if len(output) != 0 {
			return &iptables.ChainError{Chain: rule.chain, Output: output}
		}
	}

	return nil
}

func setIcc(bridgeIface string, iccEnable, insert bool) error {
	var (
		table      = iptables.Filter
		chain      = "FORWARD"
		args       = []string{"-i", bridgeIface, "-o", bridgeIface, "-j"}
		acceptArgs = append(args, "ACCEPT")
		dropArgs   = append(args, "DROP")
	)

	if insert {
		if !iccEnable {
			iptables.Raw(append([]string{"-D", chain}, acceptArgs...)...)

			if !iptables.Exists(table, chain, dropArgs...) {
				if output, err := iptables.Raw(append([]string{"-A", chain}, dropArgs...)...); err != nil {
					return fmt.Errorf("Unable to prevent intercontainer communication: %v", err)
				} else if len(output) != 0 {
					return fmt.Errorf("Error disabling intercontainer communication: %s", output)
				}
			}
		} else {
			iptables.Raw(append([]string{"-D", chain}, dropArgs...)...)

			if !iptables.Exists(table, chain, acceptArgs...) {
				if output, err := iptables.Raw(append([]string{"-I", chain}, acceptArgs...)...); err != nil {
					return fmt.Errorf("Unable to allow intercontainer communication: %v", err)
				} else if len(output) != 0 {
					return fmt.Errorf("Error enabling intercontainer communication: %s", output)
				}
			}
		}
	} else {
		// Remove any ICC rule.
		if !iccEnable {
			if iptables.Exists(table, chain, dropArgs...) {
				iptables.Raw(append([]string{"-D", chain}, dropArgs...)...)
			}
		} else {
			if iptables.Exists(table, chain, acceptArgs...) {
				iptables.Raw(append([]string{"-D", chain}, acceptArgs...)...)
			}
		}
	}

	return nil
}
