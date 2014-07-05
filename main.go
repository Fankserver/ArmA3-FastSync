package main

import (
	"A3FastSync/counter"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/goinggo/workpool"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	MAX_CONCURRENT_DOWNLOADS = 1
	DEV_TEST_URL             = "http://teamspeak.fankservercdn.com/test.txt"
	DL_ROOT                  = "/home/nano/go/src/A3FastSync/dl"
)

type DownloadFile struct {
	Path     string `json:"path"`
	Checksum string `json:"check"`
}

type SynchFile struct {
	Version int            `json:"version"`
	Server  string         `json:"server"`
	Files   []DownloadFile `json:"files"`
}

type DownloadWork struct {
	Path     string
	Checksum string
	WP       *workpool.WorkPool
}

func (d *DownloadWork) DoWork(workRoutine int) {
	// build path
	path := filepath.Clean(d.Path)
	if !filepath.IsAbs(path) {
		log.Printf("ERROR: relative paths are not allowed for security reasons (%s)", filepath.Join(DL_ROOT, path))
		return
	}
	path = filepath.Join(DL_ROOT, path)

	log.Printf("computed path: %s", path)

	// check whether the file already exists
	if _, err := os.Stat(path); err == nil {
		log.Printf("file already exists - checking checksum...")

		// calculate SHA1 checksum
		f, err := os.Open(path)
		if err != nil {
			log.Printf("ERROR: %s", err)
			return
		}
		defer f.Close()

		hasher := sha1.New()
		if _, err := io.Copy(hasher, f); err != nil {
			log.Printf("ERROR: %s", err)
			return
		}
		hash := fmt.Sprintf("%x", hasher.Sum(nil))
		log.Printf("computed checksum: %s", hash)
		if hash == d.Checksum {
			log.Printf("correct file does exist - skipping")
		}

	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	workPool := workpool.New(runtime.NumCPU()*3, MAX_CONCURRENT_DOWNLOADS)
	bps := new(counter.Counter)

	// parse flags
	syncpath := flag.String("sync", "default.a3sync", "A3FastSync synchronization file")
	flag.Parse()
	*syncpath = fmt.Sprintf("sync/%s", *syncpath)

	// process input file
	file, e := ioutil.ReadFile(*syncpath)
	if e != nil {
		log.Fatalf("%v\n", e)
		return
	}

	var sync SynchFile
	err := json.Unmarshal(file, &sync)
	if err != nil {
		log.Fatalf("sync file parse error (%s): %s", *syncpath, err)
		return
	}

	filesPending := len(sync.Files)
	log.Printf("Files: %d", filesPending)

	go func() {
		for k := range sync.Files {
			work := &DownloadWork{
				Path:     sync.Files[k].Path,
				Checksum: sync.Files[k].Checksum,
				WP:       workPool,
			}

			err := workPool.PostWork("work_queue_routine", work)

			if err != nil {
				fmt.Printf("ERROR: %s\n", err)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	for {
		select {
		// measure download speed
		case <-time.After(time.Second):
			currentBps := bps.Value()
			bps.Reset()
			fmt.Printf("byte/s: %d\n", currentBps)
		}
	}
}
