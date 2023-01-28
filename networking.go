package main

import (
	"context"
	"net"

	goipam "github.com/metal-stack/go-ipam"
	"github.com/milosgajdos/tenus"
	"github.com/rs/zerolog/log"
	"github.com/songgao/water"
)

// should this be in its own package?
// what are go best practices in separating packages?
type network struct {
	bridge     tenus.Bridger
	tapDevices []water.Interface
	ipam       goipam.Ipamer
	prefix     goipam.Prefix
}

// Create and attach TAP interface to the network bridge
// Returns the created TAP interface
func (n *network) createTAP() *net.Interface {

	config := water.Config{
		DeviceType: water.TAP,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Persist: true, // So that TAP interface is not deleted after closing it. We have to close the TAP interface for firecracker to use.
		},
	}

	tap, err := water.New(config)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	// Close its descriptor so that firecracker can use the TAP interface
	err = tap.Close()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	tapIfce, err := net.InterfaceByName(tap.Name())
	if err != nil {
		log.Fatal().Err(err).Msgf("Umable to find %s interface", tap.Name())
	}

	n.bridge.AddSlaveIfc(tapIfce)

	return tapIfce
}

func (n *network) claimNextIp() (net.IP, *net.IPNet) {
	ipam := n.ipam
	prefix := n.prefix

	goipamIp, err := ipam.AcquireIP(context.Background(), prefix.Cidr)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	ip := net.ParseIP(goipamIp.IP.String())

	_, ipNet, err := net.ParseCIDR(prefix.Cidr)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	return ip, ipNet
}

func newNetwork() network {

	bridgeName := "firecracker-br"

	// Get existing bridge, if not create new
	br, err := tenus.BridgeFromName(bridgeName)
	if err != nil {
		log.Debug().Msgf("Bridge %s does not exist, creating..", bridgeName)
		br, err = tenus.NewBridgeWithName("firecracker-br")
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	} else {
		log.Debug().Msgf("Bridge %s exists, reusing..", bridgeName)
	}

	// Create IPAMer and init prefix
	ipam := goipam.New()
	prefix, err := ipam.NewPrefix(context.Background(), "172.16.0.0/24")
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	network := network{bridge: br, ipam: ipam, prefix: *prefix}

	// Assign IP address to bridge
	bridgeIp, bridgeIpNet := network.claimNextIp()
	network.bridge.SetLinkIp(bridgeIp, bridgeIpNet)

	// Bring the bridge up
	if err = br.SetLinkUp(); err != nil {
		log.Fatal().Msg(err.Error())
	}

	return network
}
