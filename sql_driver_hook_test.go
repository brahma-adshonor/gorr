package regression

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/brahma-adshonor/gohook"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"io"
	"reflect"
	"testing"
	"time"
)

func TestUnmarshalInterface(t *testing.T) {
	tm := time.Date(2019, time.Month(8), 8, 12, 22, 33, 44, time.UTC)
	d := sqlResultSet{
		ColumnNames: []string{"c1", "c2", "c3"},
		RowData: &sqlRowData{
			FieldInfo: []sqlField{sqlField{DatabaseType: "n1", ScanType: scanTypeIdInt32},
				sqlField{DatabaseType: "n2", ScanType: scanTypeIdString},
				sqlField{DatabaseType: "n3", ScanType: scanTypeIdRawBytes},
				sqlField{DatabaseType: "n4", ScanType: scanTypeIdNullTime}},
			Rows: [][]driver.Value{[]driver.Value{int32(12), "m", []byte{1, 2, 3, 4}, tm}}}}

	sd, err := json.Marshal(d)
	assert.Nil(t, err)
	fmt.Printf("@@@@@@@@@@@@@@testing unmarshal@@@@@@@@@@@@\n%s\n", string(sd))

	var d2 sqlResultSet
	err = json.Unmarshal(sd, &d2)

	assert.Nil(t, err)
	assert.Equal(t, d.RowData.Rows, d2.RowData.Rows)
}

type sqlData struct {
	intputNum int
	col       []string
	types     []reflect.Type
	values    [][]interface{}
}

func genTime(s string) time.Time {
	layout := "2006-01-02"
	t, _ := time.Parse(layout, s)
	return t
}

type dummyResultTable struct {
	cur  int
	data sqlData
}

func (rows *dummyResultTable) Columns() []string {
	return rows.data.col
}

func (rows *dummyResultTable) Close() error {
	return nil
}

func (rows *dummyResultTable) Next(dest []driver.Value) error {
	fmt.Printf("dumyResultTable.Next, cur:%d, total:%d\n", rows.cur, len(rows.data.values))
	if rows.cur >= len(rows.data.values) {
		return io.EOF
	}

	for i := range rows.data.values[rows.cur] {
		dest[i] = rows.data.values[rows.cur][i]
	}

	rows.cur++
	return nil
}

func (rows *dummyResultTable) ColumnTypeLength(i int) (int64, bool) {
	return int64(rows.data.types[i].Size()), true
}

func (rows *dummyResultTable) ColumnTypeNullable(i int) (nullable, ok bool) {
	return true, true
}

func (rows *dummyResultTable) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	return 0, 0, false
}

func (rows *dummyResultTable) ColumnTypeDatabaseTypeName(i int) string {
	return rows.data.col[i]
}

func (rows *dummyResultTable) ColumnTypeScanType(i int) reflect.Type {
	return rows.data.types[i]
}

// implement driver.Result interface.
// representing result from executing db cmd
type dummyResult struct {
	data sqlData
}

func (res *dummyResult) LastInsertId() (int64, error) {
	return int64(len(res.data.values) - 1), nil
}

func (res *dummyResult) RowsAffected() (int64, error) {
	return int64(len(res.data.values)), nil
}

type dummyTx struct {
}

func (tx *dummyTx) Commit() (err error) {
	return fmt.Errorf("no tx commit is allowed in replay state")
}

func (tx *dummyTx) Rollback() (err error) {
	return fmt.Errorf("no tx rollback is allowed in replay state")
}

type dummyStmt struct {
	query string
	data  sqlData
}

func (stmt *dummyStmt) Close() error {
	return nil
}

func (stmt *dummyStmt) NumInput() int {
	return stmt.data.intputNum
}

func (stmt *dummyStmt) Exec(args []driver.Value) (driver.Result, error) {
	if len(args) != stmt.data.intputNum {
		panic("inputnum doesn't match")
	}
	return &dummyResult{data: stmt.data}, nil
}

func (stmt *dummyStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if len(args) != stmt.data.intputNum {
		panic("inputnum doesn't match")
	}

	fmt.Printf("dummyStmt.ExecContext, %v, \ndata:%v\n", args, stmt.data)
	return &dummyResult{data: stmt.data}, nil
}

func (stmt *dummyStmt) Query(args []driver.Value) (driver.Rows, error) {
	if len(args) != stmt.data.intputNum {
		panic("inputnum doesn't match")
	}

	fmt.Printf("dummyStmt.Query, %v, \ndata:%v\n", args, stmt.data)
	return &dummyResultTable{cur: 0, data: stmt.data}, nil
}

func (stmt *dummyStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != stmt.data.intputNum {
		panic("inputnum doesn't match")
	}

	fmt.Printf("dummyStmt.QueryContext, %v, \ndata:%v\n", args, stmt.data)
	return &dummyResultTable{cur: 0, data: stmt.data}, nil
}

type dummyConn struct {
	dsn  string
	data sqlData
}

func (c *dummyConn) Close() error {
	return nil
}

func (conn *dummyConn) Ping(ctx context.Context) error {
	fmt.Printf("dummyConn ping\n")
	return nil
}

func (c *dummyConn) Prepare(query string) (driver.Stmt, error) {
	return &dummyStmt{query: query, data: c.data}, nil
}

func (c *dummyConn) Begin() (driver.Tx, error) {
	return &dummyTx{}, nil
}

func (conn *dummyConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	if len(args) != conn.data.intputNum {
		panic("inputnum doesn't match")
	}

	fmt.Printf("dummyConn.Query, %v, \ndata:%v\n", args, conn.data)
	return &dummyResultTable{cur: 0, data: conn.data}, nil
}

var (
	stringType = reflect.TypeOf("")
	int32Type  = reflect.TypeOf(int32(0))
	timeType   = reflect.TypeOf(time.Now())
	byteType   = reflect.TypeOf([]byte("a"))

	db_data = map[string]sqlData{
		"miliao:secret@/127.0.0.0:233": sqlData{
			intputNum: 2,
			col:       []string{"id", "age", "name", "birth"},
			types:     []reflect.Type{stringType, int32Type, stringType, timeType},
			values: [][]interface{}{[]interface{}{"m1", 23, "miliao", genTime("1987-01-01")},
				[]interface{}{"m11", 33, "miliao2", genTime("1987-02-01")},
				[]interface{}{"m21", 43, "miliao3", genTime("1987-03-01")},
			},
		},
		"miliao:secret@/127.0.0.0:235": sqlData{
			intputNum: 3,
			col:       []string{"id", "age", "name", "birth", "pic"},
			types:     []reflect.Type{stringType, int32Type, stringType, timeType, byteType},
			values:    [][]interface{}{[]interface{}{"m3", 42, "QQ", genTime("1987-02-01"), []byte{1, 2, 3, 4, 42}}},
		},
		"miliao:secret@/127.0.0.0:236?parseTime=true&loc=Local": sqlData{
			intputNum: 3,
			col:       []string{"id", "age", "name", "birth", "pic"},
			types:     []reflect.Type{stringType, int32Type, stringType, timeType, byteType},
			values: [][]interface{}{
				[]interface{}{"m3", 42, "QQ", genTime("1987-02-01"), []byte{1, 2, 3, 4, 42}},
				[]interface{}{"m4", 32, "QQ", genTime("1987-03-01"), []byte{1, 2, 3, 4, 42}},
			},
		},
	}
)

func mysqlOpenHook(d mysql.MySQLDriver, dsn string) (driver.Conn, error) {
	fmt.Printf("MySQLDriver.Open() hook, dsn:%s\n", dsn)

	v, ok := db_data[dsn]
	if !ok {
		panic(fmt.Sprintf("unkonw dsn for test:%s", dsn))
	}

	return &dummyConn{dsn: dsn, data: v}, nil
}

func setupSqlHook(t *testing.T) {
	fmt.Printf("setup sql hook\n")

	GlobalMgr.SetState(RegressionRecord)

	var dr mysql.MySQLDriver
	err := gohook.HookMethod(dr, "Open", mysqlOpenHook, nil)
	assert.Nil(t, err)

	err = HookMysqlDriver()
	assert.Nil(t, err)
}

func TestOpenDb(t *testing.T) {
	setupSqlHook(t)
	run_test(t)
	test_stmt_op(t)
	UnHookMysqlDriver()

	fmt.Printf("done recording sql driver, now to replay.....\n")

	GlobalMgr.SetState(RegressionReplay)
	err := HookMysqlDriver()
	assert.Nil(t, err)
	run_test(t)
	test_stmt_op(t)
}

func run_test(t *testing.T) {
	var id string
	var age int64
	var name string
	var birth time.Time
	var dd []byte

	// setup query
	dsn1 := "miliao:secret@/127.0.0.0:233"
	db, err := sql.Open("mysql", dsn1)
	assert.Nil(t, err)
	assert.NotNil(t, db)

	q1 := "select * from dummy where id < ? limit ?"
	stmt, err2 := db.Prepare(q1)
	assert.Nil(t, err2)

	ret, err3 := stmt.Query(42, 24)
	assert.Nil(t, err3)

	// verify output

	// result 1
	b1 := ret.Next()
	assert.True(t, b1)
	err4 := ret.Scan(&id, &age, &name, &birth)
	assert.Nil(t, err4)

	assert.Equal(t, "m1", id)
	assert.Equal(t, int64(23), age)
	assert.Equal(t, "miliao", name)
	assert.Equal(t, genTime("1987-01-01"), birth)

	b2 := ret.Next()
	assert.True(t, b2)
	err5 := ret.Scan(&id, &age, &name, &birth)
	assert.Nil(t, err5)

	assert.Equal(t, "m11", id)
	assert.Equal(t, int64(33), age)
	assert.Equal(t, "miliao2", name)
	assert.Equal(t, genTime("1987-02-01"), birth)

	// result 2
	b2 = ret.Next()
	assert.True(t, b2)
	err = ret.Scan(&id, &age, &name, &birth)
	assert.Nil(t, err)

	assert.Equal(t, "m21", id)
	assert.Equal(t, int64(43), age)
	assert.Equal(t, "miliao3", name)
	assert.Equal(t, genTime("1987-03-01"), birth)

	b2 = ret.Next()
	assert.False(t, b2)
	stmt.Close()
	db.Close()

	// another query
	dsn2 := "miliao:secret@/127.0.0.0:235"
	db, err = sql.Open("mysql", dsn2)
	assert.Nil(t, err)
	assert.NotNil(t, db)

	q2 := "select * from dummy where id < ? and name = ? limit ?"
	stmt, err = db.Prepare(q2)
	assert.Nil(t, err)

	ret, err = stmt.Query(42, 24, "vv")
	assert.Nil(t, err)

	b := ret.Next()
	assert.True(t, b)
	err = ret.Scan(&id, &age, &name, &birth, &dd)
	assert.Nil(t, err)

	assert.Equal(t, "m3", id)
	assert.Equal(t, int64(42), age)
	assert.Equal(t, "QQ", name)
	assert.Equal(t, genTime("1987-02-01"), birth)
	assert.Equal(t, []byte{1, 2, 3, 4, 42}, dd)

	b = ret.Next()
	assert.False(t, b)
	stmt.Close()
	db.Close()
}

func test_stmt_op(t *testing.T) {
	var id string
	var age int64
	var name string
	var birth time.Time
	var dd []byte

	dsn := "miliao:secret@/127.0.0.0:236?parseTime=true&loc=Local"
	db, err := sql.Open("mysql", dsn)
	assert.Nil(t, err)
	assert.NotNil(t, db)

	q := "select * from dummy where id < ? and name = ? limit ?"
	stmt, err := db.Prepare(q)
	assert.Nil(t, err)

	ret, err := stmt.QueryContext(context.TODO(), 42, 24, "vv")
	assert.Nil(t, err)

	b := ret.Next()
	assert.True(t, b)
	err = ret.Scan(&id, &age, &name, &birth, &dd)
	assert.Nil(t, err)

	assert.Equal(t, "m3", id)
	assert.Equal(t, int64(42), age)
	assert.Equal(t, "QQ", name)
	assert.Equal(t, genTime("1987-02-01"), birth)
	assert.Equal(t, []byte{1, 2, 3, 4, 42}, dd)

	b = ret.Next()
	assert.True(t, b)
	b = ret.Next()
	assert.False(t, b)
	stmt.Close()

	// test conn
	conn, err22 := db.Conn(context.TODO())
	assert.Nil(t, err22)
	ret2, err33 := conn.QueryContext(context.TODO(), q, 42, 24, "vv")
	assert.Nil(t, err33)

	// verify
	bb := ret2.Next()
	assert.True(t, bb)
	err = ret2.Scan(&id, &age, &name, &birth, &dd)
	assert.Nil(t, err)

	assert.Equal(t, "m3", id)
	assert.Equal(t, int64(42), age)
	assert.Equal(t, "QQ", name)
	assert.Equal(t, genTime("1987-02-01"), birth)
	assert.Equal(t, []byte{1, 2, 3, 4, 42}, dd)

	b = ret2.Next()
	assert.True(t, b)
	b = ret2.Next()
	assert.False(t, b)

	conn.Close()
	// end conn test

	q2 := "insert into some_table values(?,?,?)"
	stmt2, err2 := db.Prepare(q2)
	assert.Nil(t, err2)

	r2, err3 := stmt2.ExecContext(context.TODO(), 1, "2", 2.3)
	assert.Nil(t, err3)

	last_id, err4 := r2.LastInsertId()
	affected_rows, err5 := r2.RowsAffected()

	assert.Nil(t, err4)
	assert.Nil(t, err5)

	assert.Equal(t, int64(1), last_id)
	assert.Equal(t, int64(2), affected_rows)

	stmt.Close()
	db.Close()
}
