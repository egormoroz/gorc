package server

import (
	"fmt"
	"time"

	"github.com/egormoroz/gorc/internal/pb"
)

type CMsgHandler func(*Server, *client, *pb.Message)
type MMsgHandler func(*Server, *master, *pb.Message)


func (s *Server) registerHandlers() {
    s.mhandlers[pb.MessageKind_CLIENTS_GET] = onClientsGet

    fwdToMaster := []pb.MessageKind {
        pb.MessageKind_SHELL_OPEN_RES,
        pb.MessageKind_SHELL_RESP,

        pb.MessageKind_FILE_CHUNK,
    }

    fwdToClient := []pb.MessageKind {
        pb.MessageKind_SHELL_OPEN,
        pb.MessageKind_SHELL_WRITE,
        pb.MessageKind_SHELL_CLOSE,

        pb.MessageKind_DOWN_START,
        pb.MessageKind_UP_START,
        pb.MessageKind_FILE_CHUNK,
    }

    for _, kind := range fwdToMaster {
        s.chandlers[kind] = forwardToMaster
    }

    for _, kind := range fwdToClient {
        s.mhandlers[kind] = forwardToClient
    }
}


func forwardToMaster(s *Server, c *client, msg *pb.Message) {
    m := s.mast.Load()
    if m == nil { return }

    id := uint64(c.id)
    msg.ClientId = &id
    m.send <- msg
}


func forwardToClient(s *Server, m *master, msg *pb.Message) {
    cId := msg.GetClientId()
    s.clientsMtx.RLock()
    c, ok := s.clients[uId(cId)]
    s.clientsMtx.RUnlock()

    if ok {
        c.send <- msg
    } else {
        fmt.Println("client id not found", cId, msg.ClientId)
        m.send <- &pb.Message {
            Kind: pb.MessageKind_CLIENT_NOT_FOUND,
            ClientId: &cId,
        }
    }
}

func onClientsGet(s *Server, m *master, msg *pb.Message) {
    s.clientsMtx.RLock()
    defer s.clientsMtx.RUnlock()

    entries := make([]*pb.ClientList_Entry, 0, len(s.clients))
    for id, c := range s.clients {
        entries = append(entries, &pb.ClientList_Entry{
            Id: uint64(id), 
            Ip: c.conn.RemoteAddr().String(),
            Uname: c.uname,
            OnlineDur: time.Now().Unix() - c.connTime,
        })
    }

    m.send <- &pb.Message{
        Kind: pb.MessageKind_CLIENTS_RESP,
        Body: &pb.Message_Clients {
            Clients: &pb.ClientList { Entries: entries },
        },
    }
}

