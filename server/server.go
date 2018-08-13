package server

import (
	"context"
	"net"
	"strings"
	"sync"
)

// Server struct
type Server struct {
	listenAddr *net.TCPAddr
	remoteAddr *net.TCPAddr
	ctx        context.Context
	shutdown   context.CancelFunc
	Wg         *sync.WaitGroup
	ClosedChan chan struct{}
	listener   *net.TCPListener
}

// NewServer returns a new Server
func NewServer(ctx context.Context, lAddr, rAddr *net.TCPAddr) *Server {
	innerCtx, shutdown := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}
	closedChan := make(chan struct{})

	return &Server{
		listenAddr: lAddr,
		remoteAddr: rAddr,
		ctx:        innerCtx,
		shutdown:   shutdown,
		Wg:         wg,
		ClosedChan: closedChan,
	}
}

// Start server.
func (s *Server) Start() error {
	lt, err := net.ListenTCP("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	s.listener = lt
	defer func() {
		lt.Close()
		close(s.ClosedChan)
	}()

	for {
		conn, err := lt.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					continue
				}
				if !strings.Contains(err.Error(), "use of closed network connection") {
					select {
					case <-s.ctx.Done():
						break
					default:
					}
				}
			}
			return err
		}
		s.Wg.Add(1)
		go s.handleConn(conn)
	}
}

// Shutdown server.
func (s *Server) Shutdown() {
	select {
	case <-s.ctx.Done():
	default:
		s.shutdown()
		s.listener.Close()
	}
}

func (s *Server) handleConn(conn *net.TCPConn) {
	defer func() {
		conn.Close()
		s.Wg.Done()
	}()

	remoteConn, err := net.DialTCP("tcp", nil, s.remoteAddr)
	defer remoteConn.Close()
	if err != nil {
		// TODO: error handling
		return
	}

	p := NewProxy(s.ctx, conn, remoteConn)
	defer p.Close()

	p.Start()
}
