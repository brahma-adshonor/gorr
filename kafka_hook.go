package gorr

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/brahma-adshonor/gohook"
	"sync"
)

type AsyncProducerHook struct {
	addrs  []string
	config *sarama.Config

	closeChan chan int
	origProd  sarama.AsyncProducer

	errorChan   chan *sarama.ProducerError
	inputChan   chan *sarama.ProducerMessage
	successChan chan *sarama.ProducerMessage

	origErrorChan   <-chan *sarama.ProducerError
	origInputChan   chan<- *sarama.ProducerMessage
	origSuccessChan <-chan *sarama.ProducerMessage
}

func (ap *AsyncProducerHook) GetOriginProducer() sarama.AsyncProducer {
	return ap.origProd
}

func (ap *AsyncProducerHook) AsyncClose() {
	ap.closeChan <- 1
}

func (ap *AsyncProducerHook) Close() error {
	ap.closeChan <- 1
	return nil
}

func (ap *AsyncProducerHook) Input() chan<- *sarama.ProducerMessage {
	return ap.inputChan
}

func (ap *AsyncProducerHook) Successes() <-chan *sarama.ProducerMessage {
	return ap.successChan
}

func (ap *AsyncProducerHook) Errors() <-chan *sarama.ProducerError {
	return ap.errorChan
}

func NewAsyncProducerHook(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
	GlobalMgr.notifier("kafkaf async producer hook", "calling new async producer hook", []byte(""))

	pd := &AsyncProducerHook{
		config:      conf,
		closeChan:   make(chan int, 1),
		errorChan:   make(chan *sarama.ProducerError, 8),
		inputChan:   make(chan *sarama.ProducerMessage, 8),
		successChan: make(chan *sarama.ProducerMessage, 8),
	}

	// always deep copy arguments, since escape analysis doesn't work well with gohook
	pd.addrs = make([]string, len(addrs))
	copy(pd.addrs, addrs)

	if GlobalMgr.ShouldRecord() {
		orig, err := NewAsyncProducerTramp(addrs, conf)
		if err != nil {
			return nil, err
		}

		pd.origProd = orig
		pd.origInputChan = orig.Input()
		pd.origErrorChan = orig.Errors()
		pd.origSuccessChan = orig.Successes()
	}

	go func() {
		for {
			select {
			case msg := <-pd.inputChan:
				if pd.origInputChan != nil {
					pd.origInputChan <- msg
				} else {
					pd.successChan <- msg
				}
				GlobalMgr.notifier("kafkaf async producer hook", "async producer receiving msg", []byte(""))
			case msg := <-pd.origSuccessChan:
				pd.successChan <- msg
				GlobalMgr.notifier("kafkaf async producer hook", "async producer succ notification", []byte(""))
			case msg := <-pd.origErrorChan:
				pd.errorChan <- msg
				GlobalMgr.notifier("kafkaf async producer hook", "async producer error notification", []byte(""))
			case <-pd.closeChan:
				GlobalMgr.notifier("kafkaf async producer hook", "async producer closing", []byte(""))
				if pd.origProd != nil {
					pd.origProd.AsyncClose()
				}
				close(pd.inputChan)
				close(pd.successChan)
				close(pd.errorChan)
				close(pd.closeChan)
				return
			}
		}
	}()

	return pd, nil
}

//go:noinline
func NewAsyncProducerTramp(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
	fmt.Printf("dummy function for regrestion testing:%v", conf)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if conf != nil {
		panic("trampoline NewAsyncProducer function is not allowed to be called")
	}

	return nil, nil
}

func NewSyncProducerHook(addrs []string, config *sarama.Config) (sarama.SyncProducer, error) {
	return newSyncProducerReal(addrs, config)
}

///////////////////sync producer hook: copyed from sarama.SyncProducer ///////////////////////////
type SyncProducerHook struct {
	producer sarama.AsyncProducer
	wg       sync.WaitGroup
}

type userMetaData struct {
	origin      interface{}
	expectation chan *sarama.ProducerError
}

func newSyncProducerReal(addrs []string, config *sarama.Config) (sarama.SyncProducer, error) {
	GlobalMgr.notifier("kafka sync producer", "NewSyncProducerHook", []byte(""))

	if config == nil {
		config = sarama.NewConfig()
		config.Producer.Return.Successes = true
	}

	if err := verifyProducerConfig(config); err != nil {
		return nil, err
	}

	p, err := sarama.NewAsyncProducer(addrs, config)
	if err != nil {
		return nil, err
	}

	return newSyncProducerFromAsyncProducer(p), nil
}

func withRecover(fn func()) {
	defer func() {
		handler := sarama.PanicHandler
		if handler != nil {
			if err := recover(); err != nil {
				handler(err)
			}
		}
	}()

	fn()
}

func newSyncProducerFromAsyncProducer(p sarama.AsyncProducer) *SyncProducerHook {
	sp := &SyncProducerHook{producer: p}

	sp.wg.Add(2)
	go withRecover(sp.handleSuccesses)
	go withRecover(sp.handleErrors)

	return sp
}

func verifyProducerConfig(config *sarama.Config) error {
	if !config.Producer.Return.Errors {
		return sarama.ConfigurationError("Producer.Return.Errors must be true to be used in a SyncProducer")
	}
	if !config.Producer.Return.Successes {
		return sarama.ConfigurationError("Producer.Return.Successes must be true to be used in a SyncProducer")
	}
	return nil
}

func (sp *SyncProducerHook) GetAsyncProducer() sarama.AsyncProducer {
	return sp.producer
}

func (sp *SyncProducerHook) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	expectation := make(chan *sarama.ProducerError, 1)
	ud := &userMetaData{origin: msg.Metadata, expectation: expectation}

	msg.Metadata = ud
	sp.producer.Input() <- msg

	if err := <-expectation; err != nil {
		msg.Metadata = ud.origin
		return -1, -1, err.Err
	}

	msg.Metadata = ud.origin
	return msg.Partition, msg.Offset, nil
}

func (sp *SyncProducerHook) SendMessages(msgs []*sarama.ProducerMessage) error {
	expectations := make(chan chan *sarama.ProducerError, len(msgs))
	go func() {
		for _, msg := range msgs {
			expectation := make(chan *sarama.ProducerError, 1)
			ud := &userMetaData{origin: msg.Metadata, expectation: expectation}
			msg.Metadata = ud
			sp.producer.Input() <- msg
			expectations <- expectation
		}
		close(expectations)
	}()

	var errors sarama.ProducerErrors
	for expectation := range expectations {
		if err := <-expectation; err != nil {
			errors = append(errors, err)
		}
	}

	for _, msg := range msgs {
		ud, ok := msg.Metadata.(*userMetaData)
		if !ok {
			continue
		}
		msg.Metadata = ud.origin
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

func (sp *SyncProducerHook) handleSuccesses() {
	defer sp.wg.Done()
	for msg := range sp.producer.Successes() {
		GlobalMgr.notifier("kafka sync producer", "succ notification recv", []byte(""))
		ud, ok := msg.Metadata.(*userMetaData)
		if !ok {
			GlobalMgr.notifier("kafka sync producer", "no expectation chan is setup in success handler", []byte(""))
			continue
		}
		expectation := ud.expectation
		expectation <- nil
	}
}

func (sp *SyncProducerHook) handleErrors() {
	defer sp.wg.Done()
	for err := range sp.producer.Errors() {
		GlobalMgr.notifier("kafka sync producer", "error notification recv", []byte(""))
		ud, ok := err.Msg.Metadata.(*userMetaData)
		if !ok {
			GlobalMgr.notifier("kafka sync producer", "no expectation chan is setup in error handler", []byte(""))
			continue
		}
		expectation := ud.expectation
		expectation <- err
	}
}

func (sp *SyncProducerHook) Close() error {
	sp.producer.AsyncClose()
	sp.wg.Wait()
	return nil
}

func HookKafkaProducer() error {
	err := gohook.Hook(sarama.NewSyncProducer, NewSyncProducerHook, nil)
	if err != nil {
		return err
	}

	err = gohook.Hook(sarama.NewAsyncProducer, NewAsyncProducerHook, NewAsyncProducerTramp)
	return err
}

func UnHookKafkaProducer() error {
	gohook.UnHook(sarama.NewSyncProducer)
	return gohook.UnHook(sarama.NewAsyncProducer)
}
