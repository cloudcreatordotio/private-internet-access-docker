package openvpn

import (
	"os"
	libuser "os/user"
	"strconv"

	"github.com/qdm12/private-internet-access-docker/internal/constants"
)

// WriteAuthFile writes the OpenVPN auth file to disk with the right permissions
func (c *configurator) WriteAuthFile(user, password string) error {
	// TODO bundle all IO operations in files interface Files
	authExists, err := c.fileManager.FileExists(constants.OpenVPNAuthConf)
	if err != nil {
		return err
	} else if authExists { // in case of container stop/start
		c.logger.Info("%s already exists", constants.OpenVPNAuthConf)
		return nil
	}
	c.logger.Info("Writing credentials to %s", constants.OpenVPNAuthConf)
	c.fileManager.WriteLinesToFile(constants.OpenVPNAuthConf, []string{user, password})
	userObject, err := libuser.Lookup("nonrootuser")
	if err != nil {
		return err
	}
	// Operations below are run as root
	uid, err := strconv.Atoi(userObject.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(userObject.Uid)
	if err != nil {
		return err
	}
	if err := os.Chown(constants.OpenVPNAuthConf, uid, gid); err != nil {
		return err
	}
	if err := os.Chmod(constants.OpenVPNAuthConf, 0400); err != nil {
		return err
	}
	return nil
}