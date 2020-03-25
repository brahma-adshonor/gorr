package gorr

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/brahma-adshonor/gohook"
)

const (
	RegressionNone   = 0
	RegressionRecord = 1
	RegressionReplay = 2
)

const (
	RegressionResetNone    = 0
	RegressionResetStorage = 0
)

const (
	RegressionHttpHook    = 100
	RegressionConnHook    = 101
	RegressionRedisHook   = 102
	RegressionGrpcHook    = 103
	RegressionSqlHook     = 104
	RegressionOutputReset = 105
)

type Storage interface {
	Clear()
	Close()
	AllFiles() []string
	Put(key string, value []byte) error
	Get(key string) ([]byte, error)
}

type RegressionMgr struct {
	state          int
	store          Storage
	globalId       string
	curTestSuitDir string
	reset          func(int)
	notifier       func(src string, key string, value []byte)
	genKey         func(hook int, cxt context.Context, value interface{}) string
}

var (
	GlobalMgr *RegressionMgr

	RegressionRunType               = flag.Int("gorr_run_type", 0, "turn on/off gorr(0 for off, 1 for record, 2 for replay)")
	RegressionDbFile                = flag.String("gorr_db_file", "gorr.db", "file name gorr db")
	RegressionDbDirectory           = flag.String("gorr_db_dir", "/var/data/gorr", "directory to get gorr db")
	RegressionOutputDir             = flag.String("gorr_record_output_dir", "/var/data/conf/gorr", "dir to store auto generated test cases")
	RegressionOutDirRefreshInterval = flag.Int("gorr_output_dir_refresh_interval", 7200, "refresh interval in seconds")
)

func InitRegressionEngine() int {
	if *RegressionRunType == 0 {
		return 0
	}

	enableRegressionEngine(*RegressionRunType)
	dbFile := *RegressionDbDirectory + "/" + *RegressionDbFile

	if *RegressionRunType == RegressionRecord {
		os.Remove(dbFile)
	}

	RunTestCaseUploader()
	GlobalMgr.SetBoltStorage(dbFile)

	go func() {
		for {
			if *RegressionRunType == RegressionRecord {
				GlobalMgr.ResetTestSuitDir()
				GlobalMgr.reset(RegressionOutputReset)
			}
			time.Sleep(time.Duration(*RegressionOutDirRefreshInterval) * time.Second)
		}
	}()

	return *RegressionRunType
}

func enableRegressionEngine(state int) {
	if GlobalMgr != nil {
		return
	}

	GlobalMgr = newRegressionMgr(state)
}

func newRegressionMgr(state int) *RegressionMgr {
	r := &RegressionMgr{store: nil, state: state}
	r.reset = func(int) {}
	r.notifier = func(string, string, []byte) {}
	r.genKey = func(int, context.Context, interface{}) string { return "" }
	r.globalId = "gorr_global_trace_id@@20190618"
	return r
}

func createOutputDir(prefix string) string {
	tm := time.Now().Format("20060102150405")
	path := fmt.Sprintf("%s/%s%s", *RegressionOutputDir, prefix, tm)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, 0755)
		return path
	}

	for i := 0; i < 102400; i++ {
		path := fmt.Sprintf("%s/%s%s-%d", *RegressionOutputDir, prefix, tm, i)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.Mkdir(path, 0755)
			return path
		}
	}

	return ""
}

func (r *RegressionMgr) SetBoltStorageFile(file string) error {
	err := r.SetBoltStorage(*RegressionDbDirectory + "/" + file)
	if err != nil {
		return err
	}

	r.reset(RegressionResetStorage)
	return err
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

func (r *RegressionMgr) ResetTestSuitDir() string {
	dir := createOutputDir("ts")
	if len(dir) == 0 {
		return ""
	}

	r.curTestSuitDir = dir
	return dir
}

func (r *RegressionMgr) GetCurrentTestSuitDir() string {
	return r.curTestSuitDir
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

func (r *RegressionMgr) SetReset(fn func(int)) {
	r.reset = fn
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
