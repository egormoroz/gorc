package master

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
	"github.com/jedib0t/go-pretty/v6/table"
)

func (m *Master) cli() {
	var s string

loop:
	for {

		fmt.Scan(&s)

        switch s {
        case "q": break loop
        case "ls": m.getClients()
        case "shell": m.shell()
        }
	}
}

func (m *Master) getClients() {
    m.send <- &pb.Message {
        Kind: pb.MessageKind_CLIENTS_GET,
    }

    resp := <-m.cliResp
    if resp.Kind != pb.MessageKind_CLIENTS_RESP {
        fmt.Println("got unexpected response from server:", resp.Kind)
        return
    }

    t := table.NewWriter()
    t.SetOutputMirror(os.Stdout)
    t.AppendHeader(table.Row{"#", "addr", "uname", "online(s)"})

    for _, entry := range resp.GetClients().GetEntries() {
        t.AppendRow(table.Row{entry.Id, entry.Ip, entry.Uname, entry.OnlineDur})
    }

    t.Render()
}

func (m *Master) openShell() (uint64, bool) {
    var id uint64
    fmt.Scan(&id)

    m.send <- &pb.Message {
        Kind: pb.MessageKind_SHELL_OPEN,
        ClientId: &id,
    }

    resp := <-m.cliResp

    if resp.Kind == pb.MessageKind_CLIENT_NOT_FOUND {
        fmt.Println("client not found")
        return 0, false
    }

    if resp.Kind != pb.MessageKind_SHELL_OPEN_RES {
        fmt.Println("got unexpected response from server:", resp.Kind)
        return 0, false
    }

    if resp.GetRes() != pb.Result_OK {
        fmt.Println("failed to open a shell :-(", resp.Kind)
        return id, false
    }

    return id, true
}

func (m *Master) shell() {
    id, ok := m.openShell()
    if !ok { return }

    fmt.Println("1. \\q to exit")
    fmt.Println("2. \\dw <abs_remote_path> to download <abs_remote_path>")
    fmt.Println("3. \\up <local_path> to download <local_path>")
    reader := bufio.NewReader(os.Stdin)

    reader.ReadString('\n')

    for  {
        s, _ := reader.ReadString('\n')
        s = strings.TrimSpace(s)

        if s == "\\q" { break }
        if strings.HasPrefix(s, "\\dw ") { 
            m.download(id, strings.TrimPrefix(s, "\\dw "))
            continue 
        }
        if strings.HasPrefix(s, "\\up ") { 
            m.upload(id, strings.TrimPrefix(s, "\\up "))
            continue 
        }

        m.send <- &pb.Message {
            Kind: pb.MessageKind_SHELL_WRITE,
            ClientId: &id,
            Body: &pb.Message_TextContent{TextContent: s},
        }
    }

    fmt.Println("exited shell")

    m.send <- &pb.Message {
        Kind: pb.MessageKind_SHELL_CLOSE,
        ClientId: &id,
    }
}

func (m *Master) download(clientId uint64, s string) {
    id := m.counter
    m.counter++

    cb := func(*common.DWInstance) {
        fmt.Println("downloaded", s)
    }

    fmt.Println("download", s)
    m.downloader.NewDownload(
        m.ctx,
        id,
        fmt.Sprintf("%d.down", id), // TODO
        cb,
    )

    header := &pb.FTHeader {
        Path: s,
        Id: id,
    }

    // initiate upload on the client side
    m.send <- &pb.Message {
        Kind: pb.MessageKind_UP_START,
        ClientId: &clientId,
        Body: &pb.Message_Ftheader{ Ftheader: header },
    }
}

func (m* Master) upload(clientId uint64, s string) {
    id := m.counter
    m.counter++
    m.fcs.Add(id, clientId)

    cb := func(*common.UpInstance) {
        fmt.Println("uploaded", s)
    }

    fmt.Println("upload", s)
    m.uploader.NewUpload(
        m.ctx,
        id,
        s,
        m.fcs.C,
        cb,
    )

    header := &pb.FTHeader {
        Path: fmt.Sprintf("%d.up", id), // TODO
        Id: id,
    }

    // initiate download on the client side
    m.send <- &pb.Message {
        Kind: pb.MessageKind_DOWN_START,
        ClientId: &clientId,
        Body: &pb.Message_Ftheader{ Ftheader: header },
    }
}

