package gorr

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"github.com/brahma-adshonor/gohook"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// need a way to tag object pointer
// 1. cookie to identify whether a pointer is created by hook
//    --> false positive is possible by definition.
// 2. put pointer into a set
//    --> garbage collection leads to stale pointer in the set(worse, if pointer is reallocated for the same type of object)

// mongo.Connect
// the reason to hook mongo.Connect is that:
// 1. we need to obtain the client object pointer
// 2. client object to build key to record all mongo call from collection

// hook for replay mode only
// mongo.Cursor: all public functions
// mongo.SingleResult: all public functions

//  struct to hold client info
type clientHolder struct {
	cookie1 uint64
	client  mongo.Client
	opts    *options.ClientOptions
	cookie2 uint64
}

type CursorData struct {
	Id   int64
	Err  string
	Data []bson.Raw
}

type cursorHolder struct {
	cookie1  uint64
	cur      int64
	cd       CursorData
	cursor   mongo.Cursor
	registry *bsoncodec.Registry
	cookie2  uint64
}

type SingleResultData struct {
	Err  string
	Data bson.Raw
}

type singleResultHolder struct {
	cookie1  uint64
	sd       SingleResultData
	ret      mongo.SingleResult
	registry *bsoncodec.Registry
	cookie2  uint64
}

const (
	cookieValue1 uint64 = 0x1badf00d2badf00d
	cookieValue2 uint64 = 0x3badf00d4badf00d
)

func clientHolderFinalizer(c *clientHolder) {
	c.cookie1 = 23
	c.cookie2 = 23
}

func cursorHolderFinalizer(c *cursorHolder) {
	c.cookie1 = 23
	c.cookie2 = 23
}

func singleResultHolderFinalizer(c *singleResultHolder) {
	c.cookie1 = 23
	c.cookie2 = 23
}

func getClientHolder(c *mongo.Client) *clientHolder {
	if c == nil {
		return nil
	}

	var tmp clientHolder
	h := (*clientHolder)(unsafe.Pointer(uintptr(unsafe.Pointer(c)) - unsafe.Offsetof(tmp.client)))
	if h.cookie1 != cookieValue1 || h.cookie2 != cookieValue2 {
		return nil
	}

	return h
}

func getCursorHolder(c *mongo.Cursor) *cursorHolder {
	var tmp cursorHolder
	if c == nil {
		return nil
	}

	h := (*cursorHolder)(unsafe.Pointer(uintptr(unsafe.Pointer(c)) - unsafe.Offsetof(tmp.cursor)))
	if h.cookie1 != cookieValue1 || h.cookie2 != cookieValue2 {
		return nil
	}
	return h
}

func getSingleResultHolder(c *mongo.SingleResult) *singleResultHolder {
	if c == nil {
		return nil
	}

	var tmp singleResultHolder
	h := (*singleResultHolder)(unsafe.Pointer(uintptr(unsafe.Pointer(c)) - unsafe.Offsetof(tmp.ret)))
	if h.cookie1 != cookieValue1 || h.cookie2 != cookieValue2 {
		return nil
	}
	return h
}

func getRegistryByClient(c *mongo.Client) *bsoncodec.Registry {
	h := getClientHolder(c)
	if h == nil {
		return bson.DefaultRegistry
	}

	return h.opts.Registry
}

func buildKeyByClient(c *mongo.Client, op string, key string) string {
	h := getClientHolder(c)
	cred := h.opts.Auth
	authMech := "notspecified"
	authSource := "notspecified"
	authName := "notspecified"
	if cred != nil {
		authName = cred.Username
		authSource = cred.AuthSource
		authMech = cred.AuthMechanism
	}

	dbinfo := fmt.Sprintf("mongo_hook_key@%s@%s_%s_%s@%s@%s",
		strings.Join(h.opts.Hosts, "||"), authMech, authSource, authName, op, key)

	return dbinfo
}

func extractCursorData(c *mongo.Cursor) CursorData {
	cd := CursorData{
		Id:  c.ID(),
		Err: c.Err().Error(),
	}

	for c.Next(context.Background()) {
		cd.Data = append(cd.Data, c.Current)
	}

	return cd
}

func buildCursorHolder(data CursorData, cl *mongo.Client) *cursorHolder {
	h := cursorHolder{}

	h.cd = data
	h.cookie1 = cookieValue1
	h.cookie2 = cookieValue2
	h.registry = getRegistryByClient(cl)

	runtime.SetFinalizer(&h, cursorHolderFinalizer)

	return &h
}

func extractSingleResult(r *mongo.SingleResult) SingleResultData {
	err := r.Err()
	sd := SingleResultData{}
	if err != nil {
		sd.Err = err.Error()
	}

	sd.Data, err = r.DecodeBytes()
	if err != nil {
		sd.Err = err.Error()
	}

	return sd
}

func buildSingleResultHolder(d SingleResultData, cl *mongo.Client) *singleResultHolder {
	h := singleResultHolder{}

	h.sd = d
	h.cookie1 = cookieValue1
	h.cookie2 = cookieValue2
	h.registry = getRegistryByClient(cl)
	runtime.SetFinalizer(&h, singleResultHolderFinalizer)

	return &h
}

func marshalValue(v interface{}) ([]byte, error) {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func unmarshalValue(d []byte, v interface{}) error {
	buff := bytes.NewBuffer(d)
	dec := gob.NewDecoder(buff)
	err := dec.Decode(v)
	return err
}

type cursorRealFunc func() (*mongo.Cursor, error)
type singleResultRealFunc func() *mongo.SingleResult

func getSingleResultData(fname, key string, c *mongo.Client, realFunc singleResultRealFunc) *mongo.SingleResult {
	if GlobalMgr.ShouldRecord() {
		sr := realFunc()
		h := buildSingleResultHolder(extractSingleResult(sr), c)

		dd, err := marshalValue(h.sd)
		if err != nil {
			GlobalMgr.notifier(fmt.Sprintf("mongo.%s record SingleResult", fname), "marshal failed", []byte(err.Error()))
			return nil
		}

		GlobalMgr.StoreValue(key, dd)
		GlobalMgr.notifier(fmt.Sprintf("mongo.%s record SingleResult", fname), fmt.Sprintf("%s (done)", key), dd)
		return &h.ret
	}

	dd, err := GlobalMgr.GetValue(key)
	if err != nil {
		GlobalMgr.notifier(fmt.Sprintf("mongo.%s replay SingleResult", fname), "get value from db failed", []byte(err.Error()))
		return nil
	}

	var v SingleResultData
	err = unmarshalValue(dd, &v)
	if err != nil {
		GlobalMgr.notifier(fmt.Sprintf("mongo.%s replay SingleResult", fname), "unmarshal failed", []byte(err.Error()))
		return nil
	}

	h := buildSingleResultHolder(v, c)
	GlobalMgr.notifier(fmt.Sprintf("mongo.%s replay SingleResult", fname), fmt.Sprintf("%s(done)", key), dd)
	return &h.ret
}

func getCursorData(fname, key string, c *mongo.Client, realFunc cursorRealFunc) (*mongo.Cursor, error) {
	if GlobalMgr.ShouldRecord() {
		cs, err := realFunc()
		if err != nil {
			GlobalMgr.notifier(fmt.Sprintf("mongo.%s record Cursor", fname), fmt.Sprintf("%s (call origin fail)", key), []byte(""))
			return cs, err
		}

		h := buildCursorHolder(extractCursorData(cs), c)
		dd, err := marshalValue(h.cd)
		if err != nil {
			GlobalMgr.notifier(fmt.Sprintf("mongo.%s record Cursor", fname), fmt.Sprintf("%s (marshal ret fail)", key), []byte(""))
			return nil, fmt.Errorf("mongo.%s record Cursor, marshal failed, err:%s", fname, err)
		}

		GlobalMgr.StoreValue(key, dd)
		GlobalMgr.notifier(fmt.Sprintf("mongo.%s record Cursor", fname), fmt.Sprintf("%s (done)", key), dd)
		return &h.cursor, err
	}

	dd, err := GlobalMgr.GetValue(key)
	if err != nil {
		GlobalMgr.notifier(fmt.Sprintf("mongo.%s replay Cursor", fname), fmt.Sprintf("%s (get from db fail)", key), []byte(""))
		return nil, fmt.Errorf("mongo.%s replay Cursor, failed to get value from db, err:%s", fname, err)
	}

	var v CursorData
	err = unmarshalValue(dd, &v)
	if err != nil {
		GlobalMgr.notifier(fmt.Sprintf("mongo.%s replay Cursor", fname), fmt.Sprintf("%s (unmarshal fail)", key), []byte(""))
		return nil, fmt.Errorf("mongo.%s replay Cursor, unmarshal failed, err:%s", fname, err)
	}

	h := buildCursorHolder(v, c)
	GlobalMgr.notifier(fmt.Sprintf("mongo.%s replay Cursor", fname), fmt.Sprintf("%s(done)", key), dd)
	return &h.cursor, nil
}

type updateResultData struct {
	MatchCount    int64
	ModifiedCount int64
	UpsertedCount int64
	UpsertedID    bsoncore.Value
}

func marshalUpdateResult(ret *mongo.UpdateResult, fname string) ([]byte, error) {
	val := updateResultData{
		MatchCount:    ret.MatchedCount,
		ModifiedCount: ret.ModifiedCount,
		UpsertedCount: ret.UpsertedCount,
	}

	t, d, err2 := bson.MarshalValue(ret.UpsertedID)
	if err2 != nil {
		return nil, fmt.Errorf("marshal %s result id failed, err:%s", fname, err2)
	}

	val.UpsertedID.Type = t
	val.UpsertedID.Data = d

	d, err := marshalValue(&val)
	if err != nil {
		return nil, fmt.Errorf("marshal %s return data failed, err:%s", fname, err)
	}

	return d, err
}

func unmarshalUpdateResult(d []byte, fname string) (*mongo.UpdateResult, error) {
	var val updateResultData
	err := unmarshalValue(d, &val)
	if err != nil {
		return nil, fmt.Errorf("%s unmarshal val from db failed, err:%s", fname, err)
	}

	ret := mongo.UpdateResult{
		MatchedCount:  val.MatchCount,
		ModifiedCount: val.ModifiedCount,
		UpsertedCount: val.UpsertedCount,
	}

	raw := bson.RawValue{Type: val.UpsertedID.Type, Value: val.UpsertedID.Data}
	err = raw.Unmarshal(&ret.UpsertedID)

	if err != nil {
		return nil, fmt.Errorf("%s unmarshal upserted id failed, err:%s", fname, err)
	}

	return &ret, nil
}

// cursor hook
func mgCursorIDHook(c *mongo.Cursor) int64 {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorIDTramp(c)
		}
	}

	return h.cd.Id
}

func mgCursorIDTramp(c *mongo.Cursor) int64 {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v", c)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return 0
}

func mgCursorNextHook(c *mongo.Cursor, ctx context.Context) bool {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorNextTramp(c, ctx)
		}
	}

	if h.cur == int64(len(h.cd.Data)) {
		return false
	}

	h.cursor.Current = h.cd.Data[h.cur]
	h.cur++

	return true
}

func mgCursorNextTramp(c *mongo.Cursor, ctx context.Context) bool {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v", c, ctx)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return true
}

func mgCursorTryNextHook(c *mongo.Cursor, ctx context.Context) bool {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorTryNextTramp(c, ctx)
		}
	}

	return mgCursorNextHook(c, ctx)
}

func mgCursorTryNextTramp(c *mongo.Cursor, ctx context.Context) bool {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v", c, ctx)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return true
}

func mgCursorDecodeHook(c *mongo.Cursor, val interface{}) error {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorDecodeTramp(c, val)
		}
	}

	return bson.UnmarshalWithRegistry(h.registry, h.cursor.Current, val)
}

func mgCursorDecodeTramp(c *mongo.Cursor, val interface{}) error {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v", c, val)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgCursorErrHook(c *mongo.Cursor) error {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorErrTramp(c)
		}
	}

	if len(h.cd.Err) == 0 {
		return nil
	}

	return fmt.Errorf(h.cd.Err)
}

func mgCursorErrTramp(c *mongo.Cursor) error {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v", c)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgCursorCloseHook(c *mongo.Cursor, ctx context.Context) error {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorCloseTramp(c, ctx)
		}
	}

	return nil
}

func mgCursorCloseTramp(c *mongo.Cursor, ctx context.Context) error {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v", c)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgCursorAllHook(c *mongo.Cursor, ctx context.Context, results interface{}) error {
	h := getCursorHolder(c)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgCursorAllTramp(c, ctx, results)
		}
	}

	resultsVal := reflect.ValueOf(results)
	if resultsVal.Kind() != reflect.Ptr {
		return errors.New("results argument must be a pointer to a slice")
	}

	var index int
	var err error

	sliceVal := resultsVal.Elem()
	elementType := sliceVal.Type().Elem()

	docs := make([]bsoncore.Document, 0, len(h.cd.Data))
	for i := int(h.cur); i < len(h.cd.Data); i++ {
		docs = append(docs, bsoncore.Document(h.cd.Data[i]))
	}

	h.cur = int64(len(h.cd.Data))

	sliceVal, index, err = addFromBatch(h.registry, sliceVal, elementType, docs, index)
	if err != nil {
		return err
	}

	resultsVal.Elem().Set(sliceVal.Slice(0, index))
	return nil
}

func addFromBatch(registry *bsoncodec.Registry, sliceVal reflect.Value, elemType reflect.Type, docs []bsoncore.Document,
	index int) (reflect.Value, int, error) {

	for _, doc := range docs {
		if sliceVal.Len() == index {
			// slice is full
			newElem := reflect.New(elemType)
			sliceVal = reflect.Append(sliceVal, newElem.Elem())
			sliceVal = sliceVal.Slice(0, sliceVal.Cap())
		}

		currElem := sliceVal.Index(index).Addr().Interface()
		if err := bson.UnmarshalWithRegistry(registry, doc, currElem); err != nil {
			return sliceVal, index, err
		}

		index++
	}

	return sliceVal, index, nil
}

func mgCursorAllTramp(c *mongo.Cursor, ctx context.Context, results interface{}) error {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v,%+v,%+v", c, ctx, results)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

// SingleResult hook
func mgSingleResultDecodeHook(sr *mongo.SingleResult, v interface{}) error {
	h := getSingleResultHolder(sr)

	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgSingleResultDecodeTramp(sr, v)
		}
	}

	if len(h.sd.Err) != 0 {
		return fmt.Errorf(h.sd.Err)
	}

	bson.UnmarshalWithRegistry(h.registry, h.sd.Data, v)
	return nil
}

func mgSingleResultDecodeTramp(sr *mongo.SingleResult, v interface{}) error {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v,%+v", sr, v)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if sr != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgSingleResultDecodeBytesHook(sr *mongo.SingleResult) (bson.Raw, error) {
	h := getSingleResultHolder(sr)

	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgSingleResultDecodeBytesTramp(sr)
		}
	}

	if len(h.sd.Err) != 0 {
		return nil, fmt.Errorf(h.sd.Err)
	}

	return h.sd.Data, nil
}

func mgSingleResultDecodeBytesTramp(sr *mongo.SingleResult) (bson.Raw, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v", sr)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if sr != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgSingleResultErrHook(sr *mongo.SingleResult) error {
	h := getSingleResultHolder(sr)
	if GlobalMgr.ShouldRecord() {
		if h == nil {
			return mgSingleResultErrTramp(sr)
		}
	}

	if len(h.sd.Err) == 0 {
		return nil
	}

	return fmt.Errorf(h.sd.Err)
}

func mgSingleResultErrTramp(sr *mongo.SingleResult) error {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v", sr)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if sr != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

// global mongo.Connect() hook
func mgConnectHook(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	var err error
	var client *mongo.Client

	clientOpt := options.MergeClientOptions(opts...)
	client, err = mgConnectTramp(ctx, clientOpt)

	if clientOpt.Registry == nil {
		clientOpt.Registry = bson.DefaultRegistry
	}

	if err != nil {
		GlobalMgr.notifier("call to mongo.NewClient failed", fmt.Sprintf("err:%s", err), []byte(""))
		return nil, err
	}

	ch := &clientHolder{
		client:  *client,
		opts:    clientOpt,
		cookie1: cookieValue1,
		cookie2: cookieValue2}

	runtime.SetFinalizer(ch, clientHolderFinalizer)

	return &ch.client, err
}

func mgConnectTramp(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v", ctx, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if opts != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return &mongo.Client{}, nil
}

// dynamic hook api

type hooklist struct {
	obj        interface{}
	method     string
	replace    interface{}
	trampoline interface{}
	mode       int // 1 for record 2 for replay 3 for both
}

var (
	hl = []hooklist{
		hooklist{&mongo.Cursor{}, "All", mgCursorAllHook, mgCursorAllTramp, 3},
		hooklist{&mongo.Cursor{}, "Close", mgCursorCloseHook, mgCursorCloseTramp, 3},
		hooklist{&mongo.Cursor{}, "Err", mgCursorErrHook, mgCursorErrTramp, 3},
		hooklist{&mongo.Cursor{}, "ID", mgCursorIDHook, mgCursorIDTramp, 3},
		hooklist{&mongo.Cursor{}, "Next", mgCursorNextHook, mgCursorNextTramp, 3},
		hooklist{&mongo.Cursor{}, "TryNext", mgCursorTryNextHook, mgCursorTryNextTramp, 3},
		hooklist{&mongo.Cursor{}, "Decode", mgCursorDecodeHook, mgCursorDecodeTramp, 3},

		hooklist{&mongo.SingleResult{}, "DecodeBytes", mgSingleResultDecodeBytesHook, mgSingleResultDecodeBytesTramp, 3},
		hooklist{&mongo.SingleResult{}, "Decode", mgSingleResultDecodeHook, mgSingleResultDecodeTramp, 3},
		hooklist{&mongo.SingleResult{}, "Err", mgSingleResultErrHook, mgSingleResultErrTramp, 3},

		hooklist{&mongo.Client{}, "Connect", mgClientConnectHook, nil, 2},
		hooklist{&mongo.Client{}, "Disconnect", mgClientDisconnectHook, nil, 2},
		hooklist{&mongo.Client{}, "ListDatabases", mgClientListDatabasesHook, mgClientListDatabasesTramp, 3},

		hooklist{&mongo.Database{}, "Aggregate", mgDatabaseAggregateHook, mgDatabaseAggregateTramp, 3},
		hooklist{&mongo.Database{}, "Drop", mgDatabaseDropHook, nil, 2},
		hooklist{&mongo.Database{}, "ListCollections", mgDatabaseListCollectionsHook, mgDatabaseListCollectionsTramp, 3},
		hooklist{&mongo.Database{}, "RunCommand", mgDatabaseRunCommandHook, mgDatabaseRunCommandTramp, 3},
		hooklist{&mongo.Database{}, "RunCommandCursor", mgDatabaseRunCommandCursorHook, mgDatabaseRunCommandCursorTramp, 3},

		hooklist{&mongo.Collection{}, "Aggregate", mgCollectionAggregateHook, mgCollectionAggregateTramp, 3},
		hooklist{&mongo.Collection{}, "CountDocuments", mgCollectionCountDocumentsHook, mgCollectionCountDocumentsTramp, 3},
		hooklist{&mongo.Collection{}, "DeleteMany", mgCollectionDeleteManyHook, mgCollectionDeleteManyTramp, 3},
		hooklist{&mongo.Collection{}, "DeleteOne", mgCollectionDeleteOneHook, mgCollectionDeleteOneTramp, 3},
		hooklist{&mongo.Collection{}, "Distinct", mgCollectionDistinctHook, mgCollectionDistinctTramp, 3},
		hooklist{&mongo.Collection{}, "Drop", mgCollectionDropHook, nil, 2},
		hooklist{&mongo.Collection{}, "EstimatedDocumentCount", mgCollectionEstimatedDocumentCountHook, mgCollectionEstimatedDocumentCountTramp, 3},
		hooklist{&mongo.Collection{}, "Find", mgCollectionFindHook, mgCollectionFindTramp, 3},
		hooklist{&mongo.Collection{}, "FindOneAndDelete", mgCollectionFindOneAndDeleteHook, mgCollectionFindOneAndDeleteTramp, 3},
		hooklist{&mongo.Collection{}, "FindOneAndReplace", mgCollectionFindOneAndReplaceHook, mgCollectionFindOneAndReplaceTramp, 3},
		hooklist{&mongo.Collection{}, "FindOneAndUpdate", mgCollectionFindOneAndUpdateHook, mgCollectionFindOneAndUpdateTramp, 3},
		hooklist{&mongo.Collection{}, "InsertMany", mgCollectionInsertManyHook, nil, 2},
		hooklist{&mongo.Collection{}, "InsertOne", mgCollectionInsertOneHook, nil, 2},
		hooklist{&mongo.Collection{}, "ReplaceOne", mgCollectionReplaceOneHook, mgCollectionReplaceOneTramp, 3},
		hooklist{&mongo.Collection{}, "UpdateMany", mgCollectionUpdateManyHook, mgCollectionUpdateManyTramp, 3},
	}
)

func DisableMongoHook() error {
	gohook.UnHook(mongo.Connect)
	for i := range hl {
		gohook.UnHookMethod(hl[i].obj, hl[i].method)
	}

    return nil
}

func EnableMongoHook() error {
	var err error

	defer func() {
		if err != nil {
			DisableMongoHook()
		}
	}()

	mode := 2
	if GlobalMgr.ShouldRecord() {
		mode = 1
	}

	err = gohook.Hook(mongo.Connect, mgConnectHook, mgConnectTramp)
	if err != nil {
		return err
	}

	for i := range hl {
		if (mode & hl[i].mode) > 0 {
			err = gohook.HookMethod(hl[i].obj, hl[i].method, hl[i].replace, hl[i].trampoline)
			if err != nil {
				return fmt.Errorf("HookMethod failed, fn:%+v,err:%s", hl[i], err)
			}
		}
	}

	return nil
}
