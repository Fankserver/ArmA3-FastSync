package main

import (
	"A3FastSync/counter"
	"encoding/json"
	"flag"
	"fmt"
	//"github.com/goinggo/workpool"
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

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	//workPool := workpool.New(runtime.NumCPU(), MAX_CONCURRENT_DOWNLOADS)
	bps := new(counter.Counter)

	// parse flags
	syncpath := flag.String("sync", "default.a3sync", "A3FastSync synchronization file")
	flag.Parse()
	*syncpath = fmt.Sprintf("sync/%s", *syncpath)

	// process input file
	file, e := ioutil.ReadFile(*syncpath)
	if e != nil {
		log.Fatalf("config error: %v\n", e)
		return
	}

	var sync SynchFile
	err := json.Unmarshal(file, &sync)
	if err != nil {
		log.Fatalf("sync file parse error (%s): %s", *syncpath, err)
		return
	}

	/*go func(c *counter.Counter) {
		for {

		}
	}(bps)*/

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
