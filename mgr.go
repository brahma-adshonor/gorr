package regression

import (
	"context"
	"github.com/brahma-adshonor/gohook"
)

const (
	RegressionRecord = 111
	RegressionReplay = 222
)

const (
	RegressionHttpHook  = 100
	RegressionConnHook  = 101
	RegressionRedisHook = 102
	RegressionGrpcHook  = 103
	RegressionSqlHook   = 104
)

type Storage interface {
	Clear()
	Close()
	AllFiles() []string
	Put(key string, value []byte) error
	Get(key string) ([]byte, error)
}

type RegressionMgr struct {
	state    int
	store    Storage
	globalId string
	notifier func(src string, key string, value []byte)
	genKey   func(hook int, cxt context.Context, value interface{}) string
}

var GlobalMgr *RegressionMgr

func EnableRegressionEngine(state int) {
	if GlobalMgr != nil {
		return
	}

	GlobalMgr = newRegressionMgr(state)
}

func newRegressionMgr(state int) *RegressionMgr {
	r := &RegressionMgr{store: nil, state: state}
	r.notifier = func(string, string, []byte) {}
	r.genKey = func(int, context.Context, interface{}) string { return "" }
	r.globalId = "regression_global_trace_id@@20190618"
	return r
}

func (r *RegressionMgr) SetBoltStorage(path string) error {
	db, err := NewBoltStorage(path)
	if err != nil {
		return err
	}

	origin := r.store
	r.store = db

	if origin != nil {
		origin.Close()
	}

	return nil
}

func (r *RegressionMgr) GetDebugInfo() string {
	return gohook.ShowDebugInfo()
}

func (r *RegressionMgr) SetStorage(s Storage) {
	r.store = s
}

func (r *RegressionMgr) SetState(state int) {
	r.state = state
}

func (r *RegressionMgr) ShouldRecord() bool {
	return r.state == RegressionRecord
}

func (r *RegressionMgr) SetNotify(fn func(string, string, []byte)) {
	r.notifier = fn
}

func (r *RegressionMgr) EnableGenKey() {
	r.StoreValue("RegressionMgrInfo@EnableGenKey", []byte("enable"))
}

func (r *RegressionMgr) IsGenKeyEnabled() bool {
	v, err := r.GetValue("RegressionMgrInfo@EnableGenKey")
	if err != nil || len(string(v)) == 0 || string(v) != "enable" {
		return false
	}

	return true
}

func (r *RegressionMgr) SetGenKey(fn func(int, context.Context, interface{}) string) {
	r.genKey = fn
}

func (r *RegressionMgr) ClearStorage() {
	r.store.Clear()
}

func (r *RegressionMgr) StoreValue(key string, data []byte) error {
	return r.store.Put(key, data)
}

func (r *RegressionMgr) GetValue(key string) ([]byte, error) {
	data, err := r.store.Get(key)
	return data, err
}

func (r *RegressionMgr) GetCurTraceId() string {
	// TODO
	return "todo_yet_to_implement_by_gls"
}

func (r *RegressionMgr) SetCurTraceId() error {
	// TODO
	return nil
}

func (r *RegressionMgr) GetDbFiles() []string {
	return r.store.AllFiles()
}

func (r *RegressionMgr) EnableHook() error {
	hk := []func() error{
		HookHttpFunc,
		HookRedisFunc,
		HookGrpcInvoke,
		HookMysqlDriver,
		HookKafkaProducer,
	}

	for _, fn := range hk {
		err := fn()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RegressionMgr) DisableHook() {
	hk := []func() error{
		UnHookHttpFunc,
		UnHookRedisFunc,
		UnHookGrpcInvoke,
		UnHookMysqlDriver,
		UnHookKafkaProducer,
	}

	for _, fn := range hk {
		fn()
	}
}
