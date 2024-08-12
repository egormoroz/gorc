package common

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"sync"
)

const FileChunkSize = min(4096, MaxMessageSize / 2)

type UpDoneCallback func(*UpInstance)

type UpInstance struct {
	id uint64
    writeChan chan<- Chunk
    cb UpDoneCallback

    ctx context.Context
    cancel context.CancelFunc

    reader *bufio.Reader
    file *os.File
}

type FileUploader struct {
	instances map[uint64]*UpInstance
    mu sync.RWMutex
    semaphore chan struct{}
}


func NewFileUploader(maxConcurrent int) *FileUploader {
    return &FileUploader{
        instances: make(map[uint64]*UpInstance),
        semaphore: make(chan struct{}, maxConcurrent),
    }
}

func (fu *FileUploader) NewUpload(
    ctx context.Context, 
    id uint64,
    src string, 
    writeChan chan<- Chunk,
    cb UpDoneCallback,
) (*UpInstance, error) {
    select {
    case fu.semaphore <- struct{}{}:
    case <- ctx.Done(): return nil, ctx.Err()
    }

    fu.mu.Lock()
    defer fu.mu.Unlock()

    if _, exists := fu.instances[id]; exists {
        <-fu.semaphore
        return nil, errors.New("id already in use")
    }

    file, err := os.Open(src)
    if err != nil { 
        <-fu.semaphore
        return nil, err 
    }

    ctx, cancel := context.WithCancel(ctx)
    instance := &UpInstance{
        id: id,
        writeChan: writeChan,
        cb: cb,
        ctx: ctx,
        cancel: cancel,

        file: file,
        reader: bufio.NewReader(file),
    }

    fu.instances[id] = instance
    go fu.handleUpload(instance)

    return instance, nil
}

func (fu *FileUploader) handleUpload(instance *UpInstance) {
    defer func() {
        instance.file.Close()

        fu.mu.Lock()
        delete(fu.instances, instance.id)
        fu.mu.Unlock()

        <-fu.semaphore
    }()

    buf := make([]byte, FileChunkSize)

loop:
    for {
        select {
        case <-instance.ctx.Done(): return
        default:
            n, err := instance.reader.Read(buf)
            last := false
            if err != nil {
                switch err {
                case io.EOF: last = true
                default:
                    log.Println("file upload error:", err)
                    instance.cancel()
                    return
                }
            }

            instance.writeChan <- Chunk{Data: buf[:n], Last: last, Id: instance.id}
            if last { break loop }
        }
    }

    if instance.cb != nil {
        instance.cb(instance)
    }
}
