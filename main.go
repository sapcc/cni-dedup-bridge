package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/vishvananda/netlink"
	utilebtables "k8s.io/kubernetes/pkg/util/ebtables"
	utilexec "k8s.io/utils/exec"
)

const (
	dedupChain = utilebtables.Chain("KUBE-DEDUP")
)

type NetConf struct {
	types.NetConf
	Device string `json:"device"` // Device-Name, something like eth0 or can0 etc.
}

func loadConf(bytes []byte) (*NetConf, error) {
	n := &NetConf{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}
	if n.Device == "" {
		return nil, fmt.Errorf(`specify "device"`)
	}
	return n, nil
}

func main() {
	// TODO: implement plugin version
	skel.PluginMain(cmdAdd, cmdDel, version.All)
}

func cmdAdd(args *skel.CmdArgs) error {

	cfg, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	link, err := netlink.LinkByName(cfg.Device)
	if err != nil {
		return fmt.Errorf("Failed to get device %s: %s", cfg.Device, err)
	}
	if link.Attrs().Promisc != 1 {
		// promiscuous mode is not on, then turn it on.
		err := netlink.SetPromiscOn(link)
		if err != nil {
			return fmt.Errorf("Error setting promiscuous mode on %s: %v", cfg.Device, err)
		}
	}

	cidr, err := getInterfaceAddr(cfg.Device)
	if err != nil {
		return fmt.Errorf("Failed to get CIDR from interface %s: %s", cfg.Device, err)
	}

	syncEbtablesDedupRules(cfg.Device, link.Attrs().HardwareAddr, cidr)

	return types.PrintResult(&current.Result{}, cfg.CNIVersion)

}

func cmdDel(args *skel.CmdArgs) error {
	//do nothing
	return nil
}

func syncEbtablesDedupRules(device string, macAddr net.HardwareAddr, cidr *net.IPNet) {

	dedupChain := utilebtables.Chain(strings.ToUpper(device) + "-DEDUP")

	ebtables := utilebtables.New(utilexec.New())
	if err := ebtables.FlushChain(utilebtables.TableFilter, dedupChain); err != nil {
		//glog.Errorf("Failed to flush dedup chain: %v", err)
	}
	_, err := ebtables.GetVersion()
	if err != nil {
		//glog.Warningf("Failed to get ebtables version. Skip syncing ebtables dedup rules: %v", err)
		return
	}

	//glog.V(3).Infof("Filtering packets with ebtables on mac address: %v, gateway: %v, pod CIDR: %v", macAddr.String(), plugin.gateway.String(), plugin.podCidr)
	_, err = ebtables.EnsureChain(utilebtables.TableFilter, dedupChain)
	if err != nil {
		//glog.Errorf("Failed to ensure %v chain %v", utilebtables.TableFilter, dedupChain)
		return
	}

	_, err = ebtables.EnsureRule(utilebtables.Append, utilebtables.TableFilter, utilebtables.ChainOutput, "-j", string(dedupChain))
	if err != nil {
		//glog.Errorf("Failed to ensure %v chain %v jump to %v chain: %v", utilebtables.TableFilter, utilebtables.ChainOutput, dedupChain, err)
		return
	}

	commonArgs := []string{"-p", "IPv4", "-s", macAddr.String(), "-o", "veth+"}
	_, err = ebtables.EnsureRule(utilebtables.Prepend, utilebtables.TableFilter, dedupChain, append(commonArgs, "--ip-src", cidr.IP.String(), "-j", "ACCEPT")...)
	if err != nil {
		//glog.Errorf("Failed to ensure packets from cbr0 gateway to be accepted")
		return

	}
	_, err = ebtables.EnsureRule(utilebtables.Append, utilebtables.TableFilter, dedupChain, append(commonArgs, "--ip-src", cidr.String(), "-j", "DROP")...)

	if err != nil {
		//glog.Errorf("Failed to ensure packets from podCidr but has mac address of cbr0 to get dropped.")
		return
	}
}
func getInterfaceAddr(name string) (*net.IPNet, error) {
	interf, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("Failed to get interface %s: %s", name, err)
	}

	addrs, err := interf.Addrs()
	if err != nil {
		return nil, fmt.Errorf("Failed to get addresses from %s: %s", name, err)
	}

	for _, addr := range addrs {
		var ipnet *net.IPNet
		switch v := addr.(type) {
		case *net.IPNet:
			ipnet = v
		default:
			continue
		}
		if ipnet.IP.IsLoopback() {
			continue
		}
		if ipnet.IP.To4() == nil {
			continue // not an ipv4 address
		}
		return ipnet, nil
	}
	return nil, fmt.Errorf("No ipv4 addresses found on %s", name)
}
