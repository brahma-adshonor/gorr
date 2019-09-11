package regression

import (
	"adshonor/common/util"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type MoveData struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

type TestCase struct {
	Req    string `json:"req"`
	Rsp    string `json:"rsp"`
	Desc   string `json:"Desc"`
	Runner string `json:"runner"`
}

type TestItem struct {
	DB        []string   `json:"db"`
	Flags     []string   `json:"flags,omitempty"`
	Input     []MoveData `json:"input,omitempty"`
	TestCases []TestCase `json:"cases"`
}

var (
	regression_output_dir = flag.String("regression_record_output_dir", "/var/data/conf/regression", "dir to store auto generated test cases")
)

func RecordHttp(desc string, req *http.Request, rsp *http.Response, db []string) (error, string) {
	if req.Body == nil || rsp.Body == nil {
		return fmt.Errorf("invalid http data, req/rsp must contain body"), ""
	}

	reqBody, err1 := ioutil.ReadAll(req.Body)
	if err1 != nil {
		return fmt.Errorf("read http req body failed, err:%s", err1.Error()), ""
	}
	req.Body.Close()
	req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	rspBody, err2 := ioutil.ReadAll(rsp.Body)
	if err1 != nil {
		return fmt.Errorf("read http rsp body failed, err:%s", err2.Error()), ""
	}
	rsp.Body.Close()
	rsp.Body = ioutil.NopCloser(bytes.NewBuffer(rspBody))

	return RecordData(reqBody, rspBody, desc, db)
}

func RecordGrpc(desc string, req proto.Message, rsp proto.Message, db []string) (error, string) {
	d1, err1 := json.Marshal(req)
	if err1 != nil {
		return fmt.Errorf("marshal grpc request failed, err:%s", err1.Error()), ""
	}

	d2 := proto.MarshalTextString(rsp)
	return RecordData(d1, []byte(d2), desc, db)
}

func createOutputDir() string {
	for i := 0; i < 102400; i++ {
		path := fmt.Sprintf("%s/case%d", *regression_output_dir, i)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.Mkdir(path, 0755)
			return path
		}
	}

	panic("can not create regression output dir")
}

func RecordData(req []byte, rsp []byte, desc string, db []string) (error, string) {
	outDir := createOutputDir()

	f1 := "reg_req.dat"
	f2 := "reg_rsp.dat"
	reqFile := outDir + "/" + f1
	rspFile := outDir + "/" + f2

	err := ioutil.WriteFile(reqFile, req, 0644)
	if err != nil {
		return err, ""
	}
	err = ioutil.WriteFile(rspFile, rsp, 0644)
	if err != nil {
		return err, ""
	}

	var data []string
	for _, f := range db {
		idx := strings.LastIndex(f, "/")
		if idx == -1 {
			return fmt.Errorf("invalid db file path, file:%s", f), ""
		}
		name := f[idx+1:]
		to := outDir + "/" + name
		err = util.CopyFile(f, to)
		if err != nil {
			return fmt.Errorf("copy db file failed, from:%s, to:%s, err:%s", f, to, err.Error()), ""
		}

		data = append(data, name)
	}

	td := TestItem{
		DB:    data,
		Flags: []string{"-regression_run_type=2"},
		TestCases: []TestCase{
			TestCase{
				Req:  f1,
				Rsp:  f2,
				Desc: desc,
			},
		},
	}

	conf, err2 := json.MarshalIndent(td, "", "\t")
	if err2 != nil {
		return fmt.Errorf("generate test case config failed, err:%s", err2.Error()), ""
	}

	err = ioutil.WriteFile(outDir+"/reg_config.json", conf, 0644)
	if err != nil {
		return fmt.Errorf("writing config file failed, err:%s", err.Error()), ""
	}

	return nil, outDir
}
