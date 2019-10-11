package gorr

import (
	"fmt"
	"reflect"

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
	=>due to the subtle implementation of redis.Client.Process() before 6.15.1(Client.Process() not exist, only baseClient.Process, which can not be hooked), hook for client.Process won't work,
	 instead, we add wrapper to client.Process() by calling client.WrapProcess(), which does not present in later version, yep, it is tricky.

Redis command object(StringCmd, IntCmd, etc) is immutable, we are not able to set result into it, this makes a lot of troubles,
the workaround for this is we are going to hook the getter of the object. and maybe we should make a pull request to upstream, adding setter api to redis Command object.
*/

/*
hook point:
1. Client.Process()/ClusterClient.Process: for recoding/replaying cmd
2. Client.WrapProcess(): for go redis < 6.15.1, Client.Process() is not available.
3. NewClient()/NewRedisClient(): used to call WrapProcess()/AdHook() on client objects.
4. IntCmd/StringCmd/FloatCmd/SliceCmd/StatusCmd/etc: hook Result()/Val() method.
5. Client.WrapPipelineProcess()/ClusterClient.WrapPipelineProcess(): for go redis < 6.15.1, used to intercept queued cmds.
6. hook added by Client.AddHook()/ClusterClient.AdHook(): for go redis > 6.15.1, used to intercept pipeline cmd.
7. Pipeline.Exec()/Pipeline.ExecContext(): used to ignore dummy error from hook added by AdHook()
*/

// client.Process wrapper
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

func wrapRedisClientProcess(c *redis.Client, fn func(func(redis.Cmder) error) func(redis.Cmder) error) bool {
	m := reflect.ValueOf(c).MethodByName("WrapProcess")
	if !m.IsNil() {
		m.Call([]reflect.Value{reflect.ValueOf(fn)})
		return true
	}

	return false
}

func newRedisClient(opt *redis.Options) *redis.Client {
	c := newRedisClientTrampoline(opt)
	if c == nil {
		return c
	}

	if redisHasProcessWrap {
		wrap := func(old func(cmd redis.Cmder) error) func(redis.Cmder) error {
			return clientProcessWrapper(c, old)
		}

		succ := wrapRedisClientProcess(c, wrap)
		if succ {
			GlobalMgr.notifier("call redis.WrapProcess for go redis < 6.15.4 done", "", []byte(""))
		} else {
			GlobalMgr.notifier("cannot call redis.WrapProcess()", "should not hook redis.NewClient()", []byte(""))
		}

		wrap2 := func(old func([]redis.Cmder) error) func([]redis.Cmder) error {
			return clientPipelineProcessWrapper(c, old)
		}

		succ2 := wrapRedisPipelineProcessor(c, wrap2)
		if succ2 {
			GlobalMgr.notifier("call redis.WrapProcessPipeline for go redis < 6.15.4 done", "", []byte(""))
		} else {
			GlobalMgr.notifier("cannot call redis.WrapProcessPipeline()", "should not hook redis.NewClient()", []byte(""))
		}
	}

	if redisHasHook {
		m := reflect.ValueOf(c).MethodByName("AddHook")
		if !m.IsNil() {
			id := buildRedisClientId(c)
			m.Call([]reflect.Value{reflect.ValueOf(&redisHook{id: id})})
		}
	}

	return c
}

//go:noinline
func newRedisClientTrampoline(opt *redis.Options) *redis.Client {
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

func newRedisClusterClient(opt *redis.ClusterOptions) *redis.ClusterClient {
	c := newRedisClusterClientTrampoline(opt)
	if c == nil {
		return c
	}

	if redisHasProcessWrap {
		wrap2 := func(old func([]redis.Cmder) error) func([]redis.Cmder) error {
			return clusterClientPipelineProcessWrapper(c, old)
		}

		succ2 := wrapRedisPipelineProcessor(c, wrap2)
		if succ2 {
			GlobalMgr.notifier("call redis.WrapProcessPipeline for go redis < 6.15.4 done", "", []byte(""))
		} else {
			GlobalMgr.notifier("cannot call redis.WrapProcessPipeline()", "should not hook redis.NewClient()", []byte(""))
		}
	}

	if redisHasHook {
		m := reflect.ValueOf(c).MethodByName("AddHook")
		if !m.IsNil() {
			id := buildRedisClusterClientId(c)
			m.Call([]reflect.Value{reflect.ValueOf(&redisHook{id: id})})
		}
	}

	return c
}

//go:noinline
func newRedisClusterClientTrampoline(opt *redis.ClusterOptions) *redis.ClusterClient {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%v", opt)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if opt != nil {
		panic("trampoline redis NewClient() function is not allowed to be called")
	}

	return nil
}

// redis.Client.Process() hook
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
		saveRedisCmdValue(key, cmd)
	} else {
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

	var pl redis.Pipeline

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

	err12 := gohook.UnHook(redis.NewClient)
	if err12 != nil {
		msg += fmt.Sprintf("unhook redis.NewClient() failed:%s@@", err12.Error())
	}

	err13 := gohook.UnHook(redis.NewClusterClient)
	if err13 != nil {
		msg += fmt.Sprintf("unhook redis.NewClusterClient() failed:%s@@", err13.Error())
	}

	err2 := gohook.UnHookMethod(&c2, "Process")
	if err2 != nil {
		msg += fmt.Sprintf("unhook redis.ClusterClient.Process failed:%s@@", err2.Error())
	}

	if redisHasHook {
		err := gohook.UnHookMethod(&pl, "Exec")
		if err != nil {
			msg += fmt.Sprintf("unhook redis pipeline.Exec() failed:%s@@", err.Error())
		}

		err = gohook.UnHookMethod(&pl, "ExecContext")
		if err != nil {
			msg += fmt.Sprintf("unhook redis pipeline.ExecContext() failed:%s@@", err.Error())
		}
	}

	if msg != "" {
		return fmt.Errorf(msg)
	}

	return nil
}

func HookRedisFunc() error {
	var err error
	var c1 redis.Client
	var c2 redis.ClusterClient

	var pl redis.Pipeline

	defer func() {
		if err != nil {
			UnHookRedisFunc()
		}
	}()

	err = gohook.HookMethod(&c1, "Process", redisClientProcess, redisClientProcessTrampoline)
	if err != nil {
		return fmt.Errorf("hook redis.Client.Process() failed, err:%s", err.Error())
	}

	err = gohook.Hook(redis.NewClient, newRedisClient, newRedisClientTrampoline)
	if err != nil {
		GlobalMgr.notifier("hook redis.NewClient() failed", err.Error(), []byte(""))
		return fmt.Errorf("hook redis.NewClient() failed, err:%s", err.Error())
	}

	err = gohook.Hook(redis.NewClusterClient, newRedisClusterClient, newRedisClusterClientTrampoline)
	if err != nil {
		GlobalMgr.notifier("hook redis.NewClusterClient() failed", err.Error(), []byte(""))
		return fmt.Errorf("hook redis.NewClusterClient() failed, err:%s", err.Error())
	}

	err = gohook.HookMethod(&c2, "Process", redisClusterClientProcess, redisClusterClientProcessTrampoline)
	if err != nil {
		return fmt.Errorf("hook redis cluster client failed, err:%s", err.Error())
	}

	if redisHasHook {
		err = gohook.HookMethod(&pl, "Exec", redisPipelineExec, redisPipelineExecTramp)
		if err != nil {
			return fmt.Errorf("hook redis pipeline.Exec() failed, err:%s", err.Error())
		}
		err = gohook.HookMethod(&pl, "ExecContext", redisPipelineExecContext, redisPipelineExecContextTramp)
		if err != nil {
			return fmt.Errorf("hook redis pipeline.Exec() failed, err:%s", err.Error())
		}
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
