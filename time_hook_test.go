package gorr

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTimeNow(t *testing.T) {
	origTm := time.Now()
	fakeTm := origTm.Add(100 * time.Second)
	SetUnixTime(fakeTm)
	SetTimeIncDelta(10 * time.Millisecond)

	err := HookTimeNow()
	assert.Nil(t, err)

	tm1 := time.Now()
	assert.True(t, fakeTm.Equal(tm1))

	tm2 := time.Now()
	assert.True(t, fakeTm.Add(10*time.Millisecond).Equal(tm2))

	tm3 := time.Now()
	assert.True(t, fakeTm.Add(2*10*time.Millisecond).Equal(tm3))
}
