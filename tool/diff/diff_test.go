package main

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

type diffType2 struct {
	D3 int    `json:"d1"`
	D4 int    `json:"d2"`
	N2 string `json:"name"`
}

type diffType struct {
	D1 int       `json:"d1"`
	D2 int       `json:"d2"`
	N1 string    `json:"name"`
	Dt diffType2 `json:"dtype"`
	Ar []string  `json:"sar"`
}

func TestJSONDiff(t *testing.T) {
	s1 := diffType{
		D1: 23,
		D2: 42,
		N1: "miliao",
		Dt: diffType2{
			D3: 230,
			D4: 420,
			N2: "miliao2",
		},
	}

	s2 := diffType{
		D1: 23,
		D2: 42,
		N1: "miliao",
		Dt: diffType2{
			D3: 230,
			D4: 420,
			N2: "miliao2",
		},
	}

	s3 := diffType{
		D1: 23,
		D2: 42,
		N1: "change",
		Dt: diffType2{
			D3: 230,
			D4: 420,
			N2: "miliao2",
		},
	}

	s4 := diffType{
		D1: 23,
		D2: 42,
		Ar: []string{},
		Dt: diffType2{
			D3: 230,
			D4: 420,
			N2: "miliao2",
		},
	}

	s5 := diffType{
		D1: 23,
		D2: 42,
		Dt: diffType2{
			D3: 230,
			D4: 420,
			N2: "miliao2",
		},
	}

	var err error
	var d1, d2, d3, d4, d5 []byte

	d1, err = json.Marshal(s1)
	assert.Nil(t, err)

	d2, err = json.Marshal(s2)
	assert.Nil(t, err)

	d3, err = json.Marshal(s3)
	assert.Nil(t, err)

	d4, err = json.Marshal(s4)
	assert.Nil(t, err)
	d5, err = json.Marshal(s5)
	assert.Nil(t, err)

	f1 := "./d1.txt"
	err = ioutil.WriteFile(f1, d1, 0666)
	assert.Nil(t, err)

	f2 := "./d2.txt"
	err = ioutil.WriteFile(f2, d2, 0666)
	assert.Nil(t, err)

	f3 := "./d3.txt"
	err = ioutil.WriteFile(f3, d3, 0666)
	assert.Nil(t, err)

	f4 := "./d4.txt"
	err = ioutil.WriteFile(f4, d4, 0666)
	assert.Nil(t, err)

	f5 := "./d5.txt"
	err = ioutil.WriteFile(f5, d5, 0666)
	assert.Nil(t, err)

	defer func() {
		os.Remove(f1)
		os.Remove(f2)
		os.Remove(f3)
		os.Remove(f4)
		os.Remove(f5)
	}()

	var diff string
	diff, err = doDiff(recorderDataTypeJSON, f1, f2)
	assert.Nil(t, err)
	assert.Equal(t, "", diff)

	diff, err = doDiff(recorderDataTypeJSON, f1, f3)
	assert.Nil(t, err)
	assert.NotEqual(t, "", diff)
	fmt.Printf("json diff from doDiff:\n%s\n", diff)

	diff, err = doDiff(recorderDataTypeJSON, f4, f5)
	assert.Nil(t, err)
	assert.Equal(t, "", diff)
}

type dt struct {
	iv  int
	sv  string
	itv []interface{}
	mtv map[string]interface{}
}

func TestDiffMapStruct(t *testing.T) {
	d1 := map[string]interface{}{
		"iv":  23,
		"sv":  "miliao",
		"itv": []interface{}{42, "mm", map[string]interface{}{"kv1": 111, "kv2": "vvvv"}},
		"mtv": map[string]interface{}{"mv1": 2222, "mv2": "vvv2", "sl": []interface{}{"s1", 5555}},
	}

	d2 := map[string]interface{}{
		"iv":  234,
		"itv": []interface{}{432, map[string]interface{}{"kv1": 111, "kv2": "vvvv"}},
		"mtv": map[string]interface{}{"mv1": 2222, "mv2": "vvv2", "sl": []interface{}{"s1", 5555}},
	}

	d3 := map[string]interface{}{
		"iv":  234,
		"itv": []interface{}{432, map[string]interface{}{"kv1": 111, "kv2": "vvvv"}},
		"mtv": map[string]interface{}{"mv1": 2222, "mv2": "vvv2", "sl": []interface{}{"s1", 5555}},
	}

	n, err := diffSingle(1, "", d1)
	assert.Nil(t, err)

	diff := n.String(2)
	assert.NotEqual(t, "", diff)

	fmt.Printf("diff of single:\n%s\n", diff)

	n, err = diffMap("", d1, d2)
	assert.Nil(t, err)

	diff = n.String(2)
	assert.NotEqual(t, "", diff)
	fmt.Printf("diff of map:\n%s\n", diff)

	n2, err2 := diffAny("", d1, d2)
	assert.Nil(t, err2)

	diff2 := n2.String(2)
	assert.NotEqual(t, "", diff2)
	fmt.Printf("diff of any:\n%s\n", diff2)

	assert.Equal(t, diff, diff2)

	n3, err3 := diffAny("", d2, d3)
	assert.Nil(t, err3)

	assert.Nil(t, n3)
}
