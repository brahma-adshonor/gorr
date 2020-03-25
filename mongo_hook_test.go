package gorr

import (
	"context"
	"fmt"
	"testing"
	"unsafe"

	"github.com/brahma-adshonor/gohook"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

var (
	globalDummyClient = &mongo.Client{}
)

func init() {
	enableRegressionEngine(RegressionRecord)
	GlobalMgr.SetStorage(NewMapStorage(100))
	GlobalMgr.SetNotify(func(key, msg string, data []byte) {
		fmt.Printf("hook msg:%s,%s,datasize:%d\n", key, msg, len(data))
	})
}

func mgConnectDummy(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	return globalDummyClient, nil
}

func buildCursor() *mongo.Cursor {
	d := CursorData{
		Id:  233,
		Err: "dummy mongo err",
	}

	var doc1, doc2, elem1, elem2 []byte
	elem1 = bsoncore.AppendStringElement(elem1, "$key1", "value1")
	elem2 = bsoncore.AppendInt32Element(elem2, "$key2", int32(233))

	doc1 = bsoncore.BuildDocumentFromElements(doc1, elem1)
	doc2 = bsoncore.BuildDocumentFromElements(doc2, elem1, elem2)

	d.Data = append(d.Data, doc1)
	d.Data = append(d.Data, doc2)

	h := buildCursorHolder(d, nil)
	return &h.cursor
}

func checkCursor(t *testing.T, res *mongo.Cursor) {
	var docs []bson.D
	err := res.All(context.Background(), &docs)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(docs))
	ep1 := bson.D{{Key: "$key1", Value: "value1"}}
	assert.Equal(t, ep1, docs[0])
	ep2 := bson.D{{Key: "$key1", Value: "value1"}, {Key: "$key2", Value: int32(233)}}
	assert.Equal(t, ep2, docs[1])

	assert.Equal(t, int64(233), res.ID())
	assert.Equal(t, "dummy mongo err", res.Err().Error())

	assert.False(t, res.Next(context.Background()))
	assert.False(t, res.TryNext(context.Background()))

	assert.Nil(t, res.Close(context.Background()))
}

func TestMgConnect(t *testing.T) {
	fmt.Printf("testing connect")
	GlobalMgr.SetState(RegressionRecord)

	err := EnableMongoHook()
	assert.Nil(t, err)
	defer DisableMongoHook()

	err = gohook.Hook(mgConnectTramp, mgConnectDummy, nil)
	assert.Nil(t, err)

	c, err := mongo.Connect(context.Background())
	assert.Nil(t, err)

	p1 := unsafe.Pointer(c)
	p2 := unsafe.Pointer(globalDummyClient)
	assert.NotEqual(t, p1, p2)

	cc := getClientHolder(c)
	assert.NotNil(t, cc)

	c3 := &cc.client
	p3 := unsafe.Pointer(c3)
	assert.Equal(t, p1, p3)

	cc2 := getClientHolder(globalDummyClient)
	assert.Nil(t, cc2)
}

func TestMongoCursorHook(t *testing.T) {
	GlobalMgr.SetState(RegressionRecord)
	err := EnableMongoHook()
	assert.Nil(t, err)
	defer DisableMongoHook()

	cursor := buildCursor()
	checkCursor(t, cursor)

	var dd bson.D
	assert.NotNil(t, cursor.Decode(&dd))

	h := getCursorHolder(cursor)
	h.cur--
	assert.True(t, h.cursor.Next(context.Background()))
	assert.Nil(t, h.cursor.Decode(&dd))

	ep2 := bson.D{{Key: "$key1", Value: "value1"}, {Key: "$key2", Value: int32(233)}}
	assert.Equal(t, ep2, dd)
}

func buildSingleResult() *mongo.SingleResult {
	var doc, elem1, elem2 []byte
	elem1 = bsoncore.AppendStringElement(elem1, "$key1", "value1")
	elem2 = bsoncore.AppendInt32Element(elem2, "$key2", int32(233))
	doc = bsoncore.BuildDocumentFromElements(doc, elem1, elem2)

	d := SingleResultData{
		Data: doc,
		Err:  "", //fmt.Errorf("dummy mongo err"),
	}

	h := buildSingleResultHolder(d, nil)
	return &h.ret
}

func checkSingleResult(t *testing.T, sg *mongo.SingleResult) {
	var doc, elem1, elem2 []byte
	elem1 = bsoncore.AppendStringElement(elem1, "$key1", "value1")
	elem2 = bsoncore.AppendInt32Element(elem2, "$key2", int32(233))
	doc = bsoncore.BuildDocumentFromElements(doc, elem1, elem2)

	var dd bson.D
	assert.Nil(t, sg.Decode(&dd))
	ep := bson.D{{Key: "$key1", Value: "value1"}, {Key: "$key2", Value: int32(233)}}
	assert.Equal(t, dd, ep)

	dd2, err := sg.DecodeBytes()
	assert.Nil(t, err)
	assert.Equal(t, doc, []byte(dd2))

	h := getSingleResultHolder(sg)
	h.sd.Err = "dummy mongo err"
	assert.Equal(t, "dummy mongo err", sg.Err().Error())
}

func TestMongoSingleResultHook(t *testing.T) {
	GlobalMgr.SetState(RegressionRecord)
	err := EnableMongoHook()
	assert.Nil(t, err)
	defer DisableMongoHook()

	sg := buildSingleResult()
	checkSingleResult(t, sg)
}

func mgDummyConnectHook(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	return mongo.NewClient(opts...)
}

func mgDummyClientListDatabasesHook(c *mongo.Client, ctx context.Context, filter interface{}, opts ...*options.ListDatabasesOptions) (mongo.ListDatabasesResult, error) {
	return mongo.ListDatabasesResult{
		TotalSize: 233,
		Databases: []mongo.DatabaseSpecification{
			mongo.DatabaseSpecification{Name: "n1", SizeOnDisk: 244, Empty: false},
			mongo.DatabaseSpecification{Name: "n2", SizeOnDisk: 254, Empty: false},
		},
	}, nil
}

func mgDummyDatabaseRunCommand(db *mongo.Database, ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) *mongo.SingleResult {
	return buildSingleResult()
}

func mgDummyDatabaseRunCommandCursor(db *mongo.Database, ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) (*mongo.Cursor, error) {
	c := buildCursor()
	return c, nil
}

func mgDummyCollectionCountDocuments(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	return 233, nil
}

func mgDummyCollectionEstimatedDocumentCount(cl *mongo.Collection, ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	return 233, nil
}

func mgDummyCollectionDelete(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	return &mongo.DeleteResult{DeletedCount: 233}, nil
}

var (
	distinctAll = []interface{}{int32(11), int32(22), "dummy", int32(4), int32(5)}
)

func mgDummyCollectionDistinct(cl *mongo.Collection, ctx context.Context, fieldName string, filter interface{},
	opts ...*options.DistinctOptions) ([]interface{}, error) {
	return distinctAll, nil
}

func mgDummyCollectionFind(cl *mongo.Collection, ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	c := buildCursor()
	return c, nil
}

func mgDummyCollectionFindOneAndReplace(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.FindOneAndReplaceOptions) *mongo.SingleResult {
	return buildSingleResult()
}

func mgDummyCollectionFindOneAndDelete(cl *mongo.Collection, ctx context.Context, filter interface{},
	opts ...*options.FindOneAndDeleteOptions) *mongo.SingleResult {
	return buildSingleResult()
}

func mgDummyCollectionFindOneAndUpdate(cl *mongo.Collection, ctx context.Context, filter interface{},
	update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	fmt.Printf("@@@@@@@@@@@@@@@@@@@@@\n")
	return buildSingleResult()
}

func buildUpdateResult() *mongo.UpdateResult {
	d1 := bson.D{{Key: "v22_id", Value: 233}}
	id1, _ := getDocId(nil, d1)

	return &mongo.UpdateResult{
		MatchedCount:  233,
		ModifiedCount: 244,
		UpsertedCount: 255,
		UpsertedID:    id1,
	}
}

func mgDummyCollectionUpdateMany(cl *mongo.Collection, ctx context.Context, filter interface{},
	update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return buildUpdateResult(), nil
}

func mgDummyCollectionReplaceOne(cl *mongo.Collection, ctx context.Context, filter interface{},
	replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	return buildUpdateResult(), nil
}

func mgDummuyCollectionAggregate(db *mongo.Collection, ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	return buildCursor(), nil
}

func mgDummyDatabaseAggregate(db *mongo.Database, ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	return buildCursor(), nil
}

func mgDummyDatabaseListCollections(db *mongo.Database, ctx context.Context, filter interface{}, opts ...*options.ListCollectionsOptions) (*mongo.Cursor, error) {
	return buildCursor(), nil
}

func TestMongoClientHook(t *testing.T) {
	var r1, r2 mongo.ListDatabasesResult
	{
		GlobalMgr.SetState(RegressionRecord)
		err := EnableMongoHook()
		assert.Nil(t, err)
		defer DisableMongoHook()

		gohook.Hook(mgConnectTramp, mgDummyConnectHook, nil)
		defer gohook.UnHook(mgConnectTramp)
		gohook.Hook(mgClientListDatabasesTramp, mgDummyClientListDatabasesHook, nil)
		defer gohook.UnHook(mgDummyClientListDatabasesHook)
		gohook.Hook(mgDatabaseRunCommandTramp, mgDummyDatabaseRunCommand, nil)
		defer gohook.UnHook(mgDatabaseRunCommandHook)
		gohook.Hook(mgDatabaseRunCommandCursorTramp, mgDummyDatabaseRunCommandCursor, nil)
		defer gohook.UnHook(mgDatabaseRunCommandCursorHook)
		gohook.Hook(mgCollectionCountDocumentsTramp, mgDummyCollectionCountDocuments, nil)
		defer gohook.UnHook(mgCollectionCountDocumentsTramp)
		gohook.Hook(mgCollectionDeleteManyTramp, mgDummyCollectionDelete, nil)
		defer gohook.UnHook(mgCollectionDeleteManyTramp)
		gohook.Hook(mgCollectionDeleteOneTramp, mgDummyCollectionDelete, nil)
		defer gohook.UnHook(mgCollectionDeleteOneTramp)
		gohook.Hook(mgCollectionDistinctTramp, mgDummyCollectionDistinct, nil)
		defer gohook.UnHook(mgCollectionDistinctTramp)
		gohook.Hook(mgCollectionFindTramp, mgDummyCollectionFind, nil)
		defer gohook.UnHook(mgCollectionFindTramp)
		gohook.Hook(mgCollectionFindOneAndReplaceTramp, mgDummyCollectionFindOneAndReplace, nil)
		defer gohook.UnHook(mgCollectionFindOneAndReplaceTramp)
		gohook.Hook(mgCollectionFindOneAndDeleteTramp, mgDummyCollectionFindOneAndDelete, nil)
		defer gohook.UnHook(mgCollectionFindOneAndDeleteTramp)
		gohook.Hook(mgCollectionEstimatedDocumentCountTramp, mgDummyCollectionEstimatedDocumentCount, nil)
		defer gohook.UnHook(mgCollectionEstimatedDocumentCountTramp)

		assert.Nil(t, gohook.Hook(mgCollectionFindOneAndUpdateTramp, mgDummyCollectionFindOneAndUpdate, nil))
		defer gohook.UnHook(mgCollectionFindOneAndUpdateTramp)
		assert.Nil(t, gohook.Hook(mgCollectionUpdateManyTramp, mgDummyCollectionUpdateMany, nil))
		defer gohook.UnHook(mgCollectionUpdateManyTramp)
		assert.Nil(t, gohook.Hook(mgCollectionReplaceOneTramp, mgDummyCollectionReplaceOne, nil))
		defer gohook.UnHook(mgCollectionReplaceOneTramp)
		assert.Nil(t, gohook.Hook(mgCollectionAggregateTramp, mgDummuyCollectionAggregate, nil))
		defer gohook.UnHook(mgCollectionAggregateTramp)
		assert.Nil(t, gohook.Hook(mgDatabaseAggregateTramp, mgDummyDatabaseAggregate, nil))
		defer gohook.UnHook(mgDatabaseAggregateTramp)
		assert.Nil(t, gohook.Hook(mgDatabaseListCollectionsTramp, mgDummyDatabaseListCollections, nil))
		defer gohook.UnHook(mgDatabaseListCollectionsTramp)

		opt := options.Client().ApplyURI("mongodb://localhost:27017")
		c1, err1 := mongo.Connect(context.Background(), opt)
		assert.Nil(t, err1)
		assert.NotNil(t, c1)
		assert.NotNil(t, getClientHolder(c1))

		// test client
		filter := bson.D{{Key: "key1", Value: "value1"}}
		r1, err = c1.ListDatabases(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), r1.TotalSize)
		assert.Equal(t, 2, len(r1.Databases))
		assert.Equal(t, "n1", r1.Databases[0].Name)
		assert.Equal(t, "n2", r1.Databases[1].Name)

		// test database
		db := c1.Database("db1")
		assert.NotNil(t, db)

		cmd1 := bson.D{{Key: "x", Value: 1}}
		res1 := db.RunCommand(context.Background(), cmd1)
		assert.NotNil(t, res1)
		assert.Nil(t, res1.Err())
		checkSingleResult(t, res1)

		cr2, err := db.Aggregate(context.Background(), cmd1)
		checkCursor(t, cr2)

		cmd2 := bson.D{{Key: "$xy", Value: 1}}
		cr3, err := db.ListCollections(context.Background(), cmd2)
		checkCursor(t, cr3)

		// database RunCommandCursor
		res2, err := db.RunCommandCursor(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, res2)
		checkCursor(t, res2)

		// test collection
		cl := db.Collection("cl1")
		assert.NotNil(t, cl)

		cnt, err := cl.CountDocuments(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), cnt)

		cnt2, err := cl.EstimatedDocumentCount(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, int64(233), cnt2)

		resd1, err := cl.DeleteMany(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), resd1.DeletedCount)

		resd2, err := cl.DeleteOne(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), resd2.DeletedCount)

		rdis1, err := cl.Distinct(context.Background(), "xxx", cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, rdis1)
		assert.Equal(t, distinctAll, rdis1)

		rf1, err := cl.Find(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, rf1)
		checkCursor(t, rf1)

		rfr1 := cl.FindOneAndReplace(context.Background(), cmd1, cmd2)
		checkSingleResult(t, rfr1)
		rfr2 := cl.FindOneAndDelete(context.Background(), cmd1)
		checkSingleResult(t, rfr2)
		rfr3 := cl.FindOneAndUpdate(context.Background(), cmd1, cmd2)
		checkSingleResult(t, rfr3)

		ru1, err := cl.UpdateMany(context.Background(), cmd1, cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, ru1.UpsertedID)
		ru2 := buildUpdateResult()
		ru1.UpsertedID = nil
		ru2.UpsertedID = nil
		assert.Equal(t, ru2, ru1)

		cmd3 := bson.D{{Key: "xy", Value: 1}}
		ru3, err := cl.ReplaceOne(context.Background(), cmd1, cmd3)
		assert.Nil(t, err)
		assert.NotNil(t, ru3.UpsertedID)
		ru3.UpsertedID = nil
		assert.Equal(t, ru2, ru3)

		cr1, err := cl.Aggregate(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, cr1)
		checkCursor(t, cr1)
	}

	{
		GlobalMgr.SetState(RegressionReplay)
		err := EnableMongoHook()
		assert.Nil(t, err)
		defer DisableMongoHook()

		// test client
		opt := options.Client().ApplyURI("mongodb://localhost:27017")
		c1, err1 := mongo.Connect(context.Background(), opt)
		assert.Nil(t, err1)
		assert.NotNil(t, c1)
		assert.NotNil(t, getClientHolder(c1))

		assert.Nil(t, c1.Disconnect(context.Background()))

		filter := bson.D{{Key: "key1", Value: "value1"}}
		r2, err = c1.ListDatabases(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, r1, r2)

		// test database
		db := c1.Database("db1")
		assert.NotNil(t, db)
		assert.Nil(t, db.Drop(context.TODO()))

		cmd1 := bson.D{{Key: "x", Value: 1}}
		res1 := db.RunCommand(context.Background(), cmd1)
		assert.NotNil(t, res1)
		assert.Nil(t, res1.Err())
		checkSingleResult(t, res1)

		cr2, err := db.Aggregate(context.Background(), cmd1)
		checkCursor(t, cr2)
		cmd2 := bson.D{{Key: "$xy", Value: 1}}
		cr3, err := db.ListCollections(context.Background(), cmd2)
		checkCursor(t, cr3)

		// database RunCommandCursor
		res2, err := db.RunCommandCursor(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, res2)
		checkCursor(t, res2)

		// test collection
		cl := db.Collection("cl1")
		assert.NotNil(t, cl)
		assert.Nil(t, cl.Drop(context.Background()))

		cnt, err := cl.CountDocuments(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), cnt)

		cnt2, err := cl.EstimatedDocumentCount(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, int64(233), cnt2)

		resd1, err := cl.DeleteMany(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), resd1.DeletedCount)
		resd2, err := cl.DeleteOne(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.Equal(t, int64(233), resd2.DeletedCount)

		rdis1, err := cl.Distinct(context.Background(), "xxx", cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, rdis1)
		assert.Equal(t, distinctAll, rdis1)

		// find
		rf1, err := cl.Find(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, rf1)
		checkCursor(t, rf1)

		rfr1 := cl.FindOneAndReplace(context.Background(), cmd1, cmd2)
		checkSingleResult(t, rfr1)
		rfr2 := cl.FindOneAndDelete(context.Background(), cmd1)
		checkSingleResult(t, rfr2)
		rfr3 := cl.FindOneAndUpdate(context.Background(), cmd1, cmd2)
		checkSingleResult(t, rfr3)

		docs2 := []interface{}{bson.D{{Key: "$vv", Value: 23}}, bson.D{{Key: "$v2", Value: 2}}}
		r2, err := cl.InsertMany(context.Background(), docs2)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(r2.InsertedIDs))

		r3, err := cl.InsertOne(context.Background(), bson.D{{Key: "$vv", Value: 1}})
		assert.Nil(t, err)
		assert.NotNil(t, r3.InsertedID)

		ru1, err := cl.UpdateMany(context.Background(), cmd1, cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, ru1.UpsertedID)
		ru2 := buildUpdateResult()
		ru1.UpsertedID = nil
		ru2.UpsertedID = nil
		assert.Equal(t, ru2, ru1)

		cmd3 := bson.D{{Key: "xy", Value: 1}}
		ru3, err := cl.ReplaceOne(context.Background(), cmd1, cmd3)
		assert.Nil(t, err)
		assert.NotNil(t, ru3.UpsertedID)
		ru3.UpsertedID = nil
		assert.Equal(t, ru2, ru3)

		cr1, err := cl.Aggregate(context.Background(), cmd2)
		assert.Nil(t, err)
		assert.NotNil(t, cr1)
		checkCursor(t, cr1)
	}
}
