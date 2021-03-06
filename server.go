package stun

import (
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
)

// Server is RFC 5389 basic server implementation.
//
// Current implementation is UDP only and not utilizes FINGERPRINT mechanism,
// nor ALTERNATE-SERVER, nor credentials mechanisms. It does not support
// backwards compatibility with RFC 3489.
//
// The STUN server MUST support the Binding method.  It SHOULD NOT
// utilize the short-term or long-term credential mechanism.  This is
// because the work involved in authenticating the request is more than
// the work in simply processing it.  It SHOULD NOT utilize the
// ALTERNATE-SERVER mechanism for the same reason.  It MUST support UDP
// and TCP.  It MAY support STUN over TCP/TLS; however, TLS provides
// minimal security benefits in this basic mode of operation.  It MAY
// utilize the FINGERPRINT mechanism but MUST NOT require it.  Since the
// stand-alone server only runs STUN, FINGERPRINT provides no benefit.
// Requiring it would break compatibility with RFC 3489, and such
// compatibility is desirable in a stand-alone server.  Stand-alone STUN
// servers SHOULD support backwards compatibility with [RFC3489]
// clients, as described in Section 12.
//
// It is RECOMMENDED that administrators of STUN servers provide DNS
// entries for those servers as described in Section 9.
//
// A basic STUN server is not a solution for NAT traversal by itself.
// However, it can be utilized as part of a solution through STUN
// usages.  This is discussed further in Section 14.
type Server struct {
	Addr         string
	Logger       Logger
	Name         string
	LogAllErrors bool
}

// Logger is used for logging formatted messages.
type Logger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...interface{})
}

var (
	defaultLogger = logrus.New()
)

const defaultName = "cydev/stun"

func (s *Server) logger() Logger {
	if s.Logger == nil {
		return defaultLogger
	}
	return s.Logger
}

func (s *Server) getName() string {
	if len(s.Name) == 0 {
		return defaultName
	}
	return s.Name
}

func (s *Server) serveConn(c net.PacketConn) error {
	if c == nil {
		return nil
	}
	m := AcquireMessage()
	buf := make([]byte, 2048)
	n, addr, err := c.ReadFrom(buf)
	if err != nil {
		s.logger().Printf("ReadFrom: %v", err)
		return err
	}
	if _, err = m.ReadBytes(buf[:n]); err != nil {
		s.logger().Printf("ReadFrom: %v", err)
		// TODO: return error
	}
	// TODO: check request MessageType
	t := MessageType{
		Method: MethodBinding,
		Class:  ClassSuccessResponse,
	}
	res := AcquireFields(Message{
		TransactionID: m.TransactionID,
		Type:          t,
	})
	var (
		ip   net.IP
		port int
	)
	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	default:
		panic(fmt.Sprintf("unknown addr: %v", addr))
	}
	res.AddXORMappedAddress(ip, port)
	res.AddSoftware(s.getName())
	res.WriteHeader()
	_, err = c.WriteTo(res.buf.B, addr)
	if err != nil {
		s.logger().Printf("WriteTo: %v", err)
	}
	ReleaseMessage(m)
	ReleaseMessage(res)
	return err
}

// Serve reads packets from connections and responds to BINDING requests.
func (s *Server) Serve(c net.PacketConn) error {
	for {
		if err := s.serveConn(c); err != nil {
			s.logger().Printf("serve: %v", err)
			return err
		}
	}
}

// ListenUDPAndServe listens on laddr and process incoming packets.
func ListenUDPAndServe(serverNet, laddr string) error {
	c, err := net.ListenPacket(serverNet, laddr)
	if err != nil {
		return err
	}
	s := &Server{}
	return s.Serve(c)
}
