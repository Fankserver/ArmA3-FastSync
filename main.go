package main

import (
	"A3FastSync/counter"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/goinggo/workpool"
	//"io"
	"io/ioutil"
	"log"
	//"net/http"
	//"os"
	"runtime"
	"time"
)

const (
	MAX_CONCURRENT_DOWNLOADS = 1
	DEV_TEST_URL             = "http://teamspeak.fankservercdn.com/test.txt"
)

type DownloadFile struct {
	Url      string `json:"url"`
	Checksum string `json:"check"`
}

type SynchFile struct {
	Version int            `json:"version"`
	Files   []DownloadFile `json:"files"`
}

type DownloadWork struct {
	Url      string
	Checksum string
	WP       *workpool.WorkPool
}

func (d *DownloadWork) DoWork(workRoutine int) {
	// check whether the file already exists
	if _, err := os.Stat(filename); err == nil {
		log.Printf("file already exists: %s checking checksum...")
		process(filename)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	workPool := workpool.New(runtime.NumCPU(), MAX_CONCURRENT_DOWNLOADS)
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
				Url:      sync.Files[k].Url,
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
