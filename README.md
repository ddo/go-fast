# go-fast [![Build Status][semaphoreci-img]][semaphoreci-url] [![Doc][godoc-img]][godoc-url]
> fast.com api for go - pure - no headless browser or stuff

[godoc-img]: https://img.shields.io/badge/godoc-Reference-brightgreen.svg?style=flat-square
[godoc-url]: https://godoc.org/gopkg.in/ddo/go-fast.v0

[semaphoreci-img]: https://semaphoreci.com/api/v1/ddo/go-fast/branches/master/badge.svg
[semaphoreci-url]: https://semaphoreci.com/ddo/go-fast

## installation

```sh
go get -u gopkg.in/ddo/go-fast.v0
```

## example

```go
fastCom := fast.New()

// init
err := fastCom.Init()

if err != nil {
    panic(err)
}

// get urls
urls, err := fastCom.GetUrls()

if err != nil {
    panic(err)
}

// measure
KbpsChan := make(chan float64)

go func() {
    for Kbps := range KbpsChan {
        fmt.Printf("%.2f Kbps %.2f Mbps\n", Kbps, Kbps/1000)
    }

    fmt.Println("done")
}()

err = fastCom.Measure(urls, KbpsChan)

if err != nil {
    panic(err)
}
```

## debug

to enable log set environment variable as

```go
DLOG=*
```

or

```shell
DLOG=* go run/test...
```

## test

```shell
go test -v
```

## TODO

* send analytic info to fast.com