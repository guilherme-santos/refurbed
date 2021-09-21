# Refurbed Golang Test

This Golang test is composed by two parts: a library and a executable which uses the library.

## Executable

The executable can be built using the following command

```sh
go build ./cmd/notifier
```

The previous command will generate the binary `notifier` in your current directory.

The `--url` flag is mandatory and specify the endpoint to be called for each line found in the file provided, if no file is provided STDIN will be used. More information on how the executable works and which flags are available can be found passing `--help` flag.

## Library

The library is available under `github.com/guilherme-santos/refurbed/notify`.

A example of usage can be found below:

```go
package main

import "github.com/guilherme-santos/refurbed/notify"

func main() {
    // opts is an optional list of modifiers passed in the Client contructor to configure the instance
    var opts []notify.Option
    opts = append(opts, notify.WithHTTPClient(http.DefaultClient)) // Use http.DefaultClient to make HTTP requests
    opts = append(opts, notify.MaxParallel(1000)) // Set the max number of parallels requests to 1000

    ctx := context.Background()

    client := notify.NewClient(notifyURL, opts...)
    result := client.Notify(ctx, "my message goes here")

    // Block and wait the notification is over and handle the err.
    err := result.Wait()
    if err != nil {
        fmt.Println("Unable to send message:", err)
        os.Exit(1)
    }
    fmt.Println("Message sent")
}
```

### Notify method

The `Notify` is non-blocking if the number of ongoing requests is less than 1000 (this number can be changed passing the `MaxParallel()` option), once the limit is reach, new calls for the method will block until previous requests are over.

The `Notify` method returns a `*notify.Result` with two methods, `Err()` and `Wait()`.

* `Err()`: returns if the operation was success (returns nil) or a failure (return non-nil error). This method doesn't not guarantee that the operation is over, but once it is, all the subsequent calls will return the same error or nil.
* `Wait()` blocks and wait the operation is over and returns success (returns nil) or a failure (return non-nil error).

If you're not interested in the result of the operation and wants to have a completely async operation, is safe to ignore the return of `Notify()` method.

### Developing

After do the changes in the code, make sure to type `go test ./...` to run all unit tests available.
