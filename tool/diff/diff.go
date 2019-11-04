package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	//"github.com/golang/protobuf/jsonpb"
	//"github.com/golang/protobuf/proto"
	//"github.com/golang/protobuf/ptypes/struct"
	"github.com/pmezard/go-difflib/difflib"
	"io/ioutil"
	"os"
)

const (
	recorderDataTypeUnknown  = 23
	recorderDataTypeJSON     = 24
	recorderDataTypePbText   = 25
	recorderDataTypePbBinary = 26
)

var (
	expect = flag.String("expect", "", "expected data")
	actual = flag.String("actual", "", "actual data")
	dType  = flag.Int("type", recorderDataTypeJSON, "data type for diff")
)

func doDiff(dt int, ef, af string) (string, error) {
	var err error
	var diff string
	var epData, atData []byte

	epData, err = ioutil.ReadFile(ef)
	if err != nil {
		return "", fmt.Errorf("read expect data failed, file:%s, err: %s", ef, err)
	}

	atData, err = ioutil.ReadFile(af)
	if err != nil {
		return "", fmt.Errorf("read actual data failed, file:%s, err: %s", af, err)
	}

	rawDiff := false
	if dt == recorderDataTypeJSON {
		diff, err = diffJSON(epData, atData)
		if err != nil {
			rawDiff = true
		}
	} else if dt == recorderDataTypePbBinary {
		diff, err = diffProtobuf(epData, atData)
		if err != nil {
			rawDiff = true
		}
	} else {
		rawDiff = true
	}

	if rawDiff {
		if !bytes.Equal(epData, atData) {
			diff, err = difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(epData)),
				B:        difflib.SplitLines(string(atData)),
				FromFile: "Expected",
				FromDate: "",
				ToFile:   "Actual",
				ToDate:   "",
				Context:  1,
			})

			if err != nil {
				diff = fmt.Sprintf("failed to perform raw diff, err:%s", err)
			}
		}
	}

	if len(diff) > 0 {
		return fmt.Sprintf("diff output:\n%s\n", diff), nil
	}

	return "", nil
}

func diffProtobuf(ep, at []byte) (string, error) {
	// NOT working
	panic("pb binary is not supported")
}

func diffJSON(ep, at []byte) (string, error) {
	var ep1, at1 map[string]interface{}

	err1 := json.Unmarshal(ep, &ep1)
	if err1 != nil {
		return "", fmt.Errorf("unmarshal expect data failed, err:%s", err1)
	}

	err2 := json.Unmarshal(at, &at1)
	if err2 != nil {
		return "", fmt.Errorf("unmarshal actual data failed, err:%s", err2)
	}

	d, err := diffAny("", ep1, at1)
	if err != nil {
		return "", fmt.Errorf("diff json map failed, err:%s", err)
	}

	if d == nil {
		return "", nil
	}

	diff := d.String(2)
	if len(diff) == 0 {
		return "", nil
	}

	return "--- Expected\n+++ Actual\n@@@@@@@@@@@@@@@@@\n" + diff, nil
}

func diffJSONText(ep, at []byte) (string, error) {
	var ep1, at1 map[string]interface{}

	err1 := json.Unmarshal(ep, &ep1)
	if err1 != nil {
		return "", fmt.Errorf("unmarshal expect data failed, err:%s", err1)
	}

	err2 := json.Unmarshal(at, &at1)
	if err2 != nil {
		return "", fmt.Errorf("unmarshal actual data failed, err:%s", err2)
	}

	var d1, d2 []byte

	d1, err1 = json.MarshalIndent(ep1, "", "\t")
	if err1 != nil {
		return "", fmt.Errorf("marshal expect data failed, err:%s", err1)
	}

	d2, err2 = json.MarshalIndent(at1, "", "\t")
	if err2 != nil {
		return "", fmt.Errorf("marshal actual data failed, err:%s", err2)
	}

	var err error
	var diff string
	if !bytes.Equal(d1, d2) {
		diff, err = difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(d1)),
			B:        difflib.SplitLines(string(d2)),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})

		if err != nil {
			return "", fmt.Errorf("failed to perform raw diff, err:%s", err)
		}
	}

	return diff, nil
}

func main() {
	flag.Parse()
	diff, err := doDiff(*dType, *expect, *actual)
	if err != nil {
		fmt.Printf("failed to perform diff, err:%s\n", err)
		os.Exit(23)
	}

	if len(diff) > 0 {
		fmt.Printf("%s", diff)
		os.Exit(250)
	}

	os.Exit(0)
}
