package gorr

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"gorr/util"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	tsChan          = make(chan string, 128)
	s3CaseDir       = flag.String("case_store_dir", "", "s3 path to store test case")
	testCaseHandler = flag.String("test_case_handler_tool", "", "tool to handle test case")
)

type MoveData struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

type TestCase struct {
	Req     string `json:"req"`
	Rsp     string `json:"rsp"`
	ReqType int    `json:"ReqType"`
	RspType int    `json:"RspType"`
	Desc    string `json:"Desc"`
	URI     string `json:"Uri"`
	Runner  string `json:"runner"`
}

type TestItem struct {
	DB        []string   `json:"db"`
	Flags     []string   `json:"flags,omitempty"`
	Input     []MoveData `json:"input,omitempty"`
	TestCases []TestCase `json:"cases"`
	Version   int        `json:"version"`
}

const (
	RecorderDataTypeUnknown  = 23
	RecorderDataTypeJson     = 24
	RecorderDataTypePbText   = 25
	RecorderDataTypePbBinary = 26
)

func RecordHttp(outDir, desc string, req *http.Request, rsp *http.Response, db []string) (string, error) {
	if req.Body == nil || rsp.Body == nil {
		return "", fmt.Errorf("invalid http data, req/rsp must contain body")
	}

	reqBody, err1 := ioutil.ReadAll(req.Body)
	if err1 != nil {
		return "", fmt.Errorf("read http req body failed, err:%s", err1.Error())
	}
	req.Body.Close()
	req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	rspBody, err2 := ioutil.ReadAll(rsp.Body)
	if err1 != nil {
		return "", fmt.Errorf("read http rsp body failed, err:%s", err2.Error())
	}
	rsp.Body.Close()
	rsp.Body = ioutil.NopCloser(bytes.NewBuffer(rspBody))

	//outDir := createOutputDir("case")
	return RecordData("", outDir, "", reqBody, RecorderDataTypeJson, rspBody, RecorderDataTypeJson, desc, db)
}

func RecordGrpc(outDir, desc string, req proto.Message, rsp proto.Message, db []string) (string, error) {
	d1, err1 := json.Marshal(req)
	if err1 != nil {
		return "", fmt.Errorf("marshal grpc request failed, err:%s", err1.Error())
	}

	//outDir := createOutputDir("case")

	m := jsonpb.Marshaler{}
	d2, err2 := m.MarshalToString(rsp)
	if err2 != nil {
		return "", fmt.Errorf("marshal grpc response failed, err:%s", err2.Error())
	}

	return RecordData("", outDir, "", d1, RecorderDataTypePbBinary, []byte(d2), RecorderDataTypeJson, desc, db)
}

func genUniqueFileName(dir, prefix, suggest string) string {
	name := fmt.Sprintf("%s_%s.dat", prefix, suggest)
	path := dir + "/" + name
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return name
	}

	for i := 0; i < 102400; i++ {
		name = fmt.Sprintf("%s_%s_%d.dat", prefix, suggest, i)
		path = dir + "/" + name
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return name
		}
	}

	panic("can not create gorr output dir")
}

func RecordData(uri, outDir, name string, req []byte, t1 int, rsp []byte, t2 int, desc string, db []string) (string, error) {
	f1 := genUniqueFileName(outDir, "reg_req", name)
	f2 := genUniqueFileName(outDir, "reg_rsp", name)

	reqFile := outDir + "/" + f1
	rspFile := outDir + "/" + f2
	configFile := outDir + "/reg_config.json"

	err := ioutil.WriteFile(reqFile, req, 0644)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(rspFile, rsp, 0644)
	if err != nil {
		return "", err
	}

	data := make([]string, 0, len(db))
	for _, f := range db {
		idx := strings.LastIndex(f, "/")
		if idx == -1 {
			return "", fmt.Errorf("invalid db file path, file:%s", f)
		}
		name := f[idx+1:]
		to := outDir + "/" + name
		err = util.CopyFile(f, to)
		if err != nil {
			return "", fmt.Errorf("copy db file failed, from:%s, to:%s, err:%s", f, to, err.Error())
		}

		data = append(data, name)
	}

	ts := time.Now().Format(time.RFC3339)

	td := TestItem{
		DB:      data,
		Version: 1,
		Flags:   []string{"-gorr_run_type=2", fmt.Sprintf("-server_time=%s", ts)},
		TestCases: []TestCase{
			TestCase{
				Req:     f1,
				Rsp:     f2,
				URI:     uri,
				ReqType: t1,
				RspType: t2,
				Desc:    desc,
			},
		},
	}

	if len(data) > 0 {
		mainDb := fmt.Sprintf("-gorr_db_file=%s", data[0])
		td.Flags = append(td.Flags, mainDb)
	}

	cd, err := ioutil.ReadFile(configFile)
	if err == nil && len(cd) > 0 {
		var cf TestItem
		err = json.Unmarshal(cd, &cf)
		if err != nil {
			return "", fmt.Errorf("unmarshal existed config failed, err:%s", err)
		}
		td.TestCases = append(td.TestCases, cf.TestCases...)
	}

	conf, err2 := json.MarshalIndent(td, "", "\t")
	if err2 != nil {
		return "", fmt.Errorf("generate test case config failed, err:%s", err2.Error())
	}

	err = ioutil.WriteFile(configFile, conf, 0644)
	if err != nil {
		return "", fmt.Errorf("writing config file failed, err:%s", err.Error())
	}

	if len(*testCaseHandler) > 0 && len(*s3CaseDir) > 0 {
		tsChan <- outDir
	}

	return outDir, nil
}

func RunTestCaseUploader() {
	if len(*testCaseHandler) > 0 && len(*s3CaseDir) > 0 {
		go doUpload()
	}
}

func doUpload() {
	for {
		d := <-tsChan
		cmd := fmt.Sprintf("S3_CASE_DIR=%s LOCAL_CASE_DIR=%s %s", *s3CaseDir, d, *testCaseHandler)

		out, err := util.RunCmd(cmd)

		m := fmt.Sprintf("output:%s, err:%s", out, err)
		GlobalMgr.notifier("upload test case done", cmd, []byte(m))
	}
}
