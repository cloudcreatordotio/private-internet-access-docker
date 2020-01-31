package openvpn

import (
	"fmt"
	"os"

	"github.com/qdm12/private-internet-access-docker/internal/constants"
)

// CheckTUN checks the tunnel device is present and accessible
func (c *configurator) CheckTUN() error {
	f, err := c.openFile(constants.TunnelDevice, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("TUN device is not available: %w", err)
	}
	if err := f.Close(); err != nil {
		c.logger.Warn("Could not close TUN device file: %s", err)
	}
	return nil
}