package main

import (
    "bufio"
    "bytes"
    "flag"
	"fmt"
    "io"
    "log"
    "net/http"
	"os"
    "strings"
    "sync"

    "github.com/tarm/serial"
)

type Stdio struct {
	in io.Reader
	out io.Writer
}

func (stdio Stdio) Read(p []byte) (int, error) {
	return stdio.in.Read(p)
}

func (stdio Stdio) Write(p []byte) (int, error) {
	return stdio.out.Write(p)
}

func main() {
	// CLI options
	const usage string = `Usage: forwarder ([--stdin] | [--baud BAUD] [--dev DEV]) [--verbose] [--help]
Options:
    -b, --baud BAUD    Set the baud rate of the serial connection. Defaults to 115600
    -d, --dev DEV      Set the name of the device to connect to. Defaults to "COM4"
    -s, --stdin        Use stdin instead of serial for debugging
    -v, --verbose      Enable verbose output for debugging
    -h, --help         Show this help message
`
	var baud int
	flag.IntVar(&baud, "baud", 115600, "")
	flag.IntVar(&baud, "b", 115600, "")
	var name string
	flag.StringVar(&name, "dev", "COM4", "")
	flag.StringVar(&name, "d", "COM4", "")
	var stdin bool
	flag.BoolVar(&stdin, "stdin", false, "")
	flag.BoolVar(&stdin, "s", false, "")
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "")
	flag.BoolVar(&verbose, "v", false, "")
	var help bool
	flag.BoolVar(&help, "help", false, "")
	flag.BoolVar(&help, "h", false, "")

	flag.Usage = func() { fmt.Print(usage) }
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}

	// Input-output stream
	var conn io.ReadWriter
	if stdin {
		conn = Stdio{
			in: os.Stdin,
			out: os.Stdout,
		}

		if verbose {
			log.Println("Successfully configured stdin and stdout stream")
		}
	} else {
		config := &serial.Config{
			Name: name,
			Baud: baud,
		}

		var err error
		conn, err = serial.OpenPort(config)
		if err != nil {
			log.Fatalln("Error configuring serial:", err)
		} else if verbose {
			log.Println("Successfully configured serial stream")
		}
	}

	// Read from stream items separated by NULL character
	scanner := bufio.NewScanner(conn)
	scanner.Split(ScanByteSep('\x00'))

	client := new(http.Client)
	var wg sync.WaitGroup
	var m sync.Mutex

	// Spawn goroutine for each item to parse and send each request then send
	// the response back
	for scanner.Scan() {
		if verbose {
			log.Println("Request:\n", scanner.Text())
		}

		wg.Add(1)
		go func(text string){
			defer wg.Done()
			req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(text)))
			if err != nil {
				log.Println("Error parsing request:", err)
				return
			}
			req.RequestURI, req.URL.Scheme, req.URL.Host = "", "https", req.Host
			if verbose {
				log.Println("Generated request")
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Println("Error sending request:", err)
				return
			}
			if verbose {
				log.Println("Sent request")
			}

			m.Lock()
			resp.Write(conn)
			// Write to stdout as well, if not already set to stdout
			if !stdin && verbose {
				resp.Write(os.Stdout)
			}
			m.Unlock()
		}(scanner.Text())
	}

	wg.Wait()
	log.Println("End of input")
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
