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
	config := water.Config{
		DeviceType: water.TAP,
	}
	config.MultiQueue = true

	// So that TAP interface is not deleted after closing it
	config.Persist = true

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
