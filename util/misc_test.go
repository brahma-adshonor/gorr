package util

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestRunCmd(t *testing.T) {
	out, err := RunCmd("pwd")
	assert.Nil(t, err)

	pwd, _ := os.Getwd()
	pwd += "\n"
	assert.Equal(t, pwd, string(out))

	out2, err2 := RunCmd("echo miliao")
	assert.Nil(t, err2)
	assert.Equal(t, "miliao\n", string(out2))
}

func TestCopyFile(t *testing.T) {
	src := "testdata/copy.file"
	dst := "testdata/copy.file.test"

	err := CopyFile(src, dst)
	assert.Nil(t, err)
	defer os.Remove(dst)

	d1, err1 := ioutil.ReadFile(src)
	d2, err2 := ioutil.ReadFile(dst)

	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, d1, d2)
}

