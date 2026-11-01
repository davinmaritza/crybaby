package cluster

import (
	"errors"
	"fmt"
	"net"
)

// SendWakeOnLAN constructs and transmits a magic packet over UDP broadcast to wake a sleeping PC.
func SendWakeOnLAN(macAddrStr string) error {
	hwAddr, err := net.ParseMAC(macAddrStr)
	if err != nil {
		return fmt.Errorf("invalid MAC address '%s': %w", macAddrStr, err)
	}

	if len(hwAddr) != 6 {
		return errors.New("MAC address must be 6 bytes (EUI-48)")
	}

	// A Wake-on-LAN magic packet contains 6 bytes of 0xFF followed by 16 repetitions of the target MAC.
	var packet [102]byte
	copy(packet[0:6], []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:6+(i+1)*6], hwAddr)
	}

	bcastAddr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:9")
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp4", nil, bcastAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(packet[:])
	return err
}
