package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)

type Client struct {
    conn net.Conn
    send chan *pb.Message

    // This must be immutable during runtime
    handlers map[pb.MessageKind]MessageHandler

    shell *Shell

    downloader *common.FileDownloader
    uploader *common.FileUploader

    upChan chan<- common.Chunk
}

func (c *Client) Send(msg *pb.Message) {
    c.send <- msg
}

func (c *Client) Close() {
    close(c.send)
    c.conn.Close()
}

func Run(addr string) error {
    var err error

    conn, err := net.Dial("tcp", addr)
    if err != nil {
        fmt.Fprintln(os.Stderr, "Failed to dial:", err)
        return err
    }

    ctx, cancel := context.WithCancel(context.Background())
    send := make(chan *pb.Message, 16)

    c := Client { 
        conn: conn, 
        send: send,
        handlers: make(map[pb.MessageKind]MessageHandler),

        downloader: common.NewFileDownloader(16, time.Second * 5),
        uploader: common.NewFileUploader(16),

        upChan: StartChunkSender(ctx, send),
    }
    c.registerHandlers()

    go func() {
        for msg := range c.send {
            err := common.SendMessage(c.conn, msg)
            if err != nil { 
                c.Close() 
                break
            }
        }
    }()

    for {
        msg, err := common.RecvMessage(c.conn)
        if err != nil { 
            c.Close() 
            break
        }
        c.handleMessage(msg)
    }

    cancel()
    return nil
}
