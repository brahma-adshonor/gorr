package regression

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/brahma-adshonor/gohook"
	"github.com/stretchr/testify/assert"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
)

func init() {
	enableRegressionEngine(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(100))

	GlobalMgr.SetNotify(func(t string, key string, value []byte) {
		fmt.Printf("regression hook event, type:%s, key:%s, value:%s\n", t, key, string(value))
	})
}

type dummyProducer struct {
	closeChan   chan int
	errorChan   chan *sarama.ProducerError
	inputChan   chan *sarama.ProducerMessage
	successChan chan *sarama.ProducerMessage
}

func (dp *dummyProducer) AsyncClose() {
	dp.closeChan <- 1
}

func (dp *dummyProducer) Close() error {
	dp.closeChan <- 1
	return nil
}

func (dp *dummyProducer) Input() chan<- *sarama.ProducerMessage {
	return dp.inputChan
}

func (dp *dummyProducer) Successes() <-chan *sarama.ProducerMessage {
	return dp.successChan
}

func (dp *dummyProducer) Errors() <-chan *sarama.ProducerError {
	return dp.errorChan
}

func newDummyProducer(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
	fmt.Printf("calling newDummyProducer from test\n")

	pd := &dummyProducer{
		closeChan:   make(chan int, 1),
		errorChan:   make(chan *sarama.ProducerError, 8),
		inputChan:   make(chan *sarama.ProducerMessage, 8),
		successChan: make(chan *sarama.ProducerMessage, 8),
	}

	return pd, nil
}

func getRealAsyncProducer(pd sarama.SyncProducer) *dummyProducer {
	rpd, ok1 := pd.(*SyncProducerHook)
	if !ok1 {
		return nil
	}

	apd := rpd.GetAsyncProducer()
	dpd, ok2 := apd.(*AsyncProducerHook)
	if !ok2 {
		return nil
	}

	dpd2 := dpd.GetOriginProducer()
	dpd3, ok3 := dpd2.(*dummyProducer)
	if !ok3 {
		return nil
	}

	return dpd3
}

func TestProducerFlow(t *testing.T) {
	debug.SetGCPercent(-1)
	GlobalMgr.SetState(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(1000))

	err := HookKafkaProducer()
	assert.Nil(t, err)

	err = gohook.Hook(NewAsyncProducerTramp, newDummyProducer, nil)
	assert.Nil(t, err)

	fmt.Printf("hook info:\n%s\n", gohook.ShowDebugInfo())

	conf := sarama.NewConfig()
	addr := []string{"1.2.3.4:22", "2.2.2.2:22"}

	conf.Producer.Return.Successes = true
	pd, err2 := sarama.NewSyncProducer(addr, conf)

	fmt.Printf("run gc manually 1th\n")
	runtime.GC()
	runtime.GC()
	fmt.Printf("after run gc manually 1th\n")

	assert.Nil(t, err2)
	assert.NotNil(t, pd)

	text := "miliao|||test"
	msg := &sarama.ProducerMessage{
		Topic: "ProducerTest",
		Value: sarama.ByteEncoder([]byte(text)),
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		p, o, err3 := pd.SendMessage(msg)
		assert.Nil(t, err3)
		assert.Equal(t, int64(42), o)
		assert.Equal(t, int32(233), p)
		wg.Done()
	}()

	rpd := getRealAsyncProducer(pd)
	msg2 := <-rpd.inputChan
	assert.Equal(t, msg, msg2)

	msg2.Offset = 42
	msg2.Partition = 233
	rpd.successChan <- msg2
	wg.Wait()

	var wg2 sync.WaitGroup
	dummy_err := fmt.Errorf("dummy error")

	wg2.Add(1)
	go func() {
		_, _, err3 := pd.SendMessage(msg)
		assert.NotNil(t, err3)
		assert.Equal(t, dummy_err.Error(), err3.Error())
		wg2.Done()
	}()

	msg3 := <-rpd.inputChan
	assert.Equal(t, msg, msg3)

	fmt.Printf("run gc manually 2th\n")
	runtime.GC()
	fmt.Printf("after run gc manually 2th\n")

	rpd.errorChan <- &sarama.ProducerError{
		Msg: msg3,
		Err: dummy_err,
	}

	wg2.Wait()

	fmt.Printf("testing multiple messages\n")

	wg.Add(1)

	msgs := []*sarama.ProducerMessage{
		&sarama.ProducerMessage{
			Topic: "ProducerTest",
			Value: sarama.ByteEncoder([]byte(text)),
		},
		&sarama.ProducerMessage{
			Topic: "ProducerTest2",
			Value: sarama.ByteEncoder([]byte(text)),
		},
	}

	go func() {
		err := pd.SendMessages(msgs)
		assert.Nil(t, err)
		wg.Done()
	}()

	m1 := <-rpd.inputChan
	fmt.Printf("recv m1\n")
	rpd.successChan <- m1
	fmt.Printf("done recv m1\n")

	m2 := <-rpd.inputChan
	rpd.successChan <- m2
	fmt.Printf("done recv m2\n")

	wg.Wait()
	pd.Close()

	// test replay
	GlobalMgr.SetState(RegressionReplay)

	pd, err2 = sarama.NewSyncProducer(addr, conf)
	assert.Nil(t, err2)
	assert.NotNil(t, pd)

	fmt.Printf("starting replay test\n")

	p, o, err3 := pd.SendMessage(msg)
	assert.Nil(t, err3)
	assert.Equal(t, int64(42), o)
	assert.Equal(t, int32(233), p)

	pd.Close()

	fmt.Printf("starting Close() test\n")

	GlobalMgr.SetState(RegressionRecord)
	pd, _ = sarama.NewSyncProducer(addr, conf)

	pd.Close()

	rpd = getRealAsyncProducer(pd)
	<-rpd.closeChan

	fmt.Printf("run gc manually 3th\n")
	runtime.GC()
	fmt.Printf("after run gc manually 3th\n")
}
