package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mitchellh/ioprogress"
)

var wg sync.WaitGroup

func main() {
	s := "https://localhost:8080/somefile"
	res, _ := http.Head(s) // 187 MB file of random numbers per line
	u, err := url.Parse(s)
	filename := u.Path[1:]
	fmt.Printf("Downloading %s from %s\n", filename, u.Host)
	if err != nil {
		panic(err)
	}
	maps := res.Header
	length, _ := strconv.Atoi(maps["Content-Length"][0]) // Get the content length from the header request
	workers := 10                                        // 10 Go-routines for the process so each downloads 18.7MB
	len_sub := length / workers                          // Bytes for each Go-routine
	diff := length % workers                             // Get the remaining for the last request
	// body := make([]string, workers)                      // Make up a temporary array to hold the data to be written to the file
	for i := 0; i < workers; i++ {
		wg.Add(1)

		min := len_sub * i       // Min range
		max := len_sub * (i + 1) // Max range

		if i == workers-1 {
			max += diff // Add the remaining bytes in the last request
		}

		go func(min int, max int, i int) {
			fmt.Printf("Downloading part %d...\n", i)
			client := &http.Client{
				Timeout: time.Duration(24 * time.Hour),
			}
			req, _ := http.NewRequest("GET", s, nil)
			range_header := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1) // Add the data for the Range header of the form "bytes=0-100"
			req.Header.Add("Range", range_header)
			resp, err := client.Do(req)
			if err != nil {
				log.Print(err)
			}
			defer resp.Body.Close()

			if i < 3 {
				f, err := os.Create(strconv.Itoa(i))
				if err != nil {
					log.Print(err)
				}
				defer f.Close()

				// Create the progress reader
				progressR := &ioprogress.Reader{
					Reader:   resp.Body,
					DrawFunc: ioprogress.DrawTerminalf(os.Stdout, ioprogress.DrawTextFormatBar(40)),
					Size:     int64(max - min),
				}

				// Copy all of the reader to some local file f. As it copies, the
				// progressR will write progress to the terminal on os.Stdout. This is
				// customizable.
				io.Copy(f, progressR)
			} else {
				reader, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Print(err)
				}
				ioutil.WriteFile(strconv.Itoa(i), []byte(string(reader)), 0x777) // Write to the file i as a byte array
			}

			wg.Done()
			fmt.Printf("...part %d done.\n", i)
		}(min, max, i)
	}
	wg.Wait()

	out, err := os.Create(filename)
	if err != nil {
		return
	}
	defer out.Close()
	for i := 0; i < workers; i++ {
		in, err := os.Open(strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}

		_, err = io.Copy(out, in)
		if err != nil {
			log.Fatal(err)
		}
		in.Close()
		os.Remove(strconv.Itoa(i))
	}
	fmt.Printf("\n\nDownloaded %s\n", filename)
	fmt.Println("This window will now close.\nI love you. :)")
	time.Sleep(3 * time.Second)
}
