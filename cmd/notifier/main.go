package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/guilherme-santos/refurbed/notify"
)

var (
	notifyURL string
	interval  time.Duration
	parallel  int
	verbose   bool
)

func init() {
	flag.Usage = func() {
		exec, output := os.Args[0], flag.CommandLine.Output()

		fmt.Fprintf(output, "Usage: %s [flags] --url=URL [file]:\n\n", exec)
		fmt.Fprintf(output, "Call the notification URL for each line read from <file> or STDIN\n\n")
		fmt.Fprintf(output, "Flags:\n")

		flag.PrintDefaults()
	}

	flag.StringVar(&notifyURL, "url", "", "notification URL")
	flag.DurationVar(&interval, "interval", 5*time.Second, "notification interval")
	flag.IntVar(&parallel, "parallel", 1000, "max notification in parallel")
	flag.BoolVar(&verbose, "v", false, "verbose mode")
}

func main() {
	flag.Parse()

	if notifyURL == "" {
		fmt.Fprintf(os.Stderr, "The notification url is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	var scanner *bufio.Scanner

	// create a new Scanner from the file or STDIN

	nargs := flag.NArg()
	switch {
	case nargs > 1:
		flag.Usage()
		os.Exit(1)
	case nargs == 1:
		fileName := flag.Arg(0)
		if verbose {
			fmt.Fprintf(os.Stdout, "Opening %s...\n", fileName)
		}

		f, err := os.Open(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to open file: %s\n", err)
			os.Exit(1)
		}
		defer f.Close()

		scanner = bufio.NewScanner(f)
	default:
		if verbose {
			fmt.Fprint(os.Stdout, "Reading from stdin\n")
		}
		scanner = bufio.NewScanner(os.Stdin)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// monitor SIGINT signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		<-sigs
		// Cancel context when SIGINT is received
		cancel()
	}()

	c := notify.NewClient(notifyURL, notify.MaxParallel(parallel))

	var wg sync.WaitGroup

scanFor:
	for scanner.Scan() {
		txt := strings.TrimSpace(scanner.Text())
		if txt == "" {
			continue
		}
		fmt.Fprintf(os.Stdout, "Sending message: %s\n", txt)

		wg.Add(1)

		res := c.Notify(ctx, txt)
		go func() {
			res.Wait()

			err := res.Err()
			if err != nil && err != context.Canceled {
				fmt.Fprintf(os.Stderr, "Error sending message %q: %s\n", txt, err)
			} else if verbose {
				fmt.Fprintf(os.Stdout, "Message %q was sent\n", txt)
			}
			wg.Done()
		}()

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			if verbose {
				fmt.Fprint(os.Stdout, "Aborting...\n")
			}
			break scanFor
		}
	}

	wg.Wait()
}
