package gorr

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-redis/redis"
)

// handle redis pipeline
type redisHook struct {
	id string
}

func (rh *redisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (rh *redisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return nil
}

func (rh *redisHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	if !GlobalMgr.ShouldRecord() {
		GlobalMgr.notifier("calling redisHook.BeforeProcessPipeline for replaying\n", rh.id, []byte(""))
		for _, cc := range cmds {
			key := buildRedisCmdKey(rh.id, cc)
			addKeyToRedisCmd(cc, key)
		}
		return ctx, errRedisPipeNorm
	}

	return ctx, nil
}

func (rh *redisHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	if GlobalMgr.ShouldRecord() {
		GlobalMgr.notifier("calling redisHook.AfterProcessPipeline for recording\n", rh.id, []byte(""))
		for _, cc := range cmds {
			key := buildRedisCmdKey(rh.id, cc)
			saveRedisCmdValue(key, cc)
		}
		return errRedisPipeNorm
	}

	return nil
}

func redisPipelineExec(p *redis.Pipeline) ([]redis.Cmder, error) {
	r, err := redisPipelineExecTramp(p)
	if err == errRedisPipeNorm {
		return r, nil
	}
	return r, err
}

func redisPipelineExecTramp(p *redis.Pipeline) ([]redis.Cmder, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%v", p)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if p != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func redisPipelineExecContext(p *redis.Pipeline, ctx context.Context) ([]redis.Cmder, error) {
	r, err := redisPipelineExecContextTramp(p, ctx)
	if err == errRedisPipeNorm {
		return r, nil
	}
	return r, err
}

func redisPipelineExecContextTramp(p *redis.Pipeline, ctx context.Context) ([]redis.Cmder, error) {
	fmt.Printf("dummy function for regrestion testing:%v", p)
	fmt.Printf("dummy function for regrestion testing:%v", ctx)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if p != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func redisPipelineProcessor(id string, cmd []redis.Cmder, old func(cmd []redis.Cmder) error) error {
	cs := ""
	var err error

	for _, cc := range cmd {
		cs = buildRedisCmdKey(cs, cc)
	}

	GlobalMgr.notifier("calling client.Pipeline.ProcessWrapper", cs, []byte(""))

	if GlobalMgr.ShouldRecord() {
		err = old(cmd)
		if err != nil && err != redis.Nil {
			GlobalMgr.notifier("redis Client.Pipeline.ProcessWrapper() recording failed", cs, []byte(err.Error()))
			return err
		}

		for _, cc := range cmd {
			key := buildRedisCmdKey(id, cc)
			saveRedisCmdValue(key, cc)
		}
	} else {
		for _, cc := range cmd {
			key := buildRedisCmdKey(id, cc)
			addKeyToRedisCmd(cc, key)
		}
	}

	return err
}

// client.Pipeline.Process() wrapper
func clientPipelineProcessWrapper(c *redis.Client, oldProcess func(cmd []redis.Cmder) error) func([]redis.Cmder) error {
	return func(cmd []redis.Cmder) error {
		id := buildRedisClientId(c)
		return redisPipelineProcessor(id, cmd, oldProcess)
	}
}

func clusterClientPipelineProcessWrapper(c *redis.ClusterClient, oldProcess func(cmd []redis.Cmder) error) func([]redis.Cmder) error {
	return func(cmd []redis.Cmder) error {
		id := buildRedisClusterClientId(c)
		return redisPipelineProcessor(id, cmd, oldProcess)
	}
}

func wrapRedisPipelineProcessor(c interface{}, fn func(func([]redis.Cmder) error) func([]redis.Cmder) error) bool {
	m := reflect.ValueOf(c).MethodByName("WrapProcessPipeline")
	if !m.IsNil() {
		m.Call([]reflect.Value{reflect.ValueOf(fn)})
		return true
	}

	return false
}
