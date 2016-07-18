package fast

import (
	"errors"
	"io"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	. "github.com/ddo/go-between"
	"github.com/ddo/pick-json"
	"gopkg.in/ddo/go-dlog.v1"
	"gopkg.in/ddo/pick.v1"
	"gopkg.in/ddo/request.v1"
)

const (
	ENDPOINT = "https://fast.com"

	BUFFER_SIZE = 512

	MEASURE_TIMEOUT_MAX = 30
	MEASURE_TIMEOUT_MIN = 10
)

var debug = dlog.New("fast", nil)

var requestHeader = &request.Header{
	"User-Agent": "github.com/ddo/fast",
}

type Fast struct {
	token string

	url      string
	urlCount int

	client *request.Client
}

// create empty Fast instance with a http client
func New() *Fast {
	return &Fast{
		client: request.New(),
	}
}

// init token and url
func (f *Fast) Init() (err error) {
	debug()

	////////////////////////// GET JS URL //////////////////////////
	res, err := f.client.Request(&request.Option{
		Url:    ENDPOINT,
		Header: requestHeader,
	})

	if err != nil {
		debug("ERR(http)", err)
		return
	}

	defer res.Body.Close()

	srcArr := pick.PickAttr(&pick.Option{
		PageSource: res.Body,
		TagName:    "script",
		Attr:       nil,
	}, "src", 1)

	if len(srcArr) == 0 {
		err = errors.New("<script> src attr not found")
		debug("ERR(PickAttr)", err)
		return
	}

	jsUrl := srcArr[0]
	debug("jsUrl", jsUrl)
	////////////////////////// GET JS URL //////////////////////////

	////////////////////////// URL & GET TOKEN & URLCOUNT //////////////////////////
	resJs, err := f.client.Request(&request.Option{
		Url:    ENDPOINT + jsUrl,
		Header: requestHeader,
	})

	if err != nil {
		debug("ERR(http)", err)
		return
	}

	defer resJs.Body.Close()

	// read all js data
	jsByteArr, err := ioutil.ReadAll(resJs.Body)

	if err != nil {
		debug("ERR(ReadAll)", err)
		return
	}

	jsStr := string(jsByteArr)

	// apiEndpoint="api.fast.com/netflix/speedtest"
	url := Between(jsStr, `apiEndpoint="`, `"`)

	if url == "" {
		err = errors.New("token not found")
		debug("ERR(Between)", err)
	}

	f.url = "https://" + url
	debug("url", f.url)

	// token:"YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm"
	token := Between(jsStr, `token:"`, `"`)

	if token == "" {
		err = errors.New("token not found")
		debug("ERR(Between)", err)
	}

	f.token = token
	debug("token", f.token)

	// maxConnections:3
	maxConnections := Between(jsStr, `maxConnections:`, `,`)

	if maxConnections == "" {
		err = errors.New("maxConnections not found")
		debug("ERR(Between)", err)
	}

	urlCount, err := strconv.Atoi(maxConnections)

	if err != nil {
		debug("ERR(Atoi)", err)
		return
	}

	f.urlCount = urlCount
	debug("urlCount", f.urlCount)
	////////////////////////// GET TOKEN & URLCOUNT //////////////////////////

	return
}

// call after #Init
// recall if 403 err from download
func (f *Fast) GetUrls() (urls []string, err error) {
	debug()

	res, err := f.client.Request(&request.Option{
		Url: f.url,
		Query: &request.Data{
			"https":    []string{"true"},
			"token":    []string{f.token},
			"urlCount": []string{strconv.Itoa(f.urlCount)},
		},
		Header: requestHeader,
	})

	if err != nil {
		debug("ERR(http)", err)
		return
	}

	defer res.Body.Close()

	urls = pickjson.PickString(res.Body, "url", 0)

	debug("urls", len(urls))
	return
}

// download stop when eof
// or done channel receive
func (f *Fast) download(url string, byteLenChan chan<- int64, done <-chan struct{}) (err error) {
	debug(url)

	res, err := f.client.Request(&request.Option{
		Url:    url,
		Header: requestHeader,
	})

	if err != nil {
		debug("ERR(http)", err)
		return
	}

	defer res.Body.Close()

	buf := make([]byte, BUFFER_SIZE)
	length := 0

	// read res.Body loop
	// loop till <-done
	// or eof
loop:
	for {
		select {

		case <-done:
			debug("<-done")
			break loop

		default:
			length, err = res.Body.Read(buf)

			byteLenChan <- int64(length)

			if err == io.EOF {
				// remove err
				err = nil

				debug("Read done")
				break loop
			}

			if err != nil {
				debug("ERR(Read)", err)
				break loop
			}
		}
	}

	debug("done")
	return
}

func (f *Fast) Measure(urls []string, KbpsChan chan<- float64) (err error) {
	debug()

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

		debug("stopped")
	}

	// sync
	var once sync.Once
	muxTimeout := sync.Mutex{}
	muxByteLen := sync.Mutex{}

	isTimeout := false

	// timeout min
	go func() {
		<-time.After(MEASURE_TIMEOUT_MIN * time.Second)

		muxTimeout.Lock()
		isTimeout = true
		muxTimeout.Unlock()

		debug("timeout min", MEASURE_TIMEOUT_MIN)
	}()
	// timeout min

	// timeout max
	go func() {
		<-time.After(MEASURE_TIMEOUT_MAX * time.Second)

		once.Do(stop)
		debug("timeout max", MEASURE_TIMEOUT_MAX)
	}()
	// timeout max

	// get byte length from downloads
	var byteLen int64 = 0

	go func() {
		for length := range byteLenChan {
			muxByteLen.Lock()
			byteLen += length
			muxByteLen.Unlock()
		}
	}()

	// measure per second
	var secondPass float64 = 0 // should be int but to save time convert
	var avgKbps float64 = 0

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

				debug("download done index:", index)

				// return on error
				if errDownload != nil {
					debug("ERR(download) index:", index)

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

	debug("done")
	return
}
