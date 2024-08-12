package server

import (
	"net"
	"time"

	"github.com/egormoroz/gorc/internal/pb"
)

// threadsafe
type client struct {
	conn net.Conn
    send chan *pb.Message

    id uId // immutable
    connTime int64 // immutable
    uname string // immutable
}

// threadsafe
type master struct {
	conn net.Conn
    id uId // immutable
    send chan *pb.Message
}

func newClient(conn net.Conn, id uId, uname string) *client {
    return &client{ 
        conn: conn, 
        send: make(chan *pb.Message, 4),

        id: id, 
        connTime: time.Now().Unix(),
        uname: uname,
    }
}

func newMaster(conn net.Conn, id uId) *master {
    return &master{ conn, id, make(chan *pb.Message, 4) }
}

