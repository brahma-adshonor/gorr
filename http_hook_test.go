package regression

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

type dummyHttpHandlerForTest struct{}

var (
	dummyRspData = []byte("dummy data for http rsp")
)

func (h *dummyHttpHandlerForTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("dummy http server handler\n")
	w.Write(dummyRspData)
}

func TestHttpGet(t *testing.T) {
	EnableRegressionEngine(RegressionRecord)
	UnHookHttpServerHandler()

	h := &dummyHttpHandlerForTest{}

	http.Handle("/test_http", h)
	go http.ListenAndServe("127.0.0.1:2233", nil)
	time.Sleep(time.Duration(500) * time.Millisecond)

	c := http.Client{Timeout: time.Duration(1) * time.Second}
	r1, err1 := c.Get("http://localhost:2233/test_http")
	if err1 != nil {
		fmt.Printf("error:%s\n", err1.Error())
	}
	assert.Nil(t, err1)

	data1, err11 := ioutil.ReadAll(r1.Body)
	assert.Nil(t, err11)
	assert.True(t, len(data1) > 0)

	GlobalMgr.SetState(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(100))

	err := HookHttpFunc()
	assert.Nil(t, err)

	r2, err2 := c.Get("http://localhost:2233/test_http")
	if err2 != nil {
		fmt.Printf("error:%s\n", err2.Error())
	}
	assert.Nil(t, err2)

	data2, err21 := ioutil.ReadAll(r2.Body)
	assert.Nil(t, err21)
	assert.True(t, len(data2) > 0)

	GlobalMgr.SetState(RegressionReplay)

	r3, err3 := c.Get("http://localhost:2233/test_http")
	if err3 != nil {
		fmt.Printf("error:%s\n", err3.Error())
	}
	assert.Nil(t, err3)

	data3, err31 := ioutil.ReadAll(r3.Body)
	assert.Nil(t, err31)
	assert.True(t, len(data3) > 0)

	assert.Equal(t, len(data2), len(data3))
	assert.Equal(t, data2, data3)

	r4, err4 := c.Get("http://localhost:2233/test_http")
	if err4 != nil {
		fmt.Printf("error:%s\n", err4.Error())
	}
	assert.Nil(t, err4)

	data4, err41 := ioutil.ReadAll(r4.Body)
	assert.Nil(t, err41)
	assert.True(t, len(data4) > 0)

	assert.Equal(t, len(data2), len(data4))
	assert.Equal(t, data2, data4)

	_, err5 := c.Get("http://localhost:2233/test_http2")
	assert.NotNil(t, err5)

	UnHookHttpFunc()
	GlobalMgr.ClearStorage()
}

func TestHttpPost(t *testing.T) {
	c := http.Client{Timeout: 0}
	m := map[string]string{"ip": "1.1.1.1", "date": "2018-09-08"}
	j, _ := json.Marshal(m)
	d := bytes.NewBuffer(j)

	r1, err1 := c.Post("http://localhost:2233/test_http", "application/json", d)
	if err1 != nil {
		fmt.Printf("error:%s\n", err1.Error())
	}
	assert.Nil(t, err1)

	data1, err11 := ioutil.ReadAll(r1.Body)
	assert.Nil(t, err11)
	assert.True(t, len(data1) > 0)

	GlobalMgr.SetStorage(NewMapStorage(100))
	GlobalMgr.SetState(RegressionRecord)

	err := HookHttpFunc()
	assert.Nil(t, err)

	r2, err2 := c.Post("http://localhost:2233/test_http", "application/json", d)
	if err2 != nil {
		fmt.Printf("error:%s\n", err2.Error())
	}
	assert.Nil(t, err2)

	data2, err21 := ioutil.ReadAll(r2.Body)
	assert.Nil(t, err21)
	assert.True(t, len(data2) > 0)

	fmt.Printf("raw rsp from post:%s\n", data2)

	GlobalMgr.SetState(RegressionReplay)

	r3, err3 := c.Post("http://localhost:2233/test_http", "application/json", d)
	if err3 != nil {
		fmt.Printf("error:%s\n", err3.Error())
	}
	assert.Nil(t, err3)

	data3, err31 := ioutil.ReadAll(r3.Body)
	assert.Nil(t, err31)
	assert.True(t, len(data3) > 0)

	assert.Equal(t, len(data2), len(data3))
	assert.Equal(t, data2, data3)

	r4, err4 := c.Post("http://localhost:2233/test_http", "application/json", d)
	if err4 != nil {
		fmt.Printf("error:%s\n", err4.Error())
	}
	assert.Nil(t, err4)

	data4, err41 := ioutil.ReadAll(r4.Body)
	assert.Nil(t, err41)
	assert.True(t, len(data4) > 0)

	assert.Equal(t, len(data2), len(data4))
	assert.Equal(t, data2, data4)

	_, err5 := c.Post("http://localhost:2233/test_http2", "application/json", d)
	assert.NotNil(t, err5)

	UnHookHttpFunc()
	GlobalMgr.ClearStorage()
}
