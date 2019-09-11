package regression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/brahma-adshonor/gohook"
	"io/ioutil"
	"net/http"
	"sync"
)

type HttpData struct {
	Status int
	Body   []byte
	Header http.Header
}

type HttpRecorderHandler func(pattern string, req *HttpData, rsp *HttpData)

var (
	lock       sync.Mutex
	handlerMap = make(map[string]HttpRecorderHandler)
)

func RegisterHttpRecorder(pattern string, handler HttpRecorderHandler) {
	lock.Lock()
	defer lock.Unlock()
	handlerMap[pattern] = handler
}

type httpResponseWriterWrap struct {
	data HttpData
}

func (h *httpResponseWriterWrap) Header() http.Header {
	return h.data.Header
}

func (h *httpResponseWriterWrap) Write(d []byte) (int, error) {
	if h.data.Body == nil {
		h.data.Body = make([]byte, 0, len(d))
	}

	h.data.Body = append(h.data.Body, d...)

	return len(d), nil
}

func (h *httpResponseWriterWrap) WriteHeader(stCode int) {
	h.data.Status = stCode
}

type httpRecorder struct {
	pattern string
	origin  http.Handler
	handler HttpRecorderHandler
}

func (h *httpRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqData, _ := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(reqData))

	wr := &httpResponseWriterWrap{}
	wr.data.Status = -1
	wr.data.Header = w.Header()

	h.origin.ServeHTTP(wr, r)

	req := &HttpData{
		Body:   reqData,
		Header: r.Header,
	}

	if wr.data.Status != -1 {
		w.WriteHeader(wr.data.Status)
	}

	if len(wr.data.Body) > 0 {
		w.Write(wr.data.Body)
	}

	if wr.data.Header.Get("Content-Encoding") == "gzip" {
		reader, _ := gzip.NewReader(bytes.NewBuffer(wr.data.Body))
		wr.data.Body, _ = ioutil.ReadAll(reader)
		defer reader.Close()
	}

	h.handler(h.pattern, req, &wr.data)
}

func httpHandleHook(s *http.ServeMux, pattern string, handler http.Handler) {
	h := handler
	{
		lock.Lock()
		defer lock.Unlock()

		if val, ok := handlerMap[pattern]; ok {
			h = &httpRecorder{
				handler: val,
				origin:  handler,
				pattern: pattern,
			}
		}
	}

	httpHandleHookTramp(s, pattern, h)
}

// go:noinline
func httpHandleHookTramp(s *http.ServeMux, pattern string, handler http.Handler) {
	// make sure handler will be heap allocated.
	fmt.Printf("dummy function for regrestion testing:%s, %v", pattern, handler)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if handler != nil {
		panic("trampoline Http Post function is not allowed to be called")
	}
}

///////////////////////// setup hoook //////////////////////////////////

func HookHttpServerHandler() error {
	//return gohook.Hook(http.Handle, httpHandleHook, httpHandleHookTramp)
	var s *http.ServeMux
	return gohook.HookMethod(s, "Handle", httpHandleHook, httpHandleHookTramp)
}

func UnHookHttpServerHandler() error {
	//return gohook.UnHook(http.Handle)
	var s *http.ServeMux
	return gohook.UnHookMethod(s, "Handle")
}
