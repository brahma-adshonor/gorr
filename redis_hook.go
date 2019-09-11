package regression

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/brahma-adshonor/gohook"
	"github.com/go-redis/redis"
)

// please disable inlining by adding following gcflags to compiler
// go build -gcflags=all='-l=1' -o main main.go

/* there are two ways to hook redis operation. We should support both, but use #2 by default.
1. use gohook to hook NewClient() or NewClusterClient(), and insert a call to hooks.AddHook() to hijack redis command processing.
    =>support from v6.15.4, which is not formally released by the date of this writting.
	=>rely on the internal implementation of cmdable.Get() etc.
	=>will skip other user customized Hook, as our Hook will always return error on replay state.
2. use gohook to hook client.Process() or client.ProcessContext()
	=>all user customized Hook added by AddHook will be skiped
	=>Process() might be inlined in future release, as it is a really short function, inlining can be fixed by adding build flags to compiler to stop inlining in debug build.
	=>due to the subtle implementation of redis.Client.Process() before 6.15.1, hook for client.Process won't work,
	 instead, we add wrapper to client.Process() by calling client.WrapProcess(), which does not present in later version, yep, it is tricky.

Redis command object(StringCmd, IntCmd, etc) is immutable, we are not able to set result into it, this makes a lot of troubles,
the workaround for this is we are going to hook the getter of the object. and maybe we should make a pull request to upstream, adding setter api to redis Command object.
*/

func buildRedisClientId(c *redis.Client) string {
	opt := c.Options()
	id := fmt.Sprintf("redis_client_id@%s@%s", opt.Addr, opt.Network)
	return id
}

func buildRedisClusterClientId(c *redis.ClusterClient) string {
	opt := c.Options()
	id := fmt.Sprintf("redis_cluster_client_id@%s", strings.Join(opt.Addrs, "#"))
	return id
}

func buildRedisCmdKey(id string, cmd redis.Cmder) string {
	var ss []string
	switch cmd.(type) {
	case *redis.StringCmd:
		ss = append(ss, "StringCmd@")
	case *redis.StatusCmd:
		ss = append(ss, "StatusCmd@")
	case *redis.IntCmd:
		ss = append(ss, "IntCmd@")
	case *redis.FloatCmd:
		ss = append(ss, "FloatCmd@")
	case *redis.BoolCmd:
		ss = append(ss, "BoolCmd@")
	case *redis.StringSliceCmd:
		ss = append(ss, "StringSliceCmd@")
	default:
		// panic("not supported redis cmd type")
		ss = append(ss, cmd.Name())
	}

	for _, arg := range cmd.Args() {
		ss = append(ss, fmt.Sprint(arg))
	}

	cs := strings.Join(ss, "@")
	key := fmt.Sprintf("%s@%s@%s", GlobalMgr.GetCurTraceId(), id, cs)
	return key
}

func saveRedisCmdValue(key string, cmd redis.Cmder) {
	var err error
	var buff bytes.Buffer

	switch c := cmd.(type) {
	case *redis.StringCmd:
		var val string
		val, err = c.Result()
		binary.Write(&buff, binary.LittleEndian, []byte(val))
	case *redis.StatusCmd:
		var val string
		val, err = c.Result()
		binary.Write(&buff, binary.LittleEndian, []byte(val))
	case *redis.IntCmd:
		var val int64
		val, err = c.Result()
		binary.Write(&buff, binary.LittleEndian, val)
	case *redis.FloatCmd:
		var val float64
		val, err = c.Result()
		binary.Write(&buff, binary.LittleEndian, val)
	case *redis.BoolCmd:
		var val bool
		val, err = c.Result()
		binary.Write(&buff, binary.LittleEndian, val)
	case *redis.StringSliceCmd:
		var val []string
		val, err = c.Result()
		binary.Write(&buff, binary.LittleEndian, int32(len(val)))
		for _, v := range val {
			binary.Write(&buff, binary.LittleEndian, int32(len(v)))
			binary.Write(&buff, binary.LittleEndian, []byte(v))
		}
	default:
		GlobalMgr.notifier("redis cmd recording for not-supported cmd", key, []byte(""))
		return
	}

	if err == nil || err == redis.Nil {
		GlobalMgr.StoreValue(key, buff.Bytes())
		GlobalMgr.notifier("redis cmd recording", key, buff.Bytes())
	} else {
		GlobalMgr.notifier("redis cmd not recording", fmt.Sprintf("cmd error occurs, key:%s, err:%s", key, err.Error()), nil)
	}
}

func addKeyToRedisCmd(cmd redis.Cmder, key string) {
	switch c := cmd.(type) {
	case *redis.StringCmd:
		arg := c.Args()
		arg[0] = key
	case *redis.StatusCmd:
		arg := c.Args()
		arg[0] = key
	case *redis.IntCmd:
		arg := c.Args()
		arg[0] = key
	case *redis.FloatCmd:
		arg := c.Args()
		arg[0] = key
	case *redis.BoolCmd:
		arg := c.Args()
		arg[0] = key
	case *redis.StringSliceCmd:
		arg := c.Args()
		arg[0] = key
	default:
		// panic("not supported redis cmd type")
		GlobalMgr.notifier("redis cmd replaying not supported cmd", key, []byte(""))
	}
}

/*
type RedisHook struct {
	id     string
	record bool
}

func fillRedisCmdResult(cmd redis.Cmder, val []byte) {
	// cmd cannot be modified, nothing we can do here.
	// the only solution is to overwrite cmd function for different type.
}

func (rh *RedisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if rh.record {
		return ctx, nil
	}

	// should replay recorded data

	key := buildRedisCmdKey(rh.id, cmd)

	// val, _ := GlobalMgr.GetValue(key)
	// fillRedisCmdResult(cmd, val)

	addKeyToRedisCmd(cmd, key)
	return ctx, errors.New("redis hook dumy errror")
}

func (rh *RedisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if !rh.record {
		return ctx, nil
	}

	key := buildRedisCmdKey(rh.id, cmd)
	saveRedisCmdValue(key, cmd)
	return ctx, nil
}

func NewRedisClusterClient(opt *redis.ClusterOptions) *redis.ClusterClient {
	c := NewRedisClusterClientTrampoline(opt)
	if c == nil {
		return c
	}

	id := buildRedisClusterClientId(c)
	h := &RedisHook{id: id, record: GlobalMgr.ShouldRecord()}
	c.AddHook(h)
	return c
}

//go:noinline
func NewRedisClusterClientTrampoline(opt *redis.ClusterOptions) *redis.ClusterClient {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if opt != nil {
		panic("trampoline redis NewClusterClient() function is not allowed to be called")
	}

	return nil
}
*/

func clientProcessWrapper(c *redis.Client, oldProcess func(cmd redis.Cmder) error) func(redis.Cmder) error {
	return func(cmd redis.Cmder) error {
		var err error

		id := buildRedisClientId(c)
		key := buildRedisCmdKey(id, cmd)

		GlobalMgr.notifier("calling client.ProcessWrapper", key, []byte(""))

		if GlobalMgr.ShouldRecord() {
			err = oldProcess(cmd)
			if err != nil && err != redis.Nil {
				GlobalMgr.notifier("redis Client.Process() wrapper recording failed", key, []byte(err.Error()))
				return err
			}
			saveRedisCmdValue(key, cmd)
		} else {
			addKeyToRedisCmd(cmd, key)
		}

		return err
	}
}

func WrapRedisClientProcess(c *redis.Client, fn func(func(redis.Cmder) error) func(redis.Cmder) error) bool {
	m := reflect.ValueOf(c).MethodByName("WrapProcess")
	if !m.IsNil() {
		m.Call([]reflect.Value{reflect.ValueOf(fn)})
		return true
	}

	return false
}

func NewRedisClient(opt *redis.Options) *redis.Client {
	c := NewRedisClientTrampoline(opt)
	if c == nil {
		return c
	}

	wrap := func(old func(cmd redis.Cmder) error) func(redis.Cmder) error {
		return clientProcessWrapper(c, old)
	}

	succ := WrapRedisClientProcess(c, wrap)
	if succ {
		GlobalMgr.notifier("call redis.WrapProcess for go redis < 6.15.4 done", "", []byte(""))
	} else {
		GlobalMgr.notifier("cannot call redis.WrapProcess()", "should not hook redis.NewClient()", []byte(""))
	}

	return c
}

//go:noinline
func NewRedisClientTrampoline(opt *redis.Options) *redis.Client {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if opt != nil {
		panic("trampoline redis NewClient() function is not allowed to be called")
	}

	return nil
}

func redisClientProcess(c *redis.Client, cmd redis.Cmder) error {
	var err error

	id := buildRedisClientId(c)
	key := buildRedisCmdKey(id, cmd)

	if GlobalMgr.ShouldRecord() {
		err = redisClientProcessTrampoline(c, cmd)
		if err != nil && err != redis.Nil {
			GlobalMgr.notifier("redis Client.Process() recording failed", key, []byte(err.Error()))
			return err
		}
		saveRedisCmdValue(key, cmd)
	} else {
		addKeyToRedisCmd(cmd, key)
	}

	return err
}

//go:nosplit
func redisClientProcessTrampoline(c *redis.Client, cmd redis.Cmder) error {
	fmt.Printf("dummy function for regrestion testing:%v", c)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		fmt.Printf("id:%d\n", 233)
		panic("trampoline redis redis.Client.Process() function is not allowed to be called")
	}

	return nil
}

func redisClusterClientProcess(c *redis.ClusterClient, cmd redis.Cmder) error {
	var err error

	id := buildRedisClusterClientId(c)
	key := buildRedisCmdKey(id, cmd)

	if GlobalMgr.ShouldRecord() {
		err = redisClusterClientProcessTrampoline(c, cmd)
		if err != nil && err != redis.Nil {
			GlobalMgr.notifier("redis ClusterClient.Process() recording failed", key, []byte(err.Error()))
			return err
		}
		// fmt.Printf("cluster client process recording, key:%s\n", key)
		saveRedisCmdValue(key, cmd)
	} else {
		// fmt.Printf("cluster client process replaying, key:%s\n", key)
		addKeyToRedisCmd(cmd, key)
	}

	return err
}

//go:nosplit
func redisClusterClientProcessTrampoline(c *redis.ClusterClient, cmd redis.Cmder) error {
	fmt.Printf("dummy function for regrestion testing:%v", c)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		fmt.Printf("id:%d\n", 111)
		panic("trampoline redis redis.ClusterClient.Process() function is not allowed to be called")
	}

	return nil
}

func getStoredValue(args []interface{}) ([]byte, error) {
	var err error
	var value []byte
	var key string
	var ok bool

	sz := len(args)

	for {
		if sz <= 0 {
			err = errors.New("invalid arg size, key unavailable")
			break
		}

		key, ok = args[0].(string)
		if !ok {
			err = errors.New("get key from args failed, invalid type")
			break
		}

		value, err = GlobalMgr.GetValue(key)
		break
	}

	if err == nil {
		GlobalMgr.notifier("redis cmd replaying done", key, value)
	} else {
		GlobalMgr.notifier("redis cmd replaying failed", key, []byte(err.Error()))
	}

	if err == nil && len(value) == 0 {
		return nil, redis.Nil
	}

	return value, err
}

// command hook
func statusCmdValue(cmd *redis.StatusCmd) string {
	value, err := getStoredValue(cmd.Args())
	if err != nil {
		return ""
	}
	return string(value)
}

func statusCmdResult(cmd *redis.StatusCmd) (string, error) {
	value, err := getStoredValue(cmd.Args())
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func stringCmdValue(cmd *redis.StringCmd) string {
	value, err := getStoredValue(cmd.Args())
	if err != nil {
		return ""
	}
	return string(value)
}

func stringSliceCmdValue(cmd *redis.StringSliceCmd) []string {
	ret, _ := stringSliceCmdResult(cmd)
	return ret
}

func stringSliceCmdResult(cmd *redis.StringSliceCmd) ([]string, error) {
	var ret []string
	value, err1 := getStoredValue(cmd.Args())
	if err1 != nil {
		return ret, errors.New("get value from db failed")
	}

	var buff bytes.Buffer
	sz, err2 := buff.Write(value)
	if err2 != nil || sz != len(value) {
		return ret, errors.New("write to buffer failed")
	}

	var slen int32
	err3 := binary.Read(&buff, binary.LittleEndian, &slen)
	if err3 != nil {
		return ret, errors.New("read size of slice from buffer failed")
	}

	sz -= 4

	for i := 0; i < int(slen); i++ {
		var sz2 int32
		err := binary.Read(&buff, binary.LittleEndian, &sz2)
		if err != nil || sz2 < 0 || int(sz2) > sz-4 {
			return ret, errors.New("read string size from buffer failed")
		}

		sz -= 4
		data := make([]byte, sz2)
		err = binary.Read(&buff, binary.LittleEndian, data)
		if err != nil {
			return ret, errors.New("read string from buffer failed")
		}

		sz -= int(sz2)
		ret = append(ret, string(data))
	}

	return ret, nil
}

func stringCmdResult(cmd *redis.StringCmd) (string, error) {
	value, err := getStoredValue(cmd.Args())
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func intCmdValue(cmd *redis.IntCmd) int64 {
	var buff bytes.Buffer
	value, err1 := getStoredValue(cmd.Args())

	if err1 != nil {
		// fmt.Printf("read int cmd from db failed, error:%s\n", err1.Error())
		return 0
	}

	sz, err2 := buff.Write(value)
	if err2 != nil || sz != len(value) {
		// fmt.Printf("read int cmd size failed, error:%s, sz:%d, len(value):%d\n", err2.Error(), sz, len(value))
		return 0.0
	}

	var ret int64
	err := binary.Read(&buff, binary.LittleEndian, &ret)
	if err != nil {
		// fmt.Printf("read int cmd failed, error:%s\n", err.Error())
		return 0
	}

	return ret
}

func floatCmdValue(cmd *redis.FloatCmd) float64 {
	var buff bytes.Buffer
	value, err1 := getStoredValue(cmd.Args())

	if err1 != nil {
		return 0.0
	}

	sz, err2 := buff.Write(value)
	if err2 != nil || sz != len(value) {
		return 0.0
	}

	var ret float64
	err := binary.Read(&buff, binary.LittleEndian, &ret)
	if err != nil {
		return 0.0
	}

	return ret
}

type CmdValue struct {
	cmd        interface{}
	fn         string
	replace    interface{}
	trampoline interface{}
}

var (
	cmd = []CmdValue{
		CmdValue{&redis.IntCmd{}, "Val", intCmdValue, nil},
		CmdValue{&redis.FloatCmd{}, "Val", floatCmdValue, nil},
		CmdValue{&redis.StringCmd{}, "Val", stringCmdValue, nil},
		CmdValue{&redis.StatusCmd{}, "Val", statusCmdValue, nil},
		CmdValue{&redis.StringSliceCmd{}, "Val", stringSliceCmdValue, nil},

		CmdValue{&redis.StringCmd{}, "Result", stringCmdResult, nil},
		CmdValue{&redis.StatusCmd{}, "Result", statusCmdResult, nil},
		CmdValue{&redis.StringSliceCmd{}, "Result", stringSliceCmdResult, nil},
	}
)

func UnHookRedisFunc() error {
	var c1 redis.Client
	var c2 redis.ClusterClient

	msg := ""
	for _, c := range cmd {
		v := reflect.ValueOf(c.cmd)
		err := gohook.UnHookMethod(v.Interface(), c.fn)
		if err != nil {
			msg += fmt.Sprintf("unhook %s() for %s failed, err:%s@@", c.fn, v.Type().Name(), err.Error())
		}
	}

	err1 := gohook.UnHookMethod(&c1, "Process")
	if err1 != nil {
		msg += fmt.Sprintf("unhook redis.Client.Process failed:%s@@", err1.Error())
	}

	_, exist := reflect.TypeOf(c1).MethodByName("WrapProcess")
	if exist {
		err11 := gohook.UnHook(redis.NewClient)
		if err11 != nil {
			msg += fmt.Sprintf("unhook redis.NewClient() failed:%s@@", err11.Error())
		}
	}

	err2 := gohook.UnHookMethod(&c2, "Process")
	if err2 != nil {
		msg += fmt.Sprintf("unhook redis.ClusterClient.Process failed:%s@@", err2.Error())
	}

	if msg != "" {
		return fmt.Errorf(msg)
	}

	return nil
}

func HookRedisFunc() error {
	// gohook.Hook(redis.NewClient, NewRedisClient, NewRedisClientTrampoline)
	// gohook.Hook(redis.NewClusterClient, NewRedisClusterClient, NewRedisClusterClientTrampoline)

	var err error
	var c1 redis.Client
	var c2 redis.ClusterClient

	defer func() {
		if err != nil {
			UnHookRedisFunc()
		}
	}()

	err = gohook.HookMethod(&c1, "Process", redisClientProcess, redisClientProcessTrampoline)
	if err != nil {
		return fmt.Errorf("hook redis.Client.Process() failed, err:%s", err.Error())
	}

	_, exist := reflect.TypeOf(&c1).MethodByName("WrapProcess")
	if exist {
		err = gohook.Hook(redis.NewClient, NewRedisClient, NewRedisClientTrampoline)
		if err != nil {
			return fmt.Errorf("hook redis.NewClient() failed, err:%s", err.Error())
		}

		GlobalMgr.notifier("go redis < 6.15.4, hook redis.NewClient() done", "", []byte(""))
	}

	err = gohook.HookMethod(&c2, "Process", redisClusterClientProcess, redisClusterClientProcessTrampoline)
	if err != nil {
		return fmt.Errorf("hook redis cluster client failed, err:%s", err.Error())
	}

	if !GlobalMgr.ShouldRecord() {
		// replay
		for _, c := range cmd {
			v := reflect.ValueOf(c.cmd)
			r := reflect.ValueOf(c.replace)
			t := reflect.ValueOf(c.trampoline)
			if c.trampoline == nil {
				err = gohook.HookMethod(v.Interface(), c.fn, r.Interface(), nil)
			} else {
				err = gohook.HookMethod(v.Interface(), c.fn, r.Interface(), t.Interface())
			}
			if err != nil {
				return fmt.Errorf("unhook %s() for %s failed, err:%s@@", c.fn, v.Type().Name(), err.Error())
			}
		}
	}

	return nil
}
