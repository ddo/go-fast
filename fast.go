package fast

import (
	"errors"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/ddo/pick-json"
	"github.com/ddo/rq"
	"github.com/ddo/rq/client"
	"gopkg.in/ddo/go-dlog.v2"
)

const (
	endpoint = "https://fast.com"

	bufferSize = 512

	measureTimeoutMax = 30
	measureTimeoutMin = 10

	userAgent = "github.com/ddo/fast"
)

var (
	errAPI      = errors.New("Fast.com API error. Please try again later")
	errInternet = errors.New("Internet error. Please try again later")
)

var debug = dlog.New("fast", nil)

// Fast contains client config
type Fast struct {
	token string

	url      string
	urlCount int

	client *client.Client
}

// New creates empty Fast instance with a http client
func New() *Fast {
	// default client
	defaultRq := rq.Get(endpoint)
	defaultRq.Set("User-Agent", userAgent)

	return &Fast{
		client: client.New(&client.Option{
			NoCookie:  true,
			DefaultRq: defaultRq,
		}),
	}
}

// Init inits token and url
func (f *Fast) Init() (err error) {
	data, err := getJSFile(f.client)
	if err != nil {
		return
	}

	url, err := getAPIEndpoint(data)
	if err != nil {
		return
	}
	f.url = "https://" + url

	token, err := getToken(data)
	if err != nil {
		return
	}
	f.token = token

	urlCount, err := getURLCount(data)
	if err != nil {
		return
	}
	f.urlCount = urlCount

	return
}

// GetUrls gets the testing urls
// call after #Init
// recall if 403 err from download
func (f *Fast) GetUrls() (urls []string, err error) {
	r := rq.Get(f.url)
	r.Qs("https", "true")
	r.Qs("token", f.token)
	r.Qs("urlCount", strconv.Itoa(f.urlCount))

	_, res, err := f.client.Send(r, false)
	if err != nil {
		err = errInternet
		return
	}
	defer res.Body.Close()

	urls = pickjson.PickString(res.Body, "url", 0)
	debug.Info("urls:", len(urls))
	return
}

// download stop when eof
// or done channel receive
func (f *Fast) download(url string, byteLenChan chan<- int64, done <-chan struct{}) (err error) {
	r := rq.Get(url)
	_, res, err := f.client.Send(r, false)
	if err != nil {
		err = errInternet
		return
	}
	defer res.Body.Close()

	buf := make([]byte, bufferSize)
	var length int

	// read res.Body loop
	// loop till <-done
	// or eof
loop:
	for {
		select {

		case <-done:
			debug.Debug("<-done")
			break loop

		default:
			length, err = res.Body.Read(buf)

			byteLenChan <- int64(length)

			if err == io.EOF {
				// remove err
				err = nil

				debug.Debug("Read done")
				break loop
			}
			if err != nil {
				debug.Debug("Read", err)
				break loop
			}
		}
	}

	debug.Done("done")
	return
}

// Measure measures download speeds from given urls
func (f *Fast) Measure(urls []string, KbpsChan chan<- float64) (err error) {
	done := make(chan struct{})
	byteLenChan := make(chan int64)

	// measure per second
	ticker := time.NewTicker(1 * time.Second)

	// stop func
	// should clean all before stop #Measure
	stop := func() {
		ticker.Stop()

		close(done)
		// close(byteLenChan)
		close(KbpsChan)

		debug.Info("stopped")
	}

	// sync
	var once sync.Once
	muxTimeout := sync.Mutex{}
	muxByteLen := sync.Mutex{}

	isTimeout := false

	// timeout min
	go func() {
		<-time.After(measureTimeoutMin * time.Second)

		muxTimeout.Lock()
		isTimeout = true
		muxTimeout.Unlock()

		debug.Info("timeout min", measureTimeoutMin)
	}()
	// timeout min

	// timeout max
	go func() {
		<-time.After(measureTimeoutMax * time.Second)

		once.Do(stop)
		debug.Info("timeout max", measureTimeoutMax)
	}()
	// timeout max

	// get byte length from downloads
	var byteLen int64

	go func() {
		for length := range byteLenChan {
			muxByteLen.Lock()
			byteLen += length
			muxByteLen.Unlock()
		}
	}()

	// measure per second
	var secondPass float64 // should be int but to save time convert
	var avgKbps float64

	go func() {
	loop:
		for range ticker.C {
			// byte = 8 bit
			// 1 mega bit = 1,000 bit
			// 1 mega bit = 1,000,000 bit

			secondPass++

			select {

			case <-done:
				break loop

			default:
				muxByteLen.Lock()
				avgKbps = float64(byteLen) / secondPass
				muxByteLen.Unlock()

				KbpsChan <- avgKbps * 8 / 1000
			}
		}
	}()

	// start download urls
	for i := 0; i < len(urls); i++ {
		go func(index int) {

			timeout := false

			// loop re download till timeout min/max or err
			for !timeout {
				errDownload := f.download(urls[index], byteLenChan, done)

				debug.Info("download done index:", index)

				// return on error
				if errDownload != nil {
					debug.Error("download index:", index)

					err = errDownload

					once.Do(stop)
					return
				}

				muxTimeout.Lock()
				timeout = isTimeout
				muxTimeout.Unlock()
			}

			once.Do(stop)
		}(i)
	}

	<-done

	debug.Done("done")
	return
}
