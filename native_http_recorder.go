package gorr

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

type HttpRequestFixer func(pattern string, data []byte) []byte
type HttpRecorderHandler func(pattern, desc string, req *HttpData, rsp *HttpData)

type httpHandleGroup struct {
	fixer    HttpRequestFixer
	recorder HttpRecorderHandler
}

var (
	lock       sync.Mutex
	handlerMap = make(map[string]httpHandleGroup)
)

func RegisterHttpRecorder(pattern string, handler HttpRecorderHandler, fix HttpRequestFixer) {
	lock.Lock()
	defer lock.Unlock()
	handlerMap[pattern] = httpHandleGroup{recorder: handler, fixer: fix}
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
	prepare func(string, []byte) []byte
}

func (h *httpRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqData, _ := ioutil.ReadAll(r.Body)

	if h.prepare != nil {
		reqData = h.prepare(h.pattern, reqData)
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(reqData))

	dname := r.Header.Get("RegressionName")

	// reset user-agent
	r.Header.Set("User-Agent", "RegressionTool")

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

	if wr.data.Header.Get("HttpResponseType") == "Failure" {
		v := fmt.Sprintf("pattern:%s, name:%s", h.pattern, dname)
		GlobalMgr.notifier("native http recorder", "ignoring error response", []byte(v))
		return
	}

	GlobalMgr.notifier("native http recorder", "recording http done", []byte(r.URL.Path+"@@"+r.URL.RawQuery))

	p := h.pattern
	if len(r.URL.RawQuery) > 0 {
		p = p + "?" + r.URL.RawQuery
	}

    if len(wr.data.Body) > 0 {
            h.handler(p, dname, req, &wr.data)
            GlobalMgr.notifier("native http recorder", "recording http done", []byte(r.URL.Path+"@@"+r.URL.RawQuery))
    } else {
            GlobalMgr.notifier("native http recorder", "empty reponse not recorded", []byte(r.URL.Path+"@@"+r.URL.RawQuery))
    }
}

func httpHandleHook(s *http.ServeMux, pattern string, handler http.Handler) {
	h := handler
	{
		lock.Lock()
		defer lock.Unlock()

		if val, ok := handlerMap[pattern]; ok {
			h = &httpRecorder{
				prepare: val.fixer,
				handler: val.recorder,
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
