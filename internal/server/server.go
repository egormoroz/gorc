package server

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)

type uId uint64

type Server struct {
	listener net.Listener

	// for simplicity, there always is a single master
	// master struct is threadsafe, but a pointer isn't, so we use atomic
	mast atomic.Pointer[master]

	clients    map[uId]*client
	clientsMtx sync.RWMutex

	chandlers map[pb.MessageKind]CMsgHandler // client message handlers
	mhandlers map[pb.MessageKind]MMsgHandler // master message handlers
}

func Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s := &Server{
		listener: listener,

		clients: make(map[uId]*client),

		// immutable during runtime
		chandlers: make(map[pb.MessageKind]CMsgHandler),
		mhandlers: make(map[pb.MessageKind]MMsgHandler),
	}
	s.registerHandlers()

	fmt.Println("listening on", addr)
	return s.listen()
}

func (s *Server) listen() error {
	defer s.listener.Close()
	// make sure id 0 is invalid: if clientId unspecified in message, protobuf defaults it to 0
	var id uId = 1
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleUnauthorized(id, conn)
		id++
	}
}

func (s *Server) handleUnauthorized(id uId, conn net.Conn) {
	defer conn.Close()
	// silly substition for proper authorization
	common.SendMessage(conn, &pb.Message{Kind: pb.MessageKind_LOGIN_GET})

	msg, err := common.RecvMessage(conn)
	if err != nil {
		return
	}
	if msg.Kind != pb.MessageKind_LOGIN_RESP {
		return
	}

	sendStopped := make(chan struct{})
	sendRoutine := func(send <-chan *pb.Message) {
		// just in case close the recv routine indirectly, which cleans everything up
		defer conn.Close()
		for msg := range send {
			err := common.SendMessage(conn, msg)
			if err != nil {
				break
			}
		}
		sendStopped <- struct{}{}
	}

	login := msg.GetLogin()
	switch login.Role {
	case pb.Role_CLIENT:
		info := login.GetInfo()
		if info == nil {
			return
		}

		c := newClient(conn, id, info.Uname)
		s.addClient(c)
		go sendRoutine(c.send)
		s.clientRecvRoutine(c)

		close(c.send)
		<-sendStopped
		s.remClient(c)
	case pb.Role_MASTER:
		mast := newMaster(conn, id)
		if !s.mast.CompareAndSwap(nil, mast) {
			return
		}

		fmt.Println("new master")

		go sendRoutine(mast.send)
		s.masterRecvRoutine(mast)

		close(mast.send)
		<-sendStopped
		s.mast.Store(nil)
	}
}

func (s *Server) addClient(c *client) {
	s.clientsMtx.Lock()
	s.clients[c.id] = c
	s.clientsMtx.Unlock()

	// TODO: notify master
	fmt.Println("new client", c.id)
}

func (s *Server) remClient(c *client) {
	exists := false
	s.clientsMtx.Lock()
	if _, ok := s.clients[c.id]; ok {
		delete(s.clients, c.id)
		exists = true
	}
	s.clientsMtx.Unlock()

	if exists {
		// TODO: notify master
		fmt.Println("lost connection with client", c.id)
	}
}

func (s *Server) clientRecvRoutine(c *client) {
	for {
		msg, err := common.RecvMessage(c.conn)
		if err != nil {
			break
		}

		if handler, ok := s.chandlers[msg.Kind]; ok {
			handler(s, c, msg)
		} else {
			fmt.Println("unexpected message kind:", msg.Kind)
		}
	}
}

func (s *Server) masterRecvRoutine(m *master) {
	for {
		msg, err := common.RecvMessage(m.conn)
		if err != nil {
			break
		}

		if handler, ok := s.mhandlers[msg.Kind]; ok {
			handler(s, m, msg)
		} else {
			fmt.Println("unexpected message kind:", msg.Kind)
			fmt.Println("available:", s.mhandlers)
		}
	}
}
