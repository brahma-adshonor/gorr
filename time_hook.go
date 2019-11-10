package gorr

import (
	"github.com/brahma-adshonor/gohook"
	"sync"
	"time"
)

var (
	timeVal time.Time
	mu      sync.Mutex
	delta   time.Duration = 10 * time.Millisecond
)

func SetTimeIncDelta(d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	delta = d
}

func SetUnixTime(val time.Time) {
	mu.Lock()
	defer mu.Unlock()
	timeVal = val
}

func GetTimeValAndInc() time.Time {
	mu.Lock()
	defer mu.Unlock()

	ret := timeVal
	timeVal = timeVal.Add(delta)
	return ret
}

//go:noinline
func timeNow() time.Time {
	return GetTimeValAndInc()
}

func HookTimeNow() error {
	return gohook.Hook(time.Now, timeNow, nil)
}
