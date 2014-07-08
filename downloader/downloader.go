package downloader

import (
	"A3FastSync/counter"
	"io"
)

type Downloader struct {
	io.Reader
	total   int64
	Counter *counter.Counter
}

func (dl *Downloader) Read(p []byte) (int, error) {
	n, err := dl.Reader.Read(p)
	if err == nil || err == io.EOF {
		dl.total += int64(n)
		dl.Counter.Add(int64(n))
	}

	return n, err
}
