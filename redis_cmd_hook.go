package gorr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/go-redis/redis"
)

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
