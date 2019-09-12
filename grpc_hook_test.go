package regression

import (
	"context"
	"errors"
	"fmt"
	"github.com/brahma-adshonor/gohook"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"testing"
)

var (
	rpcRspValue = &GrpcHookResponse{
		RspId:   45678,
		ReqId:   12345,
		RspName: "miliao-test-for-response",
		RspData: "9999999999999999999999999999999999999999999999999999999999",
	}

	globalConnValue = &grpc.ClientConn{}
)

func grpcInvokeDummy(cc *grpc.ClientConn, ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error {
	globalConnValue = cc
	fmt.Printf("calling dummy invoke from testing, cc:%p\n", cc)

	rsp, ok := reply.(*GrpcHookResponse)
	if !ok {
		fmt.Println("invalid response type to dummy invoke")
		return errors.New("invalid reponse type")
	}

	req, _ := args.(*GrpcHookRequest)

	if req.ReqId == 0 {
		return fmt.Errorf("error for id 0")
	}

	*rsp = *rpcRspValue
	rsp.ReqId = req.ReqId

	return nil
}

func setupHook(t *testing.T) {
	enableRegressionEngine(RegressionRecord)

	GlobalMgr.SetState(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(100))

	err1 := HookGrpcInvoke()
	assert.Nil(t, err1)

	err2 := gohook.Hook(grpcInvokeHookTrampoline, grpcInvokeDummy, nil)
	assert.Nil(t, err2)

	GlobalMgr.SetNotify(func(t string, key string, value []byte) {
		fmt.Printf("regression event, type:%s, key:%s, value:", t, key)
		for _, b := range value {
			fmt.Printf("%x ", b)
		}
		fmt.Printf("\n")
	})
}

func TestCallSome(t *testing.T) {
	setupHook(t)

	req1 := &GrpcHookRequest{
		ReqId:   2333,
		ReqName: "miliao-test-grpc-hook",
		ReqData: "12345678900987654321qwertyuioplkjhgfdsazxcvbnm",
	}

	req2 := &GrpcHookRequest{
		ReqId:   2332,
		ReqName: "miliao-test-grpc-hook2",
		ReqData: "12345678900987654321qwertyuioplkjhgfdsazxcvbnm",
	}

	conn := &grpc.ClientConn{}
	client := NewGrpcHookServiceClient(conn)
	ctx := context.TODO()
	rsp1, err := client.SomeCall(ctx, req1)

	assert.Nil(t, err)
	if err != nil {
		fmt.Printf("rpc call to SomeCall() failed, error:%s\n", err.Error())
	}

	assert.Equal(t, conn, globalConnValue)
	assert.Equal(t, req1.GetReqId(), rsp1.GetReqId())
	assert.Equal(t, rpcRspValue.GetRspId(), rsp1.GetRspId())
	assert.Equal(t, rpcRspValue.GetRspName(), rsp1.GetRspName())
	assert.Equal(t, rpcRspValue.GetRspData(), rsp1.GetRspData())
	assert.Equal(t, req1.ReqName, req1.GetReqName())
	assert.Equal(t, req1.ReqData, req1.GetReqData())

	rpcRspValue.ReqId = req1.ReqId
	rd1, err1 := proto.Marshal(rsp1)
	rd2, err2 := proto.Marshal(rpcRspValue)

	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, rd1, rd2)

	rsp3, _ := client.SomeCall(ctx, req2)

	rpcRspValue.ReqId = req2.ReqId
	rd4, _ := proto.Marshal(rsp3)
	rd5, _ := proto.Marshal(rpcRspValue)
	assert.Equal(t, rd4, rd5)
	assert.NotEqual(t, rd1, rd4)

	// now replay
	GlobalMgr.SetState(RegressionReplay)
	gohook.UnHook(grpcInvokeHookTrampoline)

	UnHookGrpcInvoke()
	err1 = HookGrpcInvoke()
	assert.Nil(t, err1)

	req11 := &GrpcHookRequest{
		ReqId:   2333,
		ReqName: "miliao-test-grpc-hook",
		ReqData: "12345678900987654321qwertyuioplkjhgfdsazxcvbnm",
	}

	rsp2, err3 := client.SomeCall(ctx, req11)
	assert.Nil(t, err3)

	rd3, err4 := proto.Marshal(rsp2)
	assert.Nil(t, err4)
	assert.Equal(t, rd2, rd3)

	rsp4, err5 := client.SomeCall(ctx, req2)
	assert.Nil(t, err5)

	rpcRspValue.ReqId = req2.ReqId
	rd6, _ := proto.Marshal(rsp4)
	rd7, _ := proto.Marshal(rpcRspValue)
	assert.Equal(t, rd6, rd7)
	assert.Equal(t, rd4, rd6)
	assert.NotEqual(t, rd1, rd6)

	req11.ReqId = 0
	_, err6 := client.SomeCall(ctx, req11)
	assert.NotNil(t, err6)
}
