package gorr

import (
	"context"
	"errors"
	"fmt"
	//"reflect"
	"encoding/json"
	"github.com/brahma-adshonor/gohook"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// grpc functions are exposed by interface, which cannot be hooked directly.
/* there are 2 ways to hook grpc call:
1. hook the grpc NewXXX(), and return a cumstomized grpc object instead.
2. hook grpc.ClientConn.Invoke()

#1 required more work, but more compatible, at the cost of maintainability
#2 rely on grpc internal client implementation, which is much easier to implement, hook once for all grpc call.
*/

func buildReqKey(cxt context.Context, req interface{}, method string, data []byte) string {
	id := GlobalMgr.GetCurTraceId()

	tag := GlobalMgr.genKey(RegressionGrpcHook, cxt, req)
	if len(tag) == 0 {
		tag = string(data)
	}

	key := fmt.Sprintf("%s@@grpc_hook_key@@%s@@%s", id, method, tag)
	return key
}

func connGetState(*grpc.ClientConn) connectivity.State {
	return connectivity.Ready
}

//go: noinline
func grpcDialContextHook(ctx context.Context, target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	cc := &grpc.ClientConn{}
	return cc, nil
}

//go:noinline
func grpcConnClose(*grpc.ClientConn) error {
	return nil
}

type storeValue struct {
	Err   string `json:"err"`
	Value []byte `json:"value"`
}

func grpcInvokeHook(cc *grpc.ClientConn, ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error {
	if GlobalMgr.ShouldRecord() {
		err := grpcInvokeHookTrampoline(cc, ctx, method, args, reply, opts...)

		req, ok1 := args.(proto.Message)
		buff := make([]byte, 256)
		kb := proto.NewBuffer(buff)

		// by default, serialization of map type can differ from run to run even given the same input.
		// we need serialization to be consistent.
		// however we can not change this setting directly from proto.Marshal()
		// the only way available at this writting is to use proto.Buffer()
		kb.SetDeterministic(true)

		err1 := kb.Marshal(req)
		key := buildReqKey(ctx, args, method, kb.Bytes())

		if err == nil {
			rsp, ok2 := reply.(proto.Message)
			if ok1 && ok2 {
				rspData, err2 := proto.Marshal(rsp)
				if err1 == nil && err2 == nil {
					val := storeValue{
						Err:   "",
						Value: rspData,
					}

					d, _ := json.Marshal(val)
					err = GlobalMgr.StoreValue(key, d)
					GlobalMgr.notifier("grpc recording", key+"@@"+proto.MarshalTextString(req), rspData)
				}
			}
		} else {
			val := storeValue{
				Err:   err.Error(),
				Value: nil,
			}

			d, _ := json.Marshal(val)
			err = GlobalMgr.StoreValue(key, d)
			GlobalMgr.notifier("grpc recording error msg", key+"@@"+proto.MarshalTextString(req), []byte(err.Error()))
		}

		if err != nil {
			GlobalMgr.notifier("grpc recording failed", method, []byte(err.Error()))
		}

		return err
	}

	req, ok1 := args.(proto.Message)
	rsp, ok2 := reply.(proto.Message)

	err := errors.New("invalid proto message")
	// fmt.Printf("ok1:%t, ok2:%t\n", ok1, ok2)

	if ok1 && ok2 {
		buff := make([]byte, 256)
		kb := proto.NewBuffer(buff)
		kb.SetDeterministic(true)
		err1 := kb.Marshal(req)
		if err1 == nil {
			key := buildReqKey(ctx, args, method, kb.Bytes())
			value, err2 := GlobalMgr.GetValue(key)
			if err2 == nil {
				var val storeValue
				err = json.Unmarshal(value, &val)
				if err == nil {
					if len(val.Err) > 0 {
						err = fmt.Errorf("%s", val.Err)
					} else {
						err = proto.Unmarshal(val.Value, rsp)
					}
				}
			} else {
				err = errors.New("GetValue from db failed for request")
			}
			GlobalMgr.notifier("grpc replaying", key+"@@"+proto.MarshalTextString(req), value)
		} else {
			err = errors.New("marshal request failed")
		}
	}

	if err != nil {
		// serialized messages
		GlobalMgr.notifier("grpc replay failed", method, []byte(err.Error()))
	}

	return err
}

func grpcInvokeHookTrampoline(cc *grpc.ClientConn, ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error {
	fmt.Printf("dummy function for regrestion testing:%v", cc)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cc != nil {
		panic("trampoline for grpc.Invoke() function is not allowed to be called")
	}

	return nil
}

func HookGrpcInvoke() error {
	cc := &grpc.ClientConn{}
	err1 := gohook.HookMethod(cc, "Invoke", grpcInvokeHook, grpcInvokeHookTrampoline)
	if err1 != nil {
		return err1
	}

	if !GlobalMgr.ShouldRecord() {
		err2 := gohook.Hook(grpc.DialContext, grpcDialContextHook, nil)
		if err2 != nil {
			gohook.UnHookMethod(cc, "Invoke")
			return err2
		}

		err3 := gohook.HookMethod(cc, "GetState", connGetState, nil)
		if err3 != nil {
			gohook.UnHook(grpc.DialContext)
			gohook.UnHookMethod(cc, "Invoke")
			return err3
		}

		err4 := gohook.HookMethod(cc, "Close", grpcConnClose, nil)
		if err4 != nil {
			gohook.UnHook(grpc.DialContext)
			gohook.UnHookMethod(cc, "Invoke")
			gohook.UnHookMethod(cc, "GetState")
			return err4
		}
	}

	return nil
}

func UnHookGrpcInvoke() error {
	cc := &grpc.ClientConn{}
	if !GlobalMgr.ShouldRecord() {
		gohook.UnHook(grpc.DialContext)
		gohook.UnHookMethod(cc, "Close")
		gohook.UnHookMethod(cc, "GetState")
	}

	return gohook.UnHookMethod(cc, "Invoke")
}
