package client

import (
	"fmt"
	"os"
	"os/user"
    "context"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)


type MessageHandler func(*Client, *pb.Message)

func (c *Client) handleMessage(msg *pb.Message) {
    if handler, ok := c.handlers[msg.Kind]; ok {
        handler(c, msg)
    } else {
        fmt.Fprintln(os.Stderr, "unexpected message kind:", msg.Kind)
    }
}


func (c *Client) registerHandlers() {
    c.handlers[pb.MessageKind_LOGIN_GET] = onLoginGet

    c.handlers[pb.MessageKind_SHELL_OPEN] = onShellOpen
    c.handlers[pb.MessageKind_SHELL_WRITE] = onShellWrite
    c.handlers[pb.MessageKind_SHELL_CLOSE] = onShellClose

    c.handlers[pb.MessageKind_DOWN_START] = onDownStart
    c.handlers[pb.MessageKind_UP_START] = onUpStart
    c.handlers[pb.MessageKind_FILE_CHUNK] = onFileChunk
}

func onLoginGet(c *Client, _ *pb.Message) {
    user, _ := user.Current()
    info := &pb.SysInfo{ Uname: user.Username }

    c.Send(&pb.Message{
        Kind: pb.MessageKind_LOGIN_RESP,
        Body: &pb.Message_Login { 
            Login: &pb.Login {
                Role: pb.Role_CLIENT,
                Info: info,
            },
        },
    })
}

func onShellOpen(c *Client, _ *pb.Message) {
    if c.shell != nil { c.shell.Close() }
    var err error
    var res pb.Result

    reader := func(shell *Shell) {
        for line := range shell.stdout {
            c.send <- &pb.Message {
                Kind: pb.MessageKind_SHELL_RESP,
                Body: &pb.Message_TextContent{ TextContent: line },
            }
        }
    }

    c.shell, err = NewShell()
    if err != nil {
        fmt.Fprintln(os.Stderr, "failed to open shell:", err)
        res = pb.Result_ERROR
    } else {
        res = pb.Result_OK
        go reader(c.shell)
        fmt.Println("open shell")
    }

    c.send <- &pb.Message {
        Kind: pb.MessageKind_SHELL_OPEN_RES,
        Body: &pb.Message_Res{ Res: res },
    }
}

func onShellWrite(c *Client, msg *pb.Message) { 
    if c.shell == nil {
        // TODO: return some sort of error
        return
    }
    s := msg.GetTextContent()
    fmt.Println("Shell write:", s)
    c.shell.stdin <- s
}

func onShellClose(c *Client, _ *pb.Message) { 
    fmt.Println("close shell")
    if c.shell != nil {
        c.shell.Close()
        c.shell = nil
    }
}

func onFileChunk(c *Client, msg *pb.Message) {
    body := msg.GetFileChunk()
    err := c.downloader.WriteChunk(common.Chunk{
        Id: body.Id,
        Data: body.Data, 
        Last: body.Last,
    })

    if err != nil {
        fmt.Println("Error on file download:", err)
    }
}

func onDownStart(c *Client, msg *pb.Message) {
    header := msg.GetFtheader()

    cb := func(*common.DWInstance) {
        fmt.Println("downloaded", header.Path)
    }

    fmt.Println("download", header.Path)
    c.downloader.NewDownload(
        context.TODO(),
        header.Id,
        header.Path,
        cb,
    )
}

func onUpStart(c *Client, msg *pb.Message) {
    header := msg.GetFtheader()

    cb := func(*common.UpInstance) {
        fmt.Println("uploaded", header.Path)
    }

    fmt.Println("upload", header.Path)
    c.uploader.NewUpload(
        context.TODO(),
        header.Id,
        header.Path,
        c.upChan,
        cb,
    )
}

