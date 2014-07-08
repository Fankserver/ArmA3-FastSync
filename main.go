package main

import (
	"A3FastSync/counter"
	"A3FastSync/downloader"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/goinggo/workpool"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
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
	Url      string
	Path     string
	Checksum string
	WP       *workpool.WorkPool
	CTR      *counter.Counter
	Client   *http.Client
	Config   *Config
}

type Config struct {
	ArmaPath string `json:"armapath"`
}

func (d *DownloadWork) DoWork(workRoutine int) {
	for {
		// build path
		path := filepath.Clean(d.Path)
		path = filepath.Join(d.Config.ArmaPath, path)

		log.Printf("%d computed path: %s", workRoutine, path)

		// check whether the file already exists
		download := true
		if _, err := os.Stat(path); err == nil {
			log.Printf("%d file already exists - verifying checksum...", workRoutine)

			// calculate SHA1 checksum
			f, err := os.Open(path)
			defer f.Close()
			if err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}

			hasher := sha1.New()
			if _, err := io.Copy(hasher, f); err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}
			hash := fmt.Sprintf("%x", hasher.Sum(nil))
			log.Printf("%d computed checksum: %s", workRoutine, hash)
			if hash == d.Checksum {
				log.Printf("%d valid file exists: skipping", workRoutine)
				download = false
			} else {
				log.Printf("%d checksum mismatch: downloading file", workRoutine)
			}
		} else {
			if os.IsNotExist(err) {
				// file does not exist
			} else {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}
		}

		// create the directory if needed
		err := os.MkdirAll(filepath.Dir(path), 0775)
		if err != nil {
			log.Printf("%d ERROR: %s", workRoutine, err)
			return
		}

		// download the file
		if download {

			// assemble http request
			dlUrl := d.Url + filepath.Clean(d.Path)
			log.Printf("%d dl url: %s", workRoutine, dlUrl)

			req, err := http.NewRequest("GET", dlUrl, nil)
			if err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}

			// resume partial download if we already have some bytes
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0775)
			defer f.Close()
			if err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}

			info, err := f.Stat()
			if err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}

			req.Header.Add("Range", fmt.Sprintf("bytes=%d-", info.Size()))
			log.Printf("%d resuming download at %d byte", workRoutine, info.Size())

			// do request
			resp, err := d.Client.Do(req)
			defer resp.Body.Close()

			// check response
			if err != nil {
				log.Printf("%d ERROR: %v (%s)", workRoutine, err, resp.Status)
				return
			}

			switch resp.StatusCode {
			case http.StatusOK:
			case http.StatusPartialContent:
			case http.StatusRequestedRangeNotSatisfiable:
			default:
				log.Printf("%d ERROR: %v (%s)", workRoutine, err, resp.Status)
				return
			}

			// reset file if content is too long (file corrupted)
			if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
				log.Printf("%d file corrupted - downloading again", workRoutine)
				err = os.Remove(path)
				if err != nil {
					log.Printf("%d ERROR: %s", workRoutine, err)
					return
				}
				continue
			}

			body := &downloader.Downloader{Reader: resp.Body, Counter: d.CTR}

			// write to file
			log.Printf("%d content length: %d", workRoutine, resp.ContentLength)
			n, err := io.Copy(f, body)
			log.Printf("%d successfully read %d bytes", workRoutine, n)

			// calculate SHA1 checksum
			f, err = os.Open(path)
			defer f.Close()
			if err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}

			hasher := sha1.New()
			if _, err := io.Copy(hasher, f); err != nil {
				log.Printf("%d ERROR: %s", workRoutine, err)
				return
			}
			hash := fmt.Sprintf("%x", hasher.Sum(nil))
			log.Printf("%d computed checksum: %s", workRoutine, hash)
			if hash != d.Checksum {
				log.Printf("%d file corrupted - downloading again", workRoutine)
				err = os.Remove(path)
				if err != nil {
					log.Printf("%d ERROR: %s", workRoutine, err)
					return
				}
				continue
			} else {
				log.Printf("%d download successful: valid checksum", workRoutine)
			}
		}
		return
	}
}

func main() {
	fmt.Println("# A3FastSync v.0.2 ALPHA")
	fmt.Println("# (c) 2014 nano2k - http://sync.nano.sx\n")
	runtime.GOMAXPROCS(runtime.NumCPU())

	// parse flags
	syncpath := flag.String("sync", "", "Path to a valid synchronization file (*.a3sync)")
	index_path := flag.String("index", "", "(only for admins) Path to a directory which should be indexed. Creates a *.a3sync file.")
	output_path := flag.String("o", "", "(only for admins) Path to a directory where the *.a3sync file should be created.")
	max_concurrent_downloads := flag.Int("n", 2, "Concurrent download limit")
	configpath := flag.String("config", "config/config.json", "json config file")

	flag.Parse()

	// json parse
	file, e := ioutil.ReadFile(*configpath)
	if e != nil {
		log.Fatalf("ERROR: %v\n", e)
		return
	}

	var config Config
	err := json.Unmarshal(file, &config)
	if err != nil {
		log.Fatalf("ERROR: (%s) %s", *configpath, err)
		return
	}

	workPool := workpool.New(runtime.NumCPU()*3, int32(*max_concurrent_downloads))
	bps := new(counter.Counter)
	client := &http.Client{}

	if *index_path != "" {
		if *output_path != "" {

		} else {
			log.Println("missing -o parameter")
		}
	} else if *syncpath != "" {

		//*syncpath = fmt.Sprintf("sync/%s", *syncpath)

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
					Url:      sync.Server,
					Path:     sync.Files[k].Path,
					Checksum: sync.Files[k].Checksum,
					WP:       workPool,
					CTR:      bps,
					Client:   client,
					Config:   &config,
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
				if workPool.ActiveRoutines() < 1 {
					log.Printf("no jobs available - finished")
					return
				}
				currentBps := bps.Value()
				bps.Reset()
				fmt.Printf("Kbyte/s: %.2f\n", float32(currentBps)/float32(1024))
			}
		}
	} else {
		flag.Usage()
	}
}
