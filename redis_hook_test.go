package gorr

import (
	"context"
	"fmt"
	"github.com/brahma-adshonor/gohook"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"time"
)

var (
	redisIntValue         = 2334
	redisFloatValue       = 93.334
	redisStringValue      = "foo redis value"
	redisStringSliceValue = []string{"foo redis value", "foo redis value22", "miliao"}
)

func stringCmdHookVal(cmd *redis.StringCmd) string {
	fmt.Println("calling hook for testing StringCmd.Val()")
	return redisStringValue
}

func stringCmdHookResult(cmd *redis.StringCmd) (string, error) {
	fmt.Println("calling hook for testing StringCmd.Result()")
	return redisStringValue, nil
}

func statusCmdHookVal(cmd *redis.StatusCmd) string {
	fmt.Println("calling hook for testing StatusCmd.Val()")
	return redisStringValue
}

func statusCmdHookResult(cmd *redis.StatusCmd) (string, error) {
	fmt.Println("calling hook for testing StatusCmd.Result()")
	return redisStringValue, nil
}

func stringSliceCmdHookVal(cmd *redis.StringSliceCmd) []string {
	fmt.Println("calling hook for testing StringSliceCmd.Val()")
	return redisStringSliceValue
}

func stringSliceCmdHookResult(cmd *redis.StringSliceCmd) ([]string, error) {
	fmt.Println("calling hook for testing StringSliceCmd.Result()")
	return redisStringSliceValue, nil
}

func intCmdHookVal(cmd *redis.IntCmd) int64 {
	fmt.Println("calling hook for testing IntCmd.Val()")
	return int64(redisIntValue)
}

func intCmdHookResult(cmd *redis.IntCmd) (int64, error) {
	fmt.Println("calling hook for testing IntCmd.Result()")
	return int64(redisIntValue), nil
}

func floatCmdHookVal(cmd *redis.FloatCmd) float64 {
	fmt.Println("calling hook for testing FloatCmd.Val()")
	return float64(redisFloatValue)
}

func floatCmdHookResult(cmd *redis.FloatCmd) (float64, error) {
	fmt.Println("calling hook for testing FloatCmd.Result()")
	return float64(redisFloatValue), nil
}

func clientProcessTramplineHook(c *redis.Client, cmd redis.Cmder) error {
	fmt.Println("calling hook for testing client.Process()")
	return nil
}

func clusterClientProcessTramplineHook(c *redis.ClusterClient, cmd redis.Cmder) error {
	fmt.Println("calling hook for testing ClusterClient.Process()")
	return nil
}

func wrapRedisClientProcessHook(c *redis.Client, fn func(func(redis.Cmder) error) func(redis.Cmder) error) bool {
	// setup 'old' processor
	wrapRedisClientProcessTrampoline(c, func(func(redis.Cmder) error) func(redis.Cmder) error {
		return func(redis.Cmder) error {
			return nil
		}
	})

	// install 'new' processor
	return wrapRedisClientProcessTrampoline(c, fn)
}

//go:noinline
func wrapRedisClientProcessTrampoline(c *redis.Client, fn func(func(redis.Cmder) error) func(redis.Cmder) error) bool {
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline redis WrapClientProcess() function is not allowed to be called")
	}

	return true
}

func wrapPipelineProcessHook(c interface{}, fn func(func([]redis.Cmder) error) func([]redis.Cmder) error) bool {
	wrapPipelineProcessTrampoline(c, func(func([]redis.Cmder) error) func([]redis.Cmder) error {
		return func([]redis.Cmder) error {
			return nil
		}
	})

	return wrapPipelineProcessTrampoline(c, fn)
}

//go:noinline
func wrapPipelineProcessTrampoline(c interface{}, fn func(func([]redis.Cmder) error) func([]redis.Cmder) error) bool {
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline redis WrapClientProcess() function is not allowed to be called")
	}

	return true
}

func pipelineExecHook(*redis.Pipeline) ([]redis.Cmder, error) {
	ret := []redis.Cmder{redis.NewStringCmd("get", "1"), redis.NewStringCmd("get", "2")}
	return ret, nil
}

type redisTestHook struct {
	id string
}

func (rh *redisTestHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (rh *redisTestHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return nil
}

func (rh *redisTestHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	if GlobalMgr.ShouldRecord() {
		for _, cc := range cmds {
			key := buildRedisCmdKey(rh.id, cc)
			saveRedisCmdValue(key, cc)
		}
		return ctx, errRedisPipeNorm
	}
	return ctx, nil
}

func (rh *redisTestHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return nil
}

func setupRedisHook(t *testing.T) {
	enableRegressionEngine(RegressionRecord)

	GlobalMgr.SetState(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(100))

	GlobalMgr.SetNotify(func(t string, key string, value []byte) {
		fmt.Printf("gorr event, type:%s, key(%d):%s, value:%s", t, len(key), key, string(value))
		fmt.Printf("\n")
	})

	err1 := HookRedisFunc()
	if err1 != nil {
		fmt.Printf("hook redis failed, err:%s", err1.Error())
	}

	assert.Nil(t, err1)

	// hook trampoline, so that we can skip the real Process()
	err2 := gohook.Hook(redisClientProcessTrampoline, clientProcessTramplineHook, nil)
	if err2 != nil {
		fmt.Printf("hook redis client trampoline failed, err:%s", err2.Error())
	}
	assert.Nil(t, err2)

	err3 := gohook.Hook(wrapRedisClientProcess, wrapRedisClientProcessHook, wrapRedisClientProcessTrampoline)
	if err3 != nil {
		fmt.Printf("hook redis client trampoline failed, err:%s", err3.Error())
	}
	assert.Nil(t, err3)

	err4 := gohook.Hook(wrapRedisPipelineProcessor, wrapPipelineProcessHook, wrapPipelineProcessTrampoline)
	if err4 != nil {
		fmt.Printf("hook redis pipeline processor wrapper failed, err:%s", err4.Error())
	}
	assert.Nil(t, err4)

	fmt.Printf("debug:\n%s\n", GlobalMgr.GetDebugInfo())
}

func setupRedisClusterHook(t *testing.T) {
	enableRegressionEngine(RegressionRecord)
	GlobalMgr.SetState(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(100))

	err1 := HookRedisFunc()
	if err1 != nil {
		fmt.Printf("hook redis failed, err:%s", err1.Error())
	}

	assert.Nil(t, err1)

	// hook trampoline, so that we can skip the real Process()
	err2 := gohook.Hook(redisClusterClientProcessTrampoline, clusterClientProcessTramplineHook, nil)
	if err2 != nil {
		fmt.Printf("hook redis cluster client trampoline failed, err:%s", err2.Error())
	}

	assert.Nil(t, err2)

	err4 := gohook.Hook(wrapRedisPipelineProcessor, wrapPipelineProcessHook, wrapPipelineProcessTrampoline)
	if err4 != nil {
		fmt.Printf("hook redis pipeline processor wrapper failed, err:%s", err4.Error())
	}
	assert.Nil(t, err4)

	fmt.Printf("debug:\n%s\n", GlobalMgr.GetDebugInfo())
}

func TestStringCmd(t *testing.T) {
	setupRedisHook(t)

	defer func() {
		UnHookRedisFunc()
	}()

	var sc1 redis.StringCmd
	err3 := gohook.HookMethod(&sc1, "Val", stringCmdHookVal, nil)
	err32 := gohook.HookMethod(&sc1, "Result", stringCmdHookResult, nil)
	if err3 != nil || err32 != nil {
		fmt.Printf("hook redis StringCmd.Val failed, err:%s", err3.Error())
	}

	assert.Nil(t, err3)

	c := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.0:2333",
		DialTimeout: time.Duration(222) * time.Second,
		ReadTimeout: time.Duration(333) * time.Second,
	})

	assert.NotNil(t, c)

	c.Get("foo_the_bar")

	pp := c.Pipeline()

	if redisHasHook {
		m := reflect.ValueOf(c).MethodByName("AddHook")
		if !m.IsNil() {
			id := buildRedisClientId(c)
			m.Call([]reflect.Value{reflect.ValueOf(&redisTestHook{id: id})})
		}
	}

	fmt.Printf("@@@@@redis pipeline type:%v\n", pp)

	pp.Get("foo2")
	pp.Get("foo3")
	res, err22 := pp.Exec()
	assert.Nil(t, err22)
	assert.Equal(t, 2, len(res))

	v1, _ := res[0].(*redis.StringCmd).Result()
	v2, _ := res[1].(*redis.StringCmd).Result()

	assert.Equal(t, redisStringValue, v1)
	assert.Equal(t, redisStringValue, v2)

	redisStoredString := redisStringValue
	redisStringValue = "dummy"
	gohook.UnHookMethod(&sc1, "Val")
	gohook.UnHookMethod(&sc1, "Result")
	gohook.UnHook(redisClientProcessTrampoline)
	UnHookRedisFunc()

	fmt.Printf("done recording StringCmd, now start to replay\n")

	GlobalMgr.SetState(RegressionReplay)
	err4 := HookRedisFunc()
	if err4 != nil {
		fmt.Printf("hook redis failed, err:%s", err4.Error())
	}

	assert.Nil(t, err4)

	fmt.Printf("debug replay:\n%s\n", GlobalMgr.GetDebugInfo())

	c2 := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.0:2333",
		DialTimeout: time.Duration(222) * time.Second,
		ReadTimeout: time.Duration(333) * time.Second,
	})

	cmd1 := c2.Get("foo_the_bar")
	val1 := cmd1.Val()
	val2, err5 := cmd1.Result()
	assert.Nil(t, err5)

	assert.Equal(t, redisStoredString, val1)
	assert.Equal(t, redisStoredString, val2)

	{
		pp2 := c2.Pipeline()
		if redisHasHook {
			m := reflect.ValueOf(c2).MethodByName("AddHook")
			if !m.IsNil() {
				id := buildRedisClientId(c2)
				m.Call([]reflect.Value{reflect.ValueOf(&redisTestHook{id: id})})
			}
		}

		pp2.Get("foo2")
		pp2.Get("foo3")

		res2, err23 := pp2.Exec()
		assert.Nil(t, err23)
		assert.Equal(t, 2, len(res2))

		v21, _ := res2[0].(*redis.StringCmd).Result()
		v22, _ := res2[1].(*redis.StringCmd).Result()

		assert.Equal(t, v1, v21)
		assert.Equal(t, v2, v22)
	}

	redisStringValue = redisStoredString
}

func TestIntCmd(t *testing.T) {
	fmt.Printf("@@@@@start testing IntCmd\n")

	setupRedisHook(t)

	defer func() {
		UnHookRedisFunc()
	}()

	var ic1 redis.IntCmd
	err3 := gohook.HookMethod(&ic1, "Val", intCmdHookVal, nil)
	err32 := gohook.HookMethod(&ic1, "Result", intCmdHookResult, nil)
	if err3 != nil || err32 != nil {
		fmt.Printf("hook redis IntCmd.Val failed, err:%s", err3.Error())
	}

	assert.Nil(t, err3)

	c := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.0:2334",
		DialTimeout: time.Duration(222) * time.Second,
		ReadTimeout: time.Duration(333) * time.Second,
	})

	assert.NotNil(t, c)

	c.BitCount("foo_the_bar_int", &redis.BitCount{Start: 2, End: 200})

	redisStoredInt := redisIntValue
	redisIntValue = 4444

	gohook.UnHookMethod(&ic1, "Val")
	gohook.UnHookMethod(&ic1, "Result")
	gohook.UnHook(redisClientProcessTrampoline)
	UnHookRedisFunc()

	fmt.Printf("@@@@@now to start replay\n")
	GlobalMgr.SetState(RegressionReplay)
	err4 := HookRedisFunc()
	if err4 != nil {
		fmt.Printf("hook redis failed, err:%s", err4.Error())
	}

	assert.Nil(t, err4)

	cmd1 := c.BitCount("foo_the_bar_int", &redis.BitCount{Start: 2, End: 200})
	val1 := cmd1.Val()

	assert.Equal(t, int64(redisStoredInt), val1)

	redisIntValue = redisStoredInt
}

func TestStatusCmd(t *testing.T) {
	setupRedisClusterHook(t)

	defer func() {
		UnHookRedisFunc()
	}()

	var sc1 redis.StatusCmd
	err3 := gohook.HookMethod(&sc1, "Val", statusCmdHookVal, nil)
	err32 := gohook.HookMethod(&sc1, "Result", statusCmdHookResult, nil)
	if err3 != nil || err32 != nil {
		fmt.Printf("hook redis StatusCmd.Val failed, err:%s", err3.Error())
	}

	assert.Nil(t, err3)

	c := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:       []string{"127.0.0.0:2333", "127.0.0.1:2333", "127.0.0.2:2333"},
		Password:    "dummy passwd",
		DialTimeout: time.Duration(12222) * time.Millisecond,
		ReadTimeout: time.Duration(44444) * time.Millisecond,
		PoolSize:    128,
	})

	assert.NotNil(t, c)

	c.Ping()

	pp := c.Pipeline()

	if redisHasHook {
		m := reflect.ValueOf(c).MethodByName("AddHook")
		if !m.IsNil() {
			id := buildRedisClusterClientId(c)
			m.Call([]reflect.Value{reflect.ValueOf(&redisTestHook{id: id})})
		}
	}

	fmt.Printf("@@@@@redis pipeline type:%v\n", pp)

	pp.Type("foo2")
	pp.Type("foo3")
	res, err22 := pp.Exec()
	assert.Nil(t, err22)
	assert.Equal(t, 2, len(res))

	v1, _ := res[0].(*redis.StatusCmd).Result()
	v2, _ := res[1].(*redis.StatusCmd).Result()

	assert.Equal(t, redisStringValue, v1)
	assert.Equal(t, redisStringValue, v2)

	redisStoredString := redisStringValue
	redisStringValue = "dummy"
	gohook.UnHookMethod(&sc1, "Val")
	gohook.UnHookMethod(&sc1, "Result")
	gohook.UnHook(redisClusterClientProcessTrampoline)
	UnHookRedisFunc()

	GlobalMgr.SetState(RegressionReplay)
	err4 := HookRedisFunc()
	if err4 != nil {
		fmt.Printf("hook redis failed, err:%s", err4.Error())
	}

	assert.Nil(t, err4)

	cmd1 := c.Ping()
	val1 := cmd1.Val()
	val2, err5 := cmd1.Result()
	assert.Nil(t, err5)

	assert.Equal(t, redisStoredString, val1)
	assert.Equal(t, redisStoredString, val2)

	{
		pp2 := c.Pipeline()

		pp2.Type("foo2")
		pp2.Type("foo3")

		res2, err23 := pp2.Exec()
		assert.Nil(t, err23)
		assert.Equal(t, 2, len(res2))

		v21, _ := res2[0].(*redis.StatusCmd).Result()
		v22, _ := res2[1].(*redis.StatusCmd).Result()

		assert.Equal(t, v1, v21)
		assert.Equal(t, v2, v22)
	}

	redisStringValue = redisStoredString
}

func TestFloatCmd(t *testing.T) {
	setupRedisClusterHook(t)

	defer func() {
		UnHookRedisFunc()
	}()

	var sc1 redis.FloatCmd
	err3 := gohook.HookMethod(&sc1, "Val", floatCmdHookVal, nil)
	err32 := gohook.HookMethod(&sc1, "Result", floatCmdHookResult, nil)
	if err3 != nil || err32 != nil {
		fmt.Printf("hook redis FloatCmd.Val failed, err:%s", err3.Error())
	}

	assert.Nil(t, err3)

	c := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:       []string{"127.0.0.0:2334", "127.0.0.1:2334", "127.0.0.2:2334"},
		Password:    "dummy passwd",
		DialTimeout: time.Duration(12222) * time.Millisecond,
		ReadTimeout: time.Duration(44444) * time.Millisecond,
		PoolSize:    128,
	})

	assert.NotNil(t, c)

	c.IncrByFloat("foo_the_bar_float", 2.33)

	redisStoredFloat := redisFloatValue
	redisFloatValue = 23.333
	gohook.UnHookMethod(&sc1, "Val")
	gohook.UnHookMethod(&sc1, "Result")
	gohook.UnHook(redisClusterClientProcessTrampoline)
	UnHookRedisFunc()

	GlobalMgr.SetState(RegressionReplay)
	err4 := HookRedisFunc()
	if err4 != nil {
		fmt.Printf("hook redis failed, err:%s", err4.Error())
	}

	assert.Nil(t, err4)

	cmd1 := c.IncrByFloat("foo_the_bar_float", 2.33)
	val1 := cmd1.Val()
	val2, err5 := cmd1.Result()
	assert.Nil(t, err5)

	assert.Equal(t, redisStoredFloat, val1)
	assert.Equal(t, redisStoredFloat, val2)

	redisFloatValue = redisStoredFloat
}

func TestStringSliceCmd(t *testing.T) {
	setupRedisClusterHook(t)

	defer func() {
		UnHookRedisFunc()
	}()

	var sc redis.StringSliceCmd
	err := gohook.HookMethod(&sc, "Val", stringSliceCmdHookVal, nil)
	err22 := gohook.HookMethod(&sc, "Result", stringSliceCmdHookResult, nil)
	if err != nil || err22 != nil {
		fmt.Printf("hook redis StringSliceCmd.Val failed, err:%s", err.Error())
	}

	assert.Nil(t, err)

	c := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:       []string{"127.0.0.0:2333", "127.0.0.1:2333", "127.0.0.2:2333"},
		Password:    "dummy passwd",
		DialTimeout: time.Duration(12222) * time.Millisecond,
		ReadTimeout: time.Duration(44444) * time.Millisecond,
		PoolSize:    128,
	})

	assert.NotNil(t, c)

	c.ZRevRange("miliao-zrange", 23, 1024)

	redisStringSliceValueStored := redisStringSliceValue
	redisStringSliceValue = []string{"abc", "cd"}
	gohook.UnHookMethod(&sc, "Val")
	gohook.UnHookMethod(&sc, "Result")

	gohook.UnHook(redisClusterClientProcessTrampoline)
	UnHookRedisFunc()

	GlobalMgr.SetState(RegressionReplay)
	err4 := HookRedisFunc()
	if err4 != nil {
		fmt.Printf("hook redis failed, err:%s", err4.Error())
	}

	assert.Nil(t, err4)
	cmd := c.ZRevRange("miliao-zrange", 23, 1024)

	ret := cmd.Val()

	assert.Equal(t, redisStringSliceValueStored, ret)
	redisStringSliceValue = redisStringSliceValueStored
}
