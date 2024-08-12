package master

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/egormoroz/gorc/internal/common"
	"github.com/egormoroz/gorc/internal/pb"
)

type FChunkSender struct {
	t2c map[uint64]uint64
	mu  sync.RWMutex

	C chan<- common.Chunk
}

func NewFChunkSender(ctx context.Context, send chan<- *pb.Message) *FChunkSender {
	c := make(chan common.Chunk, 16)
	fcs := &FChunkSender{
		t2c: make(map[uint64]uint64),
		C:   c,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk := <-c:
				cid, found := fcs.find(chunk.Id)
				if !found {
					log.Println("tried to forward a nonexistant file upload...")
					continue
				}

				ch := &pb.FileChunk{
					Data: chunk.Data,
					Last: chunk.Last,
				}
				send <- &pb.Message{
					Kind:     pb.MessageKind_FILE_CHUNK,
					ClientId: &cid,
					Body:     &pb.Message_FileChunk{FileChunk: ch},
				}

				if chunk.Last {
					fcs.mu.Lock()
					delete(fcs.t2c, chunk.Id)
					fcs.mu.Unlock()
				}
			}
		}
	}()

	return fcs
}

func (fcs *FChunkSender) find(tid uint64) (uint64, bool) {
	fcs.mu.RLock()
	defer fcs.mu.RUnlock()

	if cid, exists := fcs.t2c[tid]; exists {
		return cid, true
	}

	return 0, false
}

func (fcs *FChunkSender) Add(tid, cid uint64) error {
	fcs.mu.Lock()
	defer fcs.mu.Unlock()

	if _, exists := fcs.t2c[tid]; exists {
		return errors.New("transaction already registered")
	}

	fcs.t2c[tid] = cid
	return nil
}
