package util

import (
	"fmt"
	"io"
	"os"
)

// BlockingBuffer is a type of buffer to which data can be written while it is being read.
//
// Reads will block when the current eof is reached until more data is
// available or writing is finished with a call to Close.
type BlockingBuffer struct {
	filename string
	file     *os.File

	events Emitter
}

// NewBlockingBuffer creates a ne BlockingBuffer.
func NewBlockingBuffer() (*BlockingBuffer, error) {
	filename := TempName("bbuf")
	fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0600)
	if err != nil {
		return nil, err
	}
	bbuf := &BlockingBuffer{
		filename: filename,
		file:     fd,
	}
	return bbuf, nil
}

func (bbuf *BlockingBuffer) closed() bool {
	return bbuf.file == nil
}

func (bbuf *BlockingBuffer) Write(p []byte) (int, error) {
	if bbuf.closed() {
		return 0, fmt.Errorf("BBuf is closed")
	}
	n, err := bbuf.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("BBuf write: %v", err)
	}
	bbuf.events.Emit("write")
	return n, nil
}

// Close closes the buffer for writing.
func (bbuf *BlockingBuffer) Close() error {
	if bbuf.closed() {
		return fmt.Errorf("BBuf: duplicate call to Close")
	}
	err := bbuf.file.Close()
	bbuf.file = nil
	bbuf.events.Emit("write")
	return err
}

// Destroy closes the buffer for reading and writing and frees the underlying storage.
func (bbuf *BlockingBuffer) Destroy() error {
	if !bbuf.closed() {
		bbuf.Close()
	}
	return os.Remove(bbuf.filename)
}

// Reader returns a reader to read data from this buffer.
func (bbuf *BlockingBuffer) Reader() io.ReadCloser {
	return &blockingBufferReader{
		bbuf:     bbuf,
		listener: bbuf.events.Listen(),
	}
}

type blockingBufferReader struct {
	bbuf     *BlockingBuffer
	listener <-chan string

	file   *os.File
	offset int64
}

func (bbufr *blockingBufferReader) Read(p []byte) (int, error) {
	if bbufr.file == nil {
		fd, err := os.OpenFile(bbufr.bbuf.filename, os.O_RDONLY|os.O_SYNC, 0600)
		if err != nil {
			return 0, fmt.Errorf("BBuf read open: %v", err)
		}
		if _, err := fd.Seek(bbufr.offset, 0); err != nil {
			return 0, fmt.Errorf("BBuf read seek: %v", err)
		}
		bbufr.file = fd
	}

	n, err := bbufr.file.Read(p)
	bbufr.offset += int64(n)
	if err == io.EOF {
		bbufr.file.Close()
		bbufr.file = nil
		if bbufr.bbuf.closed() {
			return n, io.EOF
		}
		<-bbufr.listener
		return n, nil
	} else if err != nil {
		return n, fmt.Errorf("BBuf read: %v", err)
	}
	return n, nil
}

func (bbufr *blockingBufferReader) Close() error {
	bbufr.bbuf.events.Unlisten(bbufr.listener)
	if bbufr.file != nil {
		return bbufr.file.Close()
	}
	return nil
}
