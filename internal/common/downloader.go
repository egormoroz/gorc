package common

import (
	"bufio"
	"context"
	"errors"
	"os"
	"sync"
    "log"
    "time"
)

type DWDoneCallback func(*DWInstance)

type Chunk struct {
    Id uint64
    Data []byte
    Last bool
}

type DWInstance struct {
    id uint64
    chunkStream chan Chunk
    cb DWDoneCallback

    ctx context.Context
    cancel context.CancelFunc

    writer *bufio.Writer
    file *os.File
}

type FileDownloader struct {
	instances map[uint64]*DWInstance
    mu sync.RWMutex
    semaphore chan struct{}
    timeout time.Duration
}

func NewFileDownloader(maxConcurrent int, chunkTimeout time.Duration) *FileDownloader {
    return &FileDownloader{
        instances: make(map[uint64]*DWInstance),
        semaphore: make(chan struct{}, maxConcurrent),
        timeout: chunkTimeout,
    }
}

func (fd *FileDownloader) NewDownload(
    ctx context.Context, 
    id uint64,
    dst string, 
    cb DWDoneCallback,
) (*DWInstance, error) {

    select {
    case fd.semaphore <- struct{}{}:
    case <- ctx.Done(): return nil, ctx.Err()
    }

    fd.mu.Lock()
    defer fd.mu.Unlock()

    if _, exists := fd.instances[id]; exists {
        <-fd.semaphore
        return nil, errors.New("id already in use")
    }

    file, err := os.Create(dst)
    if err != nil { 
        <-fd.semaphore
        return nil, err 
    }

    ctx, cancel := context.WithCancel(ctx)
    instance := &DWInstance{
        id: id,
        chunkStream: make(chan Chunk, 16),
        cb: cb,
        ctx: ctx,
        cancel: cancel,

        file: file,
        writer: bufio.NewWriter(file),
    }

    fd.instances[id] = instance
    go fd.handleDownload(instance)

    return instance, nil
}

func (fd *FileDownloader) handleDownload(instance *DWInstance) {
    defer func() {
        instance.writer.Flush()
        instance.file.Close()

        fd.mu.Lock()
        delete(fd.instances, instance.id)
        fd.mu.Unlock()

        <-fd.semaphore
    }()

loop:
    for {
        select {
        case chunk := <-instance.chunkStream:
            _, err := instance.writer.Write(chunk.Data)
            if err != nil { 
                log.Println("file download error:", err)
                instance.cancel()
                return
            }
            if chunk.Last { break loop }

        case <-time.After(fd.timeout):
            log.Println("chunk timeout: no data received for", fd.timeout)
            instance.cancel()
            return

        case <-instance.ctx.Done(): return
        }
    }

    if instance.cb != nil {
        instance.cb(instance)
    }
}

func (fd *FileDownloader) WriteChunk(chunk Chunk) error {
    fd.mu.RLock()
    instance, ok := fd.instances[chunk.Id]
    fd.mu.RUnlock()

    if !ok { return errors.New("instance not found") }

    select {
    case instance.chunkStream <-chunk: return nil
    case <-instance.ctx.Done(): return instance.ctx.Err()
    }
}

