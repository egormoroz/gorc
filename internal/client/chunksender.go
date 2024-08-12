package client

import (
	"context"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)

func StartChunkSender(ctx context.Context, send chan<- *pb.Message) chan<- common.Chunk {
    c := make(chan common.Chunk, 16)

    go func() {
        for {
            select {
            case <-ctx.Done(): return
            case chunk := <-c:
                ch := &pb.FileChunk {
                    Data: chunk.Data,
                    Last: chunk.Last,
                }

                send <- &pb.Message{
                    Kind: pb.MessageKind_FILE_CHUNK,
                    Body: &pb.Message_FileChunk{ FileChunk: ch },
                }
            }
        }
    }()

    return c
}

