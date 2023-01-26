package main

import (
	"net"

	"github.com/milosgajdos/tenus"
	"github.com/rs/zerolog/log"
	"github.com/songgao/water"
)

// should this be in its own package?
// what are go best practices in separating packages?
type network struct {
	bridge     tenus.Bridger
	tapDevices []water.Interface
}

// Create and attach TAP interface to the network bridge
// Returns the created TAP interface
func (n *network) createTAP() *net.Interface {
	tap, err := water.New(water.Config{
		DeviceType: water.TAP,
	})
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	tapIfce, err := net.InterfaceByName(tap.Name())

	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	n.bridge.AddSlaveIfc(tapIfce)

	return tapIfce
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

	// Bring the bridge up
	if err = br.SetLinkUp(); err != nil {
		log.Fatal().Msg(err.Error())
	}

	network := network{bridge: br}
	return network
}
