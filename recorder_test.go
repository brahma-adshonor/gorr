package gorr

import (
	"encoding/json"
	"github.com/golang/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestRecordHttp(t *testing.T) {
	*RegressionOutputDir = "/tmp"

	body1 := "miliao-http-test-data-set"
	req, err := http.NewRequest("POST", "http://some-no-exist.url.com", strings.NewReader(body1))
	assert.Nil(t, err)
	req.Header.Add("Content-Type", "application/json")

	body2 := "miliao-http-test-data-set"
	rsp := &http.Response{
		Body: ioutil.NopCloser(strings.NewReader(body2)),
	}

	db1 := "./reg_db1.db"
	db2 := "./reg_db2.db"
	ioutil.WriteFile(db1, []byte("ddddd"), 0644)
	ioutil.WriteFile(db2, []byte("ddddd"), 0644)

	var out string
	out, err = RecordHttp("miliao_http_test", req, rsp, []string{db1, db2})
	assert.Nil(t, err)

	reqData, err1 := ioutil.ReadFile(out + "/reg_req_.dat")
	rspData, err2 := ioutil.ReadFile(out + "/reg_rsp_.dat")

	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, body1, string(reqData))
	assert.Equal(t, body2, string(rspData))

	os.Remove(db1)
	os.Remove(db2)

	var ti TestItem
	conf, err3 := ioutil.ReadFile(out + "/reg_config.json")
	assert.Nil(t, err3)
	err = json.Unmarshal(conf, &ti)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(ti.DB))
	assert.Equal(t, 2, len(ti.Flags))
	assert.Equal(t, 1, len(ti.TestCases))

	assert.Equal(t, "-gorr_run_type=2", ti.Flags[0])
	assert.Equal(t, []string{"reg_db1.db", "reg_db2.db"}, ti.DB)
	assert.Equal(t, TestCase{Req: "reg_req_.dat", Rsp: "reg_rsp_.dat", Desc: "miliao_http_test", ReqType: RecorderDataTypeJson, RspType: RecorderDataTypeJson}, ti.TestCases[0])

	os.RemoveAll(out)
}

func TestRecordGrpc(t *testing.T) {
	*RegressionOutputDir = "/tmp"

	req := &GrpcHookRequest{
		ReqId:   2333,
		ReqName: "miliao-test-grpc-record",
		ReqData: "12345678900987654321qwertyuioplkjhgfdsazxcvbnm",
	}
	rsp := &GrpcHookResponse{
		RspId:   45678,
		ReqId:   12345,
		RspName: "miliao-test-for-response",
		RspData: "9999999999999999999999999999999999999999999999999999999999",
	}

	db1 := "./reg_db1.db"
	db2 := "./reg_db2.db"
	ioutil.WriteFile(db1, []byte("ddddd"), 0644)
	ioutil.WriteFile(db2, []byte("ddddd"), 0644)

	var out string
	out, err := RecordGrpc("miliao_test_grpc_record", req, rsp, []string{db1, db2})

	assert.Nil(t, err)

	os.Remove(db1)
	os.Remove(db2)

	reqData, err1 := ioutil.ReadFile(out + "/reg_req_.dat")
	rspData, err2 := ioutil.ReadFile(out + "/reg_rsp_.dat")

	assert.Nil(t, err1)
	assert.Nil(t, err2)

	req2 := &GrpcHookRequest{}
	rsp2 := &GrpcHookResponse{}

	err1 = json.Unmarshal(reqData, req2)
	err2 = jsonpb.UnmarshalString(string(rspData), rsp2)
	assert.Nil(t, err1)
	assert.Nil(t, err2)

	assert.Equal(t, req, req2)
	assert.Equal(t, rsp, rsp2)

	assert.Nil(t, os.RemoveAll(out))
}
