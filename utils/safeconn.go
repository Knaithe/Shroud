package utils

import (
	"net"
	"sync"
)

type SafeConn struct {
	net.Conn
	writeMu sync.Mutex
}

func (c *SafeConn) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.Conn.Write(data)
}

func WrapConn(conn net.Conn) net.Conn {
	if _, ok := conn.(*SafeConn); ok {
		return conn
	}
	return &SafeConn{Conn: conn}
}
