package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"log"
	"os"
	"runtime"
	"sync"
)

var flagHash = flag.String("hash", "sha256", "hash function to use (crc32, md5, sha1, sha224, sha256, sha512)")
var flagJobs = flag.Int("jobs", 0, "number of hash jobs to run (default 1 per CPU core)")
var flagFmt = flag.String("fmt", "text", "output format (options: hex, base64, json, json-base64)")

type hashTask struct {
	path string
	fs   fs.FS
}

type hashResult struct {
	path string
	hash []byte
}

type hashFactory func() hash.Hash

type hashPrinter interface {
	Print(hashResult)
}

type jsonResult struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

func hashByName(name string) hashFactory {
	switch name {
	case "crc32":
		return func() hash.Hash { return crc32.New(crc32.IEEETable) }
	case "md5":
		return md5.New
	case "sha1":
		return sha1.New
	case "sha224":
		return sha256.New224
	case "sha256":
		return sha256.New
	case "sha512":
		return sha512.New
	default:
		log.Fatal("hash function not supported")
		return nil
	}
}

func hasher(hf hashFactory, tasks <-chan hashTask, results chan<- hashResult) {
	buf := make([]byte, 1024*1024)

	for task := range tasks {
		f, err := task.fs.Open(task.path)
		if err != nil {
			log.Fatal(err)
		}

		h := hf()
		io.CopyBuffer(h, f, buf)
		results <- hashResult{task.path, h.Sum(nil)}

		f.Close()
	}
}

// hexHashPrinter prints hashes in the classic "hexhash <spc><spc> filename" format.
type hexHashPrinter struct{}

func (hp hexHashPrinter) Print(r hashResult) {
	fmt.Printf("%s  %s\n", hex.EncodeToString(r.hash), r.path)
}

// base64HashPrinter prints hashes in "base64hash <spc><spc> filename" format, using standard Base64 with padding
type base64HashPrinter struct{}

func (hp base64HashPrinter) Print(r hashResult) {
	fmt.Printf("%s  %s\n", base64.StdEncoding.EncodeToString(r.hash), r.path)
}

// jsonBase64HashPrinter prints hashes as JSON lines with keys "hash" and
// "path", with "hash" containing a hex hash in the same format as
// hexHashPrinter.
type jsonHexHashPrinter struct {
	enc *json.Encoder
}

func (hp jsonHexHashPrinter) Print(r hashResult) {
	hp.enc.Encode(jsonResult{r.path, hex.EncodeToString(r.hash)})
}

// jsonBase64HashPrinter prints hashes as JSON lines with keys "hash" and
// "path", with "hash" containing a Base64 hash in the same format as
// base64HashPrinter.
type jsonBase64HashPrinter struct {
	enc *json.Encoder
}

func (hp jsonBase64HashPrinter) Print(r hashResult) {
	hp.enc.Encode(jsonResult{r.path, base64.StdEncoding.EncodeToString(r.hash)})
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s [opts] <paths...>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	jobs := *flagJobs
	if jobs == 0 {
		jobs = runtime.NumCPU()
	}

	// Set up task queues
	tasks := make(chan hashTask, jobs*2)
	results := make(chan hashResult, jobs*2)

	// Get hash function
	hb := hashByName(*flagHash)

	// Launch workers
	var wgHasher sync.WaitGroup
	for i := 0; i < jobs; i++ {
		go func() {
			defer wgHasher.Done()
			hasher(hb, tasks, results)
		}()
	}
	wgHasher.Add(jobs)

	// Initialize and launch the hash printer
	var hp hashPrinter
	switch *flagFmt {
	case "hex":
		hp = &hexHashPrinter{}
	case "base64":
		hp = &base64HashPrinter{}
	case "json", "json-hex":
		hp = &jsonHexHashPrinter{json.NewEncoder(os.Stdout)}
	case "json-base64":
		hp = &jsonBase64HashPrinter{json.NewEncoder(os.Stdout)}
	}

	var wgPrinter sync.WaitGroup
	go func() {
		defer wgPrinter.Done()
		for r := range results {
			hp.Print(r)
		}
	}()
	wgPrinter.Add(1)

	// Start walking the filesystem and generating paths
	for _, rootPath := range flag.Args() {
		dir := os.DirFS(rootPath)
		fs.WalkDir(dir, ".", func(p string, dirent fs.DirEntry, err error) error {
			if err != nil {
				log.Fatal(err)
			}
			if dirent.IsDir() {
				return nil
			}
			tasks <- hashTask{p, dir}
			return nil
		})
	}

	// Wait for all workers to exit
	close(tasks)
	wgHasher.Wait()
	close(results)
	wgPrinter.Wait()
}
