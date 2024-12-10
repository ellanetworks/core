package nms

import (
	"github.com/yeastengine/ella/internal/nms/server"
)

func Start(port int, cert string, key string) error {
	server.Start(port, cert, key)
	return nil
}
