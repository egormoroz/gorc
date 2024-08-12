package master

import (
	"fmt"
	"os"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)


type MessageHandler func(*Master, *pb.Message)

func (c *Master) handleMessage(msg *pb.Message) {
    if handler, ok := c.handlers[msg.Kind]; ok {
        handler(c, msg)
    } else {
        fmt.Fprintln(os.Stderr, "unexpected message kind:", msg.Kind)
    }
}


func (c *Master) registerHandlers() {
    c.handlers[pb.MessageKind_LOGIN_GET] = onLoginGet

    c.handlers[pb.MessageKind_CLIENTS_RESP] = forwardToCli
    c.handlers[pb.MessageKind_SHELL_OPEN_RES] = forwardToCli
    c.handlers[pb.MessageKind_SHELL_RESP] = onShellResp

    c.handlers[pb.MessageKind_FILE_CHUNK] = onFileChunk
}

func onLoginGet(m *Master, _ *pb.Message) {
    m.send <- &pb.Message{
        Kind: pb.MessageKind_LOGIN_RESP,
        Body: &pb.Message_Login {
            Login: &pb.Login{ Role: pb.Role_MASTER },
        },
    }
}

func forwardToCli(m *Master, msg *pb.Message) {
    m.cliResp <- msg
}

func onShellResp(_ *Master, msg *pb.Message) {
    fmt.Println(msg.GetTextContent())
}

func onFileChunk(m *Master, msg *pb.Message) {
    body := msg.GetFileChunk()
    err := m.downloader.WriteChunk(common.Chunk{
        Id: body.Id,
        Data: body.Data, 
        Last: body.Last,
    })

    if err != nil {
        fmt.Println("Error on file download:", err)
    }
}

