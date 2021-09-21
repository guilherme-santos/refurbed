package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/guilherme-santos/refurbed/notify"
)

var (
	notifyURL string
	interval  time.Duration
	parallel  int
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
		f, err := os.Open(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to open file: %s\n", err)
			os.Exit(1)
		}
		defer f.Close()

		scanner = bufio.NewScanner(f)
	default:
		scanner = bufio.NewScanner(os.Stdin)
	}

	c := notify.NewClient(notifyURL, notify.MaxParallel(parallel))
	_ = c

	// monitor SIGINT signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Keep track of the last Result and wait for it
	// before abort.
	var res *notify.Result

scanFor:
	for scanner.Scan() {
		txt := strings.TrimSpace(scanner.Text())
		if txt == "" {
			continue
		}
		fmt.Fprintf(os.Stdout, "Sending: %s\n", txt)

		res = c.Notify(ctx, txt)

		select {
		case <-time.After(interval):
		case <-sigs:
			// SIGINT was received
			// cancel the context to abort any remaining notification
			cancel()
			break scanFor
		}
	}

	if res != nil {
		// Wait until the last notification is over before abort
		res.Wait()
	}
}
