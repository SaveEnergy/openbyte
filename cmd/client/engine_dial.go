package client

import (
	"fmt"
	"net"
	"time"
)

func (e *TestEngine) captureConnectionNetworkInfo() {
	if len(e.connections) == 0 {
		return
	}
	tcpConn, ok := e.connections[0].(*net.TCPConn)
	if !ok {
		return
	}
	if addr := tcpConn.RemoteAddr(); addr != nil {
		e.networkInfo.SetServerIP(addr.String())
	}
	if addr := tcpConn.LocalAddr(); addr != nil {
		e.networkInfo.DetectNAT(addr.String(), e.networkInfo.ClientIP)
	}
}

func (e *TestEngine) createConnections() error {
	for i := 0; i < e.config.Streams; i++ {
		conn, err := e.dialConnection()
		if err != nil {
			e.closeConnections()
			return err
		}
		e.connections = append(e.connections, conn)
	}
	return nil
}

func (e *TestEngine) dialConnection() (net.Conn, error) {
	if e.config.Protocol == protocolUDP {
		return dialUDPConnection(e.config.ServerAddr)
	}
	return dialTCPConnection(e.config.ServerAddr)
}

func dialUDPConnection(serverAddr string) (net.Conn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve UDP: %w", err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial UDP: %w", err)
	}
	return conn, nil
}

func dialTCPConnection(serverAddr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial TCP: %w", err)
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetReadBuffer(256 * 1024)
		tcpConn.SetWriteBuffer(256 * 1024)
	}
	return conn, nil
}

func (e *TestEngine) closeConnections() {
	for _, conn := range e.connections {
		if conn != nil {
			conn.Close()
		}
	}
	e.connections = nil
}
