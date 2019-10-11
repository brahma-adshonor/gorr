package gorr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-redis/redis"
)

var (
	redisHasHook        = false
	redisHasProcessWrap = true
	errRedisPipeNorm    = fmt.Errorf("redis dummy error for pipelinek")
)

func init() {
	var c *redis.Client
	_, redisHasHook = reflect.TypeOf(c).MethodByName("AddHook")
	_, redisHasProcessWrap = reflect.TypeOf(c).MethodByName("WrapProcess")
}

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
