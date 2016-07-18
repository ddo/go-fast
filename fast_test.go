package fast

import (
	"fmt"
	"testing"
	"time"

	"github.com/ddo/go-spin"
)

var f *Fast
var testing_urls []string

func TestNew(t *testing.T) {
	f = New()

	if f.client == nil {
		t.Error()
	}
}

func TestInit(t *testing.T) {
	err := f.Init()

	if err != nil {
		t.Error()
		return
	}

	if f.url == "" {
		t.Error()
	}

	if f.token == "" {
		t.Error()
	}

	if f.urlCount == 0 {
		t.Error()
	}
}

func TestGetUrls(t *testing.T) {
	urls, err := f.GetUrls()

	if err != nil {
		t.Error()
		return
	}

	if len(urls) != f.urlCount {
		t.Error()
	}

	testing_urls = urls
}

func TestDownload(t *testing.T) {
	byteLenChan := make(chan int64)
	done := make(chan struct{})

	spinner := spin.New("")

	go func() {
		for range byteLenChan {
			fmt.Printf(" \r %s", spinner.Spin())
		}
	}()

	err := f.download(testing_urls[0], byteLenChan, done)

	if err != nil {
		t.Error()
		return
	}

	fmt.Println("done")
}

// stop after 5s
func TestDownloadStop(t *testing.T) {
	byteLenChan := make(chan int64)
	done := make(chan struct{})

	spinner := spin.New("")

	go func() {
		for range byteLenChan {
			fmt.Printf(" \r %s", spinner.Spin())
		}
	}()

	go func() {
		<-time.After(2 * time.Second)
		close(done)
	}()

	err := f.download(testing_urls[0], byteLenChan, done)

	if err != nil {
		t.Error()
		return
	}

	fmt.Println("done")
}

func TestMeasure(t *testing.T) {
	KbpsChan := make(chan float64)

	spinner := spin.New("")

	go func() {
		for Kbps := range KbpsChan {
			fmt.Printf(" \r %s %.2f Kbps %.2f Mbps", spinner.Spin(), Kbps, Kbps/1000)
		}
	}()

	err := f.Measure(testing_urls, KbpsChan)

	if err != nil {
		t.Error()
		return
	}

	fmt.Println("done")
}
