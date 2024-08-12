package master

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)

type Master struct {
    conn net.Conn
    send chan *pb.Message

    ctx context.Context

    counter uint64
    downloader *common.FileDownloader
    uploader *common.FileUploader

    fcs *FChunkSender

    // This must be immutable during runtime
    handlers map[pb.MessageKind]MessageHandler

    cliResp chan *pb.Message
}

func Run(addr string) error {
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        fmt.Fprintln(os.Stderr, "Failed to dial:", err)
        return err
    }

    ctx, cancel := context.WithCancel(context.Background())

    send := make(chan *pb.Message, 4)

    m := Master { 
        conn: conn, 
        send: send,
        handlers: make(map[pb.MessageKind]MessageHandler),
        cliResp: make(chan *pb.Message), // blocking

        downloader: common.NewFileDownloader(16, time.Second * 5),
        uploader: common.NewFileUploader(16),

        ctx: ctx,

        fcs: NewFChunkSender(ctx, send),
    }

    m.registerHandlers()

    stopped := make(chan struct{}, 2)
    go func() {
        for msg := range m.send {
            err := common.SendMessage(m.conn, msg)
            if err != nil { break }
        }

        stopped <- struct{}{}
    }()

    go func() { 
        for {
            msg, err := common.RecvMessage(m.conn)
            if err != nil { 
                break
            }
            m.handleMessage(msg)
        }

        stopped <- struct{}{}
    }()

    m.cli()

    cancel()

    m.conn.Close()
    close(m.send)
    <-stopped
    <-stopped

    return nil
}
