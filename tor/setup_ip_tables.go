package tor

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netutils"
	"github.com/sirupsen/logrus"
)

const (
	// TorChain is the TOR iptable chain name.
	TorChain                = "TOR"
	hairpinMode             = false
	torTransparentProxyPort = "22340"
	torDNSPort              = "22353"
)

func setupIPChains() (*iptables.ChainInfo, *iptables.ChainInfo, error) {
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

type iptablesConfig struct {
	bridgeName  string
	torIP       string
	addr        *net.IPNet
	hairpinMode bool
	iccMode     bool
	ipMasqMode  bool
	blockUDP    bool
}

func (n *NetworkState) setupIPTables(torIP string) error {
	addrv4, _, err := netutils.GetIfaceAddr(n.BridgeName)
	if err != nil {
		return fmt.Errorf("Failed to setup IP tables, cannot acquire interface address for bridge %s: %v", n.BridgeName, err)
	}

	ic := &iptablesConfig{
		bridgeName:  n.BridgeName,
		torIP:       torIP,
		hairpinMode: hairpinMode,
		iccMode:     true,
		ipMasqMode:  true,
		blockUDP:    n.blockUDP,
	}

	ipnet := addrv4.(*net.IPNet)
	ic.addr = &net.IPNet{
		IP:   ipnet.IP.Mask(ipnet.Mask),
		Mask: ipnet.Mask,
	}

	if err = ic.setupIPTablesInternal(true); err != nil {
		return fmt.Errorf("setup iptables failed for bridge %s: %v", n.BridgeName, err)
	}
	n.registerIptCleanFunc(func() error {
		return ic.setupIPTablesInternal(false)
	})

	err = iptables.ProgramChain(n.natChain, n.BridgeName, ic.hairpinMode, true)
	if err != nil {
		return fmt.Errorf("Failed to program NAT chain: %s", err.Error())
	}
	n.registerIptCleanFunc(func() error {
		return iptables.ProgramChain(n.natChain, n.BridgeName, ic.hairpinMode, false)
	})

	err = iptables.ProgramChain(n.filterChain, n.BridgeName, ic.hairpinMode, true)
	if err != nil {
		return fmt.Errorf("Failed to program FILTER chain: %s", err.Error())
	}
	n.registerIptCleanFunc(func() error {
		return iptables.ProgramChain(n.filterChain, n.BridgeName, ic.hairpinMode, false)
	})

	n.portMapper.SetIptablesChain(n.filterChain, n.BridgeName)

	// forward to tor
	if err := ic.forwardToTor(iptables.Insert); err != nil {
		return fmt.Errorf("Redirecting traffic from bridge (%s) to torIP (%s) via iptables failed: %v", n.BridgeName, torIP, err)
	}
	n.registerIptCleanFunc(func() error {
		return ic.forwardToTor(iptables.Delete)
	})

	return nil
}

type iptableCleanFunc func() error
type iptablesCleanFuncs []iptableCleanFunc

func (n *NetworkState) registerIptCleanFunc(clean iptableCleanFunc) {
	n.iptCleanFuncs = append(n.iptCleanFuncs, clean)
}

type iptRule struct {
	table   iptables.Table
	chain   string
	preArgs []string
	args    []string
}

func (ic *iptablesConfig) setupIPTablesInternal(enable bool) error {

	var (
		address   = ic.addr.String()
		natRule   = iptRule{table: iptables.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: []string{"-s", address, "!", "-o", ic.bridgeName, "-j", "MASQUERADE"}}
		hpNatRule = iptRule{table: iptables.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", ic.bridgeName, "-j", "MASQUERADE"}}
		outRule   = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-i", ic.bridgeName, "!", "-o", ic.bridgeName, "-j", "ACCEPT"}}
		inRule    = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-o", ic.bridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}}
	)

	// Set NAT.
	if ic.ipMasqMode {
		if err := programChainRule(natRule, "NAT", enable); err != nil {
			return err
		}
	}

	// In hairpin mode, masquerade traffic from localhost
	if ic.hairpinMode {
		if err := programChainRule(hpNatRule, "MASQ LOCAL HOST", enable); err != nil {
			return err
		}
	}

	// Set Inter Container Communication.
	if err := setIcc(ic.bridgeName, ic.iccMode, enable); err != nil {
		return err
	}

	// Set Accept on all non-intercontainer outgoing packets.
	if err := programChainRule(outRule, "ACCEPT NON_ICC OUTGOING", enable); err != nil {
		return err
	}

	// Set Accept on incoming packets for existing connections.
	return programChainRule(inRule, "ACCEPT INCOMING", enable)
}

func programChainRule(rule iptRule, ruleDescr string, enable bool) error {
	var (
		prefix    []string
		condition bool
		doesExist = iptables.Exists(rule.table, rule.chain, rule.args...)
	)

	action := iptables.Insert
	condition = !doesExist
	if !enable {
		action = iptables.Delete
		condition = doesExist
	}
	prefix = []string{string(action), rule.chain}

	if rule.preArgs != nil {
		prefix = append(rule.preArgs, prefix...)
	}

	if condition {
		if output, err := iptables.Raw(append(prefix, rule.args...)...); err != nil {
			return fmt.Errorf("Unable to %s %s rule: %v", action, ruleDescr, err)
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
		args       = []string{"-i", bridgeIface, "-o", bridgeIface, "-p", "tcp", "-j"}
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

func (ic *iptablesConfig) forwardToTor(action iptables.Action) error {
	// route dns requests
	args := []string{"-t", string(iptables.Nat), string(action), "PREROUTING",
		"-i", ic.bridgeName,
		"-p", "udp",
		"--dport", "53",
		"-j", "REDIRECT",
		"--to-ports", torDNSPort}
	if output, err := iptables.Raw(args...); err != nil {
		return err
	} else if len(output) != 0 {
		return iptables.ChainError{Chain: "PREROUTING", Output: output}
	}

	// route tcp requests
	args = []string{"-t", string(iptables.Nat), string(action), "PREROUTING",
		"-i", ic.bridgeName,
		"-p", "tcp",
		"--syn",
		"-j", "REDIRECT",
		"--to-ports", torTransparentProxyPort}
	if output, err := iptables.Raw(args...); err != nil {
		return err
	} else if len(output) != 0 {
		return iptables.ChainError{Chain: "PREROUTING", Output: output}
	}

	// block udp traffic
	if ic.blockUDP {
		args := []string{"-t", string(iptables.Filter), string(action), "FORWARD",
			"-i", ic.bridgeName,
			"-p", "udp",
			"-j", "DROP"}
		if output, err := iptables.Raw(args...); err != nil {
			return err
		} else if len(output) != 0 {
			return iptables.ChainError{Chain: "FORWARD", Output: output}
		}
		args = []string{"-t", string(iptables.Filter), string(action), "FORWARD",
			"-o", ic.bridgeName,
			"-p", "udp",
			"-j", "DROP"}
		if output, err := iptables.Raw(args...); err != nil {
			return err
		} else if len(output) != 0 {
			return iptables.ChainError{Chain: "FORWARD", Output: output}
		}
		args = []string{"-t", string(iptables.Filter), string(action), "TOR",
			"-p", "udp",
			"-j", "DROP"}
		if output, err := iptables.Raw(args...); err != nil {
			return err
		} else if len(output) != 0 {
			return iptables.ChainError{Chain: "FORWARD", Output: output}
		}
	}

	return nil
}
