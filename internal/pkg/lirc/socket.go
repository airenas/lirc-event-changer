package lirc

import (
	"net"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

func NewSocket(path string) (net.Listener, error) {
	logrus.Infof("Open write socket at: %s", path)

	if err := syscall.Unlink(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	mask := syscall.Umask(0777)
	defer syscall.Umask(mask)

	res, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chown(path, 0, 0); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0666); err != nil {
		return nil, err
	}
	return res, nil
}
