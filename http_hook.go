package gorr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/brahma-adshonor/gohook"
)

type HttpResponseData struct {
	Status        string      `json:"status"`
	StatusCode    int         `json:"code"`
	Proto         string      `json:"proto"`
	ProtoMajor    int         `json:"major"`
	ProtoMinor    int         `json:"minor"`
	Header        http.Header `json:"header"`
	Body          []byte      `json:"body"`
	ContentLength int64       `json:"length"`
}

func genHttpReqKey(req *http.Request, url, method, proto string, body []byte) string {
	tag := GlobalMgr.genKey(RegressionHttpHook, context.Background(), req)
	if len(tag) == 0 {
		tag = string(body)
	}

	return fmt.Sprintf("http_request_key_prefix@@%s@@%s@@%s@@%s@@%s", GlobalMgr.GetCurTraceId(), url, method, proto, tag)
}

func saveResponse(key string, r *http.Response) error {
	data := HttpResponseData{
		Status:        r.Status,
		StatusCode:    r.StatusCode,
		Proto:         r.Proto,
		ProtoMajor:    r.ProtoMajor,
		ProtoMinor:    r.ProtoMinor,
		Header:        r.Header,
		Body:          nil,
		ContentLength: r.ContentLength,
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("http hook read body failed, err:%s", err.Error())
	}
	r.Body.Close()

	data.Body = body
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	jd, err2 := json.Marshal(data)
	if err2 != nil {
		return errors.New("marshal http response for hook failed")
	}

	return GlobalMgr.StoreValue(key, []byte(jd))
}

func getHttpResp(key string) (*http.Response, error) {
	value, err := GlobalMgr.GetValue(key)
	if err != nil {
		return nil, err
	}

	var data HttpResponseData
	err = json.Unmarshal(value, &data)
	if err != nil {
		return nil, err
	}

	resp := http.Response{
		Status:        data.Status,
		StatusCode:    data.StatusCode,
		Proto:         data.Proto,
		ProtoMajor:    data.ProtoMajor,
		ProtoMinor:    data.ProtoMinor,
		Header:        data.Header,
		Body:          ioutil.NopCloser(bytes.NewBuffer(data.Body)),
		ContentLength: data.ContentLength,
	}

	return &resp, nil
}

func doHttp(c *http.Client, req *http.Request) (*http.Response, error) {
	var err error
	var data []byte
	var rsp *http.Response

	url := req.URL
	proto := req.Proto
	method := req.Method

	if req.Body != nil {
		data, _ = ioutil.ReadAll(req.Body)
		req.Body.Close()
	}

	// reset body
	req.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	state := ""
	key := genHttpReqKey(req, url.String(), method, proto, data)

	if GlobalMgr.ShouldRecord() {
		state = "record"
		rsp, err = doHttpTrampoline(c, req)
		if err == nil {
			err = saveResponse(key, rsp)
		}
	} else {
		state = "replay"
		rsp, err = getHttpResp(key)
	}

	if err != nil {
		GlobalMgr.notifier(fmt.Sprintf("http.Do %s failed", state), key, []byte(err.Error()))
	} else {
		GlobalMgr.notifier(fmt.Sprintf("http.Do %s succeed", state), key, []byte(""))
	}

	return rsp, err
}

//go:noinline
func doHttpTrampoline(c *http.Client, req *http.Request) (*http.Response, error) {
	fmt.Printf("dummy function for regrestion testing:%v,%v", c, req)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline Http Post function is not allowed to be called")
	}

	return nil, nil
}

/*
func genHttpGetKey(url string) string {
	return fmt.Sprintf("%s@@%s@@%s", "http_get_key_prefix", GlobalMgr.GetCurTraceId(), url)
}

func genHttpPostKey(url, reqType string, body string) string {
	return fmt.Sprintf("%s@@%s@@%s@@%s@@%s", "http_post_key_prefix", GlobalMgr.GetCurTraceId(), url, reqType, body)
}

func HttpGet(c *http.Client, url string) (*http.Response, error) {
	var err error
	var rsp *http.Response

	key := genHttpGetKey(url)
	if GlobalMgr.ShouldRecord() {
		rsp, err = HttpGetTrampoline(c, url)
		if err == nil {
			err = saveResponse(key, rsp)
		}
		GlobalMgr.notifier("http get recording", key, []byte(""))
	} else {
		rsp, err = getHttpResp(key)
		GlobalMgr.notifier("http get replaying", key, []byte(""))
	}

	if err != nil {
		GlobalMgr.notifier("http get failed", key, []byte(err.Error()))
	} else {
		GlobalMgr.notifier("http get done", key, []byte(""))
	}

	return rsp, err
}

//go:noinline
func HttpGetTrampoline(c *http.Client, url string) (*http.Response, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
	}

	if c != nil {
		panic("trampoline Http Get function is not allowed to be called")
	}

	return nil, nil
}


//go:noinline
func HttpPostTrampoline(c *http.Client, url string, reqType string, body io.Reader) (*http.Response, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline Http Post function is not allowed to be called")
	}

	return nil, nil
}

func HttpPost(c *http.Client, url string, reqType string, body io.Reader) (*http.Response, error) {
	data, err1 := ioutil.ReadAll(body)
	if err1 != nil {
		GlobalMgr.notifier("http post failed", url, []byte("read body failed"))
		return nil, err1
	}

	var err error
	var rsp *http.Response

	bd := bytes.NewBuffer(data)
	key := genHttpPostKey(url, reqType, string(data))

	if GlobalMgr.ShouldRecord() {
		rsp, err = HttpPostTrampoline(c, url, reqType, bd)
		if err == nil {
			err = saveResponse(key, rsp)
		}
		GlobalMgr.notifier("http post recording", key, []byte(""))
	} else {
		rsp, err = getHttpResp(key)
		GlobalMgr.notifier("http post replaying", key, []byte(""))
	}

	if err != nil {
		GlobalMgr.notifier("http post replaying failed", key, []byte(err.Error()))
	} else {
		GlobalMgr.notifier("http post replaying done", key, []byte(""))
	}

	return rsp, err
}
*/

func HookHttpFunc() error {
	var c http.Client
	/*
		err := gohook.HookMethod(&c, "Get", HttpGet, HttpGetTrampoline)
		if err != nil {
			return err
		}

		err = gohook.HookMethod(&c, "Post", HttpPost, HttpPostTrampoline)
		if err != nil {
			gohook.UnHookMethod(&c, "Get")
			return err
		}
	*/

	err := gohook.HookMethod(&c, "Do", doHttp, doHttpTrampoline)
	if err != nil {
		return err
	}

	return nil
}

func UnHookHttpFunc() error {
	var c http.Client
	/*
		gohook.UnHookMethod(&c, "Get")
		gohook.UnHookMethod(&c, "Post")
		return nil
	*/
	return gohook.UnHookMethod(&c, "Do")
}
