package regression

import (
	//"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"runtime"
	"testing"
)

func TestBoltDb(t *testing.T) {
	db, err := NewBoltStorage("./r.test.data")
	assert.Nil(t, err)

	data := []byte("random value")
	err1 := db.Put("miliao", data)
	assert.Nil(t, err1)

	d2, err2 := db.Get("miliao")
	assert.Nil(t, err2)

	assert.Equal(t, data, d2)

	d3, err3 := db.Get("miliao2")
	assert.Nil(t, d3)
	assert.NotNil(t, err3)

	data2 := []byte("random value222")
	err4 := db.Put("miliao", data2)
	assert.Nil(t, err4)

	d4, err5 := db.Get("miliao")
	assert.Nil(t, err5)

	assert.Equal(t, data2, d4)
	assert.NotEqual(t, data, d4)

	db.Close()
	runtime.GC()

	*bolt_db_big_value_thresh = 1024
	db, err = NewBoltStorage("./r.test.data")
	assert.Nil(t, err)

	defer func() {
		for _, f := range db.AllFiles() {
			os.Remove(f)
		}
	}()

	d5, err6 := db.Get("miliao")
	assert.Nil(t, err6)

	runtime.GC()

	assert.Equal(t, data2, d5)
	assert.NotEqual(t, data, d5)

	runtime.GC()

	big_size := 2 * 1024 * 1024
	big_value := make([]byte, 0, big_size)
	for i := 0; i < big_size; i++ {
		big_value = append(big_value, 'a')
	}

	err = db.Put("big_value", big_value)
	assert.Nil(t, err)

	runtime.GC()

	d6, err7 := db.Get("big_value")
	assert.Nil(t, err7)
	assert.Equal(t, big_size, len(d6))
	assert.Equal(t, big_value, d6)

	assert.Equal(t, 2, len(db.AllFiles()))
}
