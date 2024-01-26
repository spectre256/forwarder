package main

import (
    "log"
    "fmt"
    "net/http"
    "strings"
    "bufio"
    "bytes"
    "sync"
)

func main() {
	// TODO: Replace with serial reader
	raw := `GET /v1/current.json?key=APIKEY&q=ZIPCODE HTTP/1.1
Host: api.weatherapi.com
User-Agent: Windows NT 10.0; +https://github.com/spectre256/forwarder Forwarder/0.0.1
Accept: application/json

` + "\x00" + `GET /v1/current.json?key=APIKEY&q=ZIPCODE&aqi=yes HTTP/1.1
Host: api.weatherapi.com
User-Agent: Windows NT 10.0; +https://github.com/spectre256/forwarder Forwarder/0.0.1
Accept: application/json

`

	input := strings.NewReader(raw)

	scanner := bufio.NewScanner(input)
	scanner.Split(ScanByteSep('\x00'))

	client := new(http.Client)
	var wg sync.WaitGroup

	for scanner.Scan() {
		fmt.Println(scanner.Text())
		wg.Add(1)
		go func(text string){
			req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(text)))
			if err != nil {
				log.Println(err)
				return
			}
			req.RequestURI, req.URL.Scheme, req.URL.Host = "", "https", req.Host

			resp, err := client.Do(req)
			if err != nil {
				log.Println(err)
				return
			}

			// TODO: Replace with serial writer
			output := new(strings.Builder)
			resp.Write(output)
			fmt.Println(output.String())

			wg.Done()
		}(scanner.Text())
	}

	wg.Wait()
}

func ScanByteSep(sep byte) bufio.SplitFunc {
	// Modified version of the ScanLines function
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.6:src/bufio/scan.go;l=350
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, sep); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}
