package gorr

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// types of objects to hook:
// 1. client
// 2. database
// 3. collection

// mongo client hook
// client.Connect
// client.Disconnect
// client.ListDatabases

func mgClientConnectHook(c *mongo.Client, ctx context.Context) error {
	if GlobalMgr.ShouldRecord() {
		panic("should not hook client.Connect() for record mode")
	}

	// replaying, no real connection is needed.
	return nil
}

func mgClientDisconnectHook(c *mongo.Client, ctx context.Context) error {
	if GlobalMgr.ShouldRecord() {
		panic("should not hook client.Disconnect() for record mode")
	}

	// replaying, no real connection is needed.
	return nil
}

func mgClientListDatabasesHook(c *mongo.Client, ctx context.Context, filter interface{}, opts ...*options.ListDatabasesOptions) (mongo.ListDatabasesResult, error) {
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Client.ListDatabases", string(fv))

	if GlobalMgr.ShouldRecord() {
		v, err := mgClientListDatabasesTramp(c, ctx, filter, opts...)
		if err != nil {
			return mongo.ListDatabasesResult{}, err
		}

		data, err := marshalValue(v)
		if err != nil {
			return mongo.ListDatabasesResult{}, fmt.Errorf("encode mongo client.ListDatabases failed, err:%s", err)
		}
		GlobalMgr.StoreValue(key, data)
		return v, nil
	}

	d, err := GlobalMgr.GetValue(key)
	if err != nil {
		return mongo.ListDatabasesResult{}, fmt.Errorf("get client.ListDatabases from db failed, err:%s", err)
	}

	var v mongo.ListDatabasesResult

	err = unmarshalValue(d, &v)
	if err != nil {
		return v, fmt.Errorf("decode client.ListDatabases value from db failed, err:%s", err)
	}

	return v, nil
}

func mgClientListDatabasesTramp(c *mongo.Client, ctx context.Context, filter interface{}, opts ...*options.ListDatabasesOptions) (mongo.ListDatabasesResult, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", c, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if c != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return mongo.ListDatabasesResult{}, nil
}

// database hook
// mongo.Database.Drop()
// mongo.Database.Aggregate()
// mongo.Database.RunCommand()
// mongo.Database.ListCollections()

func mgDatabaseDropHook(db *mongo.Database, ctx context.Context) error {
	if GlobalMgr.ShouldRecord() {
		panic("should not hook mongo.Database.Drop() for record mode")
	}
	return nil
}

func mgDatabaseAggregateHook(db *mongo.Database, ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	c := db.Client()
	fv, _ := bson.Marshal(pipeline)
	key := buildKeyByClient(c, "Database.Aggregate", string(fv))

	return getCursorData("Database.Aggregate", key, c, func() (*mongo.Cursor, error) {
		return mgDatabaseAggregateTramp(db, ctx, pipeline, opts...)
	})
}

func mgDatabaseAggregateTramp(db *mongo.Database, ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", db, ctx, pipeline, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if db != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgDatabaseRunCommandHook(db *mongo.Database, ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) *mongo.SingleResult {
	c := db.Client()
	fv, _ := bson.Marshal(runCommand)
	key := buildKeyByClient(c, "Database.RunCommand", string(fv))

	return getSingleResultData("Database.RunCommand()", key, c, func() *mongo.SingleResult {
		return mgDatabaseRunCommandTramp(db, ctx, runCommand, opts...)
	})
}

func mgDatabaseRunCommandTramp(db *mongo.Database, ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) *mongo.SingleResult {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", db, ctx, runCommand, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if db != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgDatabaseRunCommandCursorHook(db *mongo.Database, ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) (*mongo.Cursor, error) {
	c := db.Client()
	fv, _ := bson.Marshal(runCommand)
	key := buildKeyByClient(c, "Database.RunCommandCursor", string(fv))

	return getCursorData("Database.RunCommandCursor", key, c, func() (*mongo.Cursor, error) {
		return mgDatabaseRunCommandCursorTramp(db, ctx, runCommand, opts...)
	})
}

func mgDatabaseRunCommandCursorTramp(db *mongo.Database, ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) (*mongo.Cursor, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", db, ctx, runCommand, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if db != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgDatabaseListCollectionsHook(db *mongo.Database, ctx context.Context, filter interface{}, opts ...*options.ListCollectionsOptions) (*mongo.Cursor, error) {
	c := db.Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Database.ListCollection", string(fv))

	return getCursorData("Database.ListCollection", key, c, func() (*mongo.Cursor, error) {
		return mgDatabaseListCollectionsTramp(db, ctx, filter, opts...)
	})
}

func mgDatabaseListCollectionsTramp(db *mongo.Database, ctx context.Context, filter interface{}, opts ...*options.ListCollectionsOptions) (*mongo.Cursor, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", db, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if db != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

// collection hook
// mongo.Collection.Drop()
// mongo.Collection.Find()
// mongo.Collection.Distinct()
// mongo.Collection.Aggregate()
// mongo.Collection.InsertOne()
// mongo.Collection.InsertMany()
// mongo.Collection.DeleteOne()
// mongo.Collection.DeleteMany()
// mongo.Collection.ReplaceOne()
// mongo.Collection.UpdateOne()
// mongo.Collection.UpdateMany()
// mongo.Collection.CountDocuments()
// mongo.Collection.EstimatedDocumentCount()

func mgCollectionDropHook(cl *mongo.Collection, ctx context.Context) error {
	if GlobalMgr.ShouldRecord() {
		panic("should not hook collection.Drop() in record mode")
	}
	return nil
}

func mgCollectionDistinctHook(cl *mongo.Collection, ctx context.Context, fieldName string, filter interface{},
	opts ...*options.DistinctOptions) ([]interface{}, error) {
	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.Distinct", string(fv)+fieldName)

	if GlobalMgr.ShouldRecord() {
		ret, err := mgCollectionDistinctTramp(cl, ctx, fieldName, filter, opts...)
		if err != nil {
			return ret, err
		}

		bv := make([]bsoncore.Value, 0, len(ret))
		for _, v := range ret {
			var val bsoncore.Value
			t, d, err2 := bson.MarshalValue(v)
			if err2 != nil {
				return nil, fmt.Errorf("marshal distinct returned value failed, err:%s", err2)
			}
			val.Type = t
			val.Data = d
			bv = append(bv, val)
		}

		dd, err := marshalValue(bv)
		if err != nil {
			return nil, fmt.Errorf("mongo.Collection.Distinct(), marshal failed, err:%s", err)
		}

		GlobalMgr.StoreValue(key, dd)
		return ret, err
	}

	dd, err := GlobalMgr.GetValue(key)
	if err != nil {
		return nil, fmt.Errorf("mongo.Collection.Distinct(), failed to get value from db, err:%s", err)
	}

	var v []bsoncore.Value
	err = unmarshalValue(dd, &v)
	if err != nil {
		return nil, fmt.Errorf("mongo.Collection.Distinct(), unmarshal failed, err:%s", err)
	}

	retArray := make([]interface{}, len(v))

	for i, val := range v {
		raw := bson.RawValue{Type: val.Type, Value: val.Data}
		err = raw.Unmarshal(&retArray[i])
		if err != nil {
			return nil, err
		}
	}

	return retArray, nil
}

func mgCollectionDistinctTramp(cl *mongo.Collection, ctx context.Context, fieldName string, filter interface{},
	opts ...*options.DistinctOptions) ([]interface{}, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v, %+v, %+v", cl, ctx, fieldName, filter)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgCollectionAggregateHook(db *mongo.Collection, ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	c := db.Database().Client()
	fv, _ := bson.Marshal(pipeline)
	key := buildKeyByClient(c, "Collection.Aggregate", string(fv))

	return getCursorData("Collection.Aggregate", key, c, func() (*mongo.Cursor, error) {
		return mgCollectionAggregateTramp(db, ctx, pipeline, opts...)
	})
}

func mgCollectionAggregateTramp(db *mongo.Collection, ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", db, ctx, pipeline, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if db != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func convertInt64ToBytes(val int64) []byte {
	var buff bytes.Buffer
	binary.Write(&buff, binary.LittleEndian, val)
	return buff.Bytes()
}

func getInt64FromBytes(val []byte) (int64, error) {
	buff := bytes.NewBuffer(val)

	var ret int64
	err := binary.Read(buff, binary.LittleEndian, &ret)
	return ret, err
}

func mgCollectionCountDocumentsHook(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.CountDocuments", string(fv))

	if GlobalMgr.ShouldRecord() {
		cnt, err := mgCollectionCountDocumentsTramp(cl, ctx, filter, opts...)
		if err != nil {
			return cnt, err
		}

		GlobalMgr.StoreValue(key, convertInt64ToBytes(cnt))
		return cnt, nil
	}

	d, err := GlobalMgr.GetValue(key)
	if err != nil {
		return 0, fmt.Errorf("mongo.Collection.CountDocuments(), failed to get value from db, err:%s", err)
	}

	cnt, err := getInt64FromBytes(d)
	if err != nil {
		return 0, fmt.Errorf("mongo.Collection.CountDocuments, failed to convert cnt from db data, err:%s", err)
	}

	return cnt, nil
}

func mgCollectionCountDocumentsTramp(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", cl, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return 0, nil
}

func mgCollectionEstimatedDocumentCountHook(cl *mongo.Collection, ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	c := cl.Database().Client()
	key := buildKeyByClient(c, "Collection.EstimateDocumentCount", string("nn"))

	if GlobalMgr.ShouldRecord() {
		cnt, err := mgCollectionEstimatedDocumentCountTramp(cl, ctx, opts...)
		if err != nil {
			return cnt, err
		}

		GlobalMgr.StoreValue(key, convertInt64ToBytes(cnt))
		return cnt, nil
	}

	d, err := GlobalMgr.GetValue(key)
	if err != nil {
		return 0, fmt.Errorf("mongo.Collection.EstimatedDocumentCount(), failed to get value from db, err:%s", err)
	}

	cnt, err := getInt64FromBytes(d)
	if err != nil {
		return 0, fmt.Errorf("mongo.Collection.EstimatedDocumentCount, failed to convert cnt from db data, err:%s", err)
	}

	return cnt, nil
}

func mgCollectionEstimatedDocumentCountTramp(cl *mongo.Collection, ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v, %+v", cl, ctx, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return 0, nil
}

func mgCollectionDeleteOneHook(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.DeleteOne", string(fv))

	if GlobalMgr.ShouldRecord() {
		ret, err := mgCollectionDeleteOneTramp(cl, ctx, filter, opts...)
		if err != nil {
			return ret, err
		}

		d, err := marshalValue(ret)
		if err != nil {
			return nil, fmt.Errorf("mongo.Collection.DeleteOne(), marshal failed, err:%s", err)
		}

		GlobalMgr.StoreValue(key, d)
		return ret, nil
	}

	val, err := GlobalMgr.GetValue(key)
	if err != nil {
		return nil, fmt.Errorf("mongo.Collection.DeleteOne(), failed to get value from db, err:%s", err)
	}

	var ret mongo.DeleteResult
	err = unmarshalValue(val, &ret)
	if err != nil {
		return nil, fmt.Errorf("mongo.Collection.DeleteOne, failed to convert cnt from db data, err:%s", err)
	}

	return &ret, nil
}

func mgCollectionDeleteOneTramp(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", cl, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgCollectionDeleteManyHook(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.DeleteMany", string(fv))

	if GlobalMgr.ShouldRecord() {
		ret, err := mgCollectionDeleteManyTramp(cl, ctx, filter, opts...)
		if err != nil {
			return ret, err
		}

		d, err := marshalValue(ret)
		if err != nil {
			return nil, fmt.Errorf("mongo.Collection.DeleteMany(), marshal failed, err:%s", err)
		}

		GlobalMgr.StoreValue(key, d)
		return ret, nil
	}

	val, err := GlobalMgr.GetValue(key)
	if err != nil {
		return nil, fmt.Errorf("mongo.Collection.DeleteOne(), failed to get value from db, err:%s", err)
	}

	var ret mongo.DeleteResult
	err = unmarshalValue(val, &ret)
	if err != nil {
		return nil, fmt.Errorf("mongo.Collection.DeleteOne, failed to convert cnt from db data, err:%s", err)
	}

	return &ret, nil
}

func mgCollectionDeleteManyTramp(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", cl, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgCollectionFindHook(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.Find", string(fv))

	return getCursorData("Collection.Find", key, c, func() (*mongo.Cursor, error) {
		return mgCollectionFindTramp(cl, ctx, filter, opts...)
	})
}

func mgCollectionFindTramp(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v", cl, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgCollectionFindOneAndReplaceHook(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.FindOneAndReplaceOptions) *mongo.SingleResult {

	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.FindOneAndReplace", string(fv))

	return getSingleResultData("Collection.FindOneAndReplace()", key, c, func() *mongo.SingleResult {
		return mgCollectionFindOneAndReplaceTramp(cl, ctx, filter, replacement, opts...)
	})
}

func mgCollectionFindOneAndReplaceTramp(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.FindOneAndReplaceOptions) *mongo.SingleResult {

	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v,%+v", cl, ctx, filter, replacement, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgCollectionFindOneAndDeleteHook(cl *mongo.Collection, ctx context.Context, filter interface{},
	opts ...*options.FindOneAndDeleteOptions) *mongo.SingleResult {

	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.FindOneAndDelete", string(fv))

	return getSingleResultData("Collection.FindOneAndDelete()", key, c, func() *mongo.SingleResult {
		return mgCollectionFindOneAndDeleteTramp(cl, ctx, filter, opts...)
	})
}

func mgCollectionFindOneAndDeleteTramp(cl *mongo.Collection, ctx context.Context, filter interface{},
	opts ...*options.FindOneAndDeleteOptions) *mongo.SingleResult {

	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v,%+v,%+v,%+v", cl, ctx, filter, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func mgCollectionFindOneAndUpdateHook(cl *mongo.Collection, ctx context.Context, filter interface{},
	update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {

	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.FindOneAndUpdate", string(fv))

	return getSingleResultData("Collection.FindOneAndUpdate()", key, c, func() *mongo.SingleResult {
		return mgCollectionFindOneAndUpdateTramp(cl, ctx, filter, update, opts...)
	})
}

func mgCollectionFindOneAndUpdateTramp(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {

	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v,%+v", cl, ctx, filter, replacement, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil
}

func getDocId(registry *bsoncodec.Registry, val interface{}) (interface{}, error) {
	if registry == nil {
		registry = bson.NewRegistryBuilder().Build()
	}

	switch tt := val.(type) {
	case nil:
		return nil, mongo.ErrNilDocument
	case bsonx.Doc:
		val = tt.Copy()
	case []byte:
		// Slight optimization so we'll just use MarshalBSON and not go through the codec machinery.
		val = bson.Raw(tt)
	}

	// TODO(skriptble): Use a pool of these instead.
	doc := make(bsoncore.Document, 0, 256)
	doc, err := bson.MarshalAppendWithRegistry(registry, doc, val)
	if err != nil {
		return nil, mongo.MarshalError{Value: val, Err: err}
	}

	var id interface{}

	value := doc.Lookup("_id")
	switch value.Type {
	case bsontype.Type(0):
		value = bsoncore.Value{Type: bsontype.ObjectID, Data: bsoncore.AppendObjectID(nil, primitive.NewObjectID())}
		olddoc := doc
		doc = make(bsoncore.Document, 0, len(olddoc)+17) // type byte + _id + null byte + object ID
		_, doc = bsoncore.ReserveLength(doc)
		doc = bsoncore.AppendValueElement(doc, "_id", value)
		doc = append(doc, olddoc[4:]...) // remove the length
		doc = bsoncore.UpdateLength(doc, 0, int32(len(doc)))
	default:
		// We copy the bytes here to ensure that any bytes returned to the user aren't modified
		// later.
		buf := make([]byte, len(value.Data))
		copy(buf, value.Data)
		value.Data = buf
	}

	err = bson.RawValue{Type: value.Type, Value: value.Data}.UnmarshalWithRegistry(registry, &id)
	if err != nil {
		return nil, err
	}

	return id, nil
}

func mgCollectionInsertOneHook(cl *mongo.Collection, ctx context.Context, document interface{},
	opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	if GlobalMgr.ShouldRecord() {
		panic("should not hook mongo.Collection.InsertOne for recording")
	}

	c := cl.Database().Client()
	registry := getRegistryByClient(c)

	id, err := getDocId(registry, document)
	if err != nil {
		return nil, err
	}

	return &mongo.InsertOneResult{InsertedID: id}, nil
}

func mgCollectionInsertManyHook(cl *mongo.Collection, ctx context.Context, documents []interface{},
	opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	if GlobalMgr.ShouldRecord() {
		panic("should not hook mongo.Collection.InsertMany for recording")
	}

	c := cl.Database().Client()
	registry := getRegistryByClient(c)

	ids := make([]interface{}, len(documents))
	for i := range documents {
		id, err := getDocId(registry, documents[i])
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}

	return &mongo.InsertManyResult{InsertedIDs: ids}, nil
}

func mgCollectionReplaceOneHook(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {

	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.ReplaceOne", string(fv))

	if GlobalMgr.ShouldRecord() {
		ret, err := mgCollectionReplaceOneTramp(cl, ctx, filter, replacement, opts...)
		if err != nil {
			return nil, err
		}

		d, err := marshalUpdateResult(ret, "mongo.Collection.ReplaceOne")
		if err != nil {
			return nil, err
		}

		GlobalMgr.StoreValue(key, d)
		return ret, err
	}

	d, err := GlobalMgr.GetValue(key)
	if err != nil {
		return nil, fmt.Errorf("marshal mongo.Collection.ReplaceOne get value from db failed, err:%s", err)
	}

	ret, err := unmarshalUpdateResult(d, "mongo.Collection.ReplaceOne")
	return ret, err
}

func mgCollectionReplaceOneTramp(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v,%+v", cl, ctx, filter, replacement, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}

func mgCollectionUpdateManyHook(cl *mongo.Collection, ctx context.Context, filter interface{},
	update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	c := cl.Database().Client()
	fv, _ := bson.Marshal(filter)
	key := buildKeyByClient(c, "Collection.UpdateMany", string(fv))

	if GlobalMgr.ShouldRecord() {
		ret, err := mgCollectionUpdateManyTramp(cl, ctx, filter, update, opts...)
		if err != nil {
			return nil, err
		}

		d, err := marshalUpdateResult(ret, "mongo.Collection.UpdteMany")
		if err != nil {
			return nil, err
		}

		GlobalMgr.StoreValue(key, d)
		return ret, err
	}

	d, err := GlobalMgr.GetValue(key)
	if err != nil {
		return nil, fmt.Errorf("marshal mongo.Collection.UpdateMany get value from db failed, err:%s", err)
	}

	ret, err := unmarshalUpdateResult(d, "mongo.Collection.UpdateMany")
	return ret, err
}

func mgCollectionUpdateManyTramp(cl *mongo.Collection, ctx context.Context, filter interface{},
	update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing:%+v, %+v,%+v,%+v,%+v", cl, ctx, filter, update, opts)

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if cl != nil {
		panic("trampoline function is not allowed to be called directlyis not allowed to be called")
	}

	return nil, nil
}
