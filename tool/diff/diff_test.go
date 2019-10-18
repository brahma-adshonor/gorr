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

	var err error
	var d1, d2, d3 []byte

	d1, err = json.Marshal(s1)
	assert.Nil(t, err)

	d2, err = json.Marshal(s2)
	assert.Nil(t, err)

	d3, err = json.Marshal(s3)
	assert.Nil(t, err)

	f1 := "./d1.txt"
	err = ioutil.WriteFile(f1, d1, 0666)
	assert.Nil(t, err)
	defer os.Remove(f1)

	f2 := "./d2.txt"
	err = ioutil.WriteFile(f2, d2, 0666)
	assert.Nil(t, err)
	defer os.Remove(f2)

	f3 := "./d3.txt"
	err = ioutil.WriteFile(f3, d3, 0666)
	assert.Nil(t, err)
	defer os.Remove(f3)

	var diff string
	diff, err = doDiff(recorderDataTypeJSON, f1, f2)
	assert.Nil(t, err)
	assert.Equal(t, "", diff)

	diff, err = doDiff(recorderDataTypeJSON, f1, f3)
	assert.Nil(t, err)
	assert.NotEqual(t, "", diff)

	fmt.Printf("diff from doDiff:\n%s\n", diff)
}
