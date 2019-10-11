package gorr

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/brahma-adshonor/gohook"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
)

var (
	reqData = []byte("req data for http")
	rspData = []byte("rsp data for http")
)

type dummyHttpHandle struct{}

func (h *dummyHttpHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("http recorder dummy http server\n")

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	writer.Write(rspData)
	writer.Close()

	w.Write(buf.Bytes())

	w.WriteHeader(233)
	w.Header().Set("Content-Encoding", "gzip")
}

func TestRegister(t *testing.T) {

	UnHookHttpFunc()

	assert.Nil(t, HookHttpServerHandler())
	defer UnHookHttpServerHandler()

	fmt.Printf("debug info:%s\n", gohook.ShowDebugInfo())

	h := &dummyHttpHandle{}
	fmt.Printf("start testing http.Handle hook, h:%v\n", h)

	http.Handle("/test_point", h)
	assert.Equal(t, 0, len(handlerMap))

	var wg sync.WaitGroup
	wg.Add(1)

	RegisterHttpRecorder("/test_point2", func(p, n string, r *HttpData, s *HttpData) {
		fmt.Printf("recorder handler enter!\n")
		assert.Equal(t, 233, s.Status)
		assert.Equal(t, reqData, r.Body)
		assert.Equal(t, rspData, s.Body)
		wg.Done()
	}, nil)

	http.Handle("/test_point2", h)
	assert.Equal(t, 1, len(handlerMap))

	go http.ListenAndServe("127.0.0.1:2333", nil)
	time.Sleep(time.Duration(500) * time.Millisecond)

	c := http.Client{Timeout: time.Duration(1) * time.Second}
	rsp, err2 := c.Post("http://localhost:2333/test_point2", "application/json", bytes.NewBuffer(reqData))
	if err2 != nil {
		errReal, ok := err2.(*url.Error)
		if ok {
			fmt.Printf("post err:%v\n", *errReal)
		}
	}
	assert.Nil(t, err2)

	d, err3 := ioutil.ReadAll(rsp.Body)

	assert.Nil(t, err3)
	assert.Equal(t, rspData, d)

	fmt.Printf("resp data:%s\n", string(d))

	wg.Wait()
}
