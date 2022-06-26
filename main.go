package main

import (
	"errors"
	"fmt"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/040"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var l *log.Logger

func init() {
	f, err := os.OpenFile("/var/log/toycni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	l = log.New(f, "", 0)
	rand.Seed(time.Now().UTC().UnixNano())
}

func cmdAdd(args *skel.CmdArgs) error {
	conf, err := parseNetConf(args.StdinData)

	// Result will be printed to stdout if the invocation succeeds. We don't fully populate the object for simplicity.
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		Interfaces: []*current.Interface{},
		IPs:        []*current.IPConfig{},
		Routes:     []*types.Route{},
		DNS:        conf.DNS,
	}

	// Create bridge if missing. Make sure it's up, has IP and route exists.
	// Use first IP in bridgeCidr and random MAC for bridge.
	if err = setupBridge(conf.BridgeName, conf.BridgeCidr); err != nil {
		return err
	}
	// Run the IPAM plugin and get back the config to apply
	r, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}

	success := true
	// Release IP in case of failure
	defer func() {
		if !success {
			ipam.ExecDel(conf.IPAM.Type, args.StdinData)
		}
	}()

	// Convert whatever the IPAM result was into the current Result type
	ipamResult, err := current.NewResultFromResult(r)
	if err != nil {
		success = false
		return err
	}

	result.IPs = ipamResult.IPs
	result.Routes = ipamResult.Routes

	if len(result.IPs) == 0 {
		success = false
		return errors.New("IPAM plugin returned missing IP config")
	}
	containerIp := ipamResult.IPs[0].Address
	gatewayIp := ipamResult.IPs[0].Gateway

	// Create and configure veth pair
	hostIf, contIf, err := setupVeth(conf, args, containerIp, gatewayIp)
	result.Interfaces = []*current.Interface{hostIf, contIf}
	if err != nil {
		success = false
		return err
	}

	return types.PrintResult(result, current.ImplementedSpecVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	conf, err := parseNetConf(args.StdinData)
	if err != nil {
		return err
	}
	// Assume the runtime deletes the network namespace which deletes the veth pair automatically.
	// Call IPAM plugin to free IP
	return ipam.ExecDel(conf.IPAM.Type, args.StdinData)
}

func cmdCheck(args *skel.CmdArgs) error {
	_, err := parseNetConf(args.StdinData)
	if err != nil {
		return err
	}
	return errors.New("not implemented")
}

func ipHost(args ...string) error {
	argSlice := []string{"ip"}
	for _, v := range args {
		argSlice = append(argSlice, strings.Split(v, " ")...)
	}
	return run(argSlice...)
}

func ipContainer(netnsPath string, args ...string) error {
	netnsName := filepath.Base(netnsPath)
	argSlice := []string{"ip", "netns", "exec", netnsName, "ip"}
	for _, v := range args {
		argSlice = append(argSlice, strings.Split(v, " ")...)
	}
	return run(argSlice...)
}

func bridgeExists(bridgeName string) bool {
	// Check if bridge interface exists
	err := ipHost("link show", bridgeName)
	// Bridge is assumed to exist if command succeeds
	return err == nil
}

func setupBridge(bridgeName, bridgeCidr string) error {
	// Assume bridge is fully configured if it exists.
	if exists := bridgeExists(bridgeName); exists {
		l.Println("bridge already exists")
		return nil
	}

	// Create bridge
	if err := ipHost("link add name", bridgeName, "type bridge"); err != nil {
		return errors.New("failed to create bridge")
	}

	// Assign ip to bridge and implicitly create route to direct packets with
	// destination IP in our node container subnet to the bridge
	// Continue on error to prevent race condition
	_ = ipHost("addr add", bridgeCidr, "dev", bridgeName)

	// Assign random MAC to bridge. Otherwise, the bridge will take the
	// lowest-numbered mac on the bridge, and will change as interfaces churn.
	mac, err := generateMac()
	if err != nil {
		return err
	}
	// Continue on error to prevent race condition
	_ = ipHost("link set dev", bridgeName, "address", mac)
	// Bring bridge up
	if err := ipHost("link set", bridgeName, "up"); err != nil {
		return errors.New("failed to bring bridge up")
	}

	return nil
}

func setupVeth(conf *NetConf, args *skel.CmdArgs, containerIp net.IPNet, gwIP net.IP) (*current.Interface, *current.Interface, error) {
	// Create veth pair in container
	hostIf := &current.Interface{
		Name:    "veth" + randStringBytes(5),
		Sandbox: "",
		Mac:     "",
	}
	contIf := &current.Interface{
		Name:    args.IfName,
		Sandbox: args.Netns,
		Mac:     "",
	}
	if err := ipContainer(args.Netns, "link add", contIf.Name, "type veth peer name", hostIf.Name); err != nil {
		return hostIf, contIf, errors.New("failed to create veth pair in container")
	}
	// Move host interface to root network namespace
	if err := ipContainer(args.Netns, "link set", hostIf.Name, "netns 1"); err != nil {
		return hostIf, contIf, errors.New("failed to move host interface to root network namespace")
	}

	// Configure host interface of veth pair. Bring it up and connect to bridge.
	if err := ipHost("link set", hostIf.Name, "up"); err != nil {
		return hostIf, contIf, errors.New("failed to bring host interface of veth pair up")
	}
	if err := ipHost("link set", hostIf.Name, "master", conf.BridgeName); err != nil {
		return hostIf, contIf, errors.New("failed to connect host interface of veth pair to bridge")
	}

	// Configure container interface of veth pair
	if err := ipContainer(args.Netns, "link set", contIf.Name, "up"); err != nil {
		return hostIf, contIf, err
	}
	// Set IP
	if err := ipContainer(args.Netns, "addr add", containerIp.String(), "dev", contIf.Name); err != nil {
		return hostIf, contIf, err
	}
	// Set bridge as default gateway in container, need to set container IP first
	if err := ipContainer(args.Netns, "route add default via", gwIP.String(), "dev", contIf.Name); err != nil {
		return hostIf, contIf, errors.New("failed to set bridge as default gateway in container")
	}
	return hostIf, contIf, nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("toycni"))
}
