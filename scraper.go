package fast

import (
	"bytes"
	"regexp"
	"strconv"

	"github.com/ddo/rq"
	"github.com/ddo/rq/client"
	"gopkg.in/ddo/pick.v1"
)

var (
	reEndpoint = regexp.MustCompile(`apiEndpoint="([\w|\/|\.]*)"`)
	reToken    = regexp.MustCompile(`token:"(\w*)"`)
	reCount    = regexp.MustCompile(`urlCount:(\d*)`)
)

// example: /app-d17c81.js then it's data
func getJSFile(_client *client.Client) (data []byte, err error) {
	// fetch html
	r := rq.Get(endpoint)
	data, _, err = _client.Send(r, true)
	if err != nil {
		err = errInternet
		return
	}

	// get <script> src value
	srcArr := pick.PickAttr(&pick.Option{
		PageSource: bytes.NewReader(data),
		TagName:    "script",
		Attr:       nil,
	}, "src", 1)
	if len(srcArr) == 0 {
		debug.Error("PickAttr", err)
		err = errAPI
		return
	}

	jsSrc := srcArr[0]
	debug.Done("jsSrc:", jsSrc)

	// fetch js
	// TODO: url join
	r = rq.Get(endpoint + jsSrc)
	data, _, err = _client.Send(r, true)
	if err != nil {
		err = errInternet
		return
	}

	return
}

// example: api.fast.com/netflix/speedtest
func getAPIEndpoint(data []byte) (url string, err error) {
	res := reEndpoint.FindSubmatch(data)
	if len(res) < 2 {
		debug.Error("no apiEndpoint")
		err = errAPI
		return
	}

	url = string(res[1])

	debug.Done("url:", url)
	return
}

// example: YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm
func getToken(data []byte) (token string, err error) {
	res := reToken.FindSubmatch(data)
	if len(res) < 2 {
		debug.Error("no token")
		err = errAPI
		return
	}

	token = string(res[1])

	debug.Done("token:", token)
	return
}

// example: 5
func getURLCount(data []byte) (count int, err error) {
	res := reCount.FindAllSubmatch(data, -1)
	if len(res) < 2 {
		debug.Error("no urlCount")
		err = errAPI
		return
	}

	count, err = strconv.Atoi(string(res[len(res)-1][1]))
	if len(res) < 2 {
		debug.Error(err)
		err = errAPI
		return
	}

	debug.Done("count:", count)
	return
}
