package common

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/egormoroz/gorc/internal/pb"
	"google.golang.org/protobuf/proto"
)


// TODO: use these
const (
    MaxMessageSize = 1 * 1024 * 1024
    RWTimeout = 30 * time.Second
)


func SendMessage(conn net.Conn, msg *pb.Message) error {
    data, err := proto.Marshal(msg)
    if err != nil { return err }

    var sizeBuf [4]byte
    binary.BigEndian.PutUint32(sizeBuf[:], uint32(len(data)))
    _, err = conn.Write(sizeBuf[:])
    if err != nil { return err }

    _, err = conn.Write(data)
    return err
}

func RecvMessage(conn net.Conn) (*pb.Message, error) {
    var sizeBuf [4]byte
    _, err := io.ReadFull(conn, sizeBuf[:])
    if err != nil { return nil, err }
    size := binary.BigEndian.Uint32(sizeBuf[:])

    dataBuf := make([]byte, size)
    _, err = io.ReadFull(conn, dataBuf)
    if err != nil { return nil, err }

    res := &pb.Message{}
    err = proto.Unmarshal(dataBuf, res)
    if err != nil { return nil, err }

    return res, nil
}

