package gorr

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/brahma-adshonor/gohook"
	"github.com/go-sql-driver/mysql"
)

const (
	SqlHookXxxDriver   = "SqlHookXxxDriver"
	SqlHookMysqlDriver = "SqlHookMysqlDriver"
)

func genSqlHookDataKey(tag, dsn, query, input string) string {
	return fmt.Sprintf("sql_driver_hook_prefix@@%s@@%s@@%s@@%s", tag, dsn, query, input)
}

func stringifySqlParam(args []driver.Value) string {
	total := len(args)
	output := make([]byte, 0, 8*total)
	output = append(output, []byte(fmt.Sprintf("sz#%d@@", total))...)
	for _, arg := range args {
		if arg == nil {
			continue
		}
		output = append(output, []byte(fmt.Sprintf("%s#%v@@", reflect.TypeOf(arg).Name(), arg))...)
	}

	return string(output)
}

func extractRowResult(rows driver.Rows, res *sqlResultSet) error {
	res.RowData = &sqlRowData{}
	res.ColumnNames = append(res.ColumnNames, rows.Columns()...)
	res.RowData.FieldInfo = make([]sqlField, len(res.ColumnNames))
	for i := range res.RowData.FieldInfo {
		attr := &res.RowData.FieldInfo[i]
		if prop, ok := rows.(driver.RowsColumnTypeScanType); ok {
			attr.ScanType = scanTypeId(prop.ColumnTypeScanType(i))
		} else {
			attr.ScanType = scanTypeId(reflect.TypeOf(new(interface{})).Elem())
		}
		if prop, ok := rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
			attr.DatabaseType = prop.ColumnTypeDatabaseTypeName(i)
		}
		if prop, ok := rows.(driver.RowsColumnTypeLength); ok {
			attr.Length, attr.HasLength = prop.ColumnTypeLength(i)
		}
		if prop, ok := rows.(driver.RowsColumnTypeNullable); ok {
			attr.Nullable, attr.HasNullable = prop.ColumnTypeNullable(i)
		}
		if prop, ok := rows.(driver.RowsColumnTypePrecisionScale); ok {
			attr.Scale.Precision, attr.Scale.Scale, attr.Scale.Ok = prop.ColumnTypePrecisionScale(i)
		}
	}

	for {
		vs := make([]driver.Value, len(res.ColumnNames))
		err := rows.Next(vs)
		if err != nil {
			if err != io.EOF {
				return err
			}

			next, ok := rows.(driver.RowsNextResultSet)
			if !ok {
				break
			}

			if !next.HasNextResultSet() {
				break
			}

			err = next.NextResultSet()
			if err != nil {
				return err
			}
		} else {
			res.RowData.Rows = append(res.RowData.Rows, vs)
		}
	}

	return nil
}

func storeRowsValue(key string, rows *sqlResultSet) error {
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}

	GlobalMgr.StoreValue(key, data)
	return nil
}

func doQuery(key string, doer func() (driver.Rows, error)) (driver.Rows, error) {
	rows := &sqlResultTable{}
	if GlobalMgr.ShouldRecord() {
		r, err := doer()
		if err == nil {
			err = extractRowResult(r, &rows.rs)
			if err != nil {
				GlobalMgr.notifier("sql driver hook extractRowResult failed", key, []byte(err.Error()))
				return nil, err
			}
		} else {
			if err == driver.ErrSkip {
				// ErrSkip is no harm, we need to return this exact error, so that sql api could issue an retry to stmt
				GlobalMgr.notifier("sql driver hook Queryer return ErrSkip", key, []byte(err.Error()))
			} else {
				GlobalMgr.notifier("sql driver hook Queryer failed", key, []byte(err.Error()))
			}

			// store an error
			rows.rs.Err = err.Error()
			storeRowsValue(key, &rows.rs)

			return nil, err
		}

		err = storeRowsValue(key, &rows.rs)
		if err != nil {
			GlobalMgr.notifier("sql driver hook store result failed", key, []byte(err.Error()))
			return nil, fmt.Errorf("store sql data failed, err:%s", err)
		}

		GlobalMgr.notifier("sql driver hook store result done", key, []byte(""))
		return rows, err
	} else {
		v, err := GlobalMgr.GetValue(key)
		if err != nil {
			GlobalMgr.notifier("sql driver hook get result from db failed", key, []byte(err.Error()))
			return nil, err
		}

		GlobalMgr.notifier("sql driver hook unmarshal query result from db", key, v)

		err = json.Unmarshal(v, &rows.rs)
		if err != nil {
			GlobalMgr.notifier("sql driver hook unmarshal failed", key, []byte(err.Error()))
			return nil, fmt.Errorf("unmarshal query result from db failed, err:%s", err.Error())
		}

		if len(rows.rs.Err) != 0 {
			if rows.rs.Err == driver.ErrSkip.Error() {
				return nil, driver.ErrSkip
			}
			return nil, fmt.Errorf("%s", rows.rs.Err)
		}

		return rows, nil
	}
}

func doExec(key string, doer func() (driver.Result, error)) (driver.Result, error) {
	if GlobalMgr.ShouldRecord() {
		r, err := doer()
		msg := ""
		last_id := int64(-1)
		row_affected := int64(0)
		if err == nil {
			last_id, _ = r.LastInsertId()
			row_affected, _ = r.RowsAffected()
		} else {
			msg = err.Error()
		}

		v := fmt.Sprintf("%d@%d@%s", last_id, row_affected, msg)

		GlobalMgr.StoreValue(key, []byte(v))
		GlobalMgr.notifier("sql driver hook exec", key, []byte(v))

		ret := &sqlHookResult{lastInsertId: int64(last_id), rowsAffected: int64(row_affected)}
		return ret, err
	} else {
		v, err := GlobalMgr.GetValue(key)
		if err != nil {
			GlobalMgr.notifier("sql driver hook exec get value from db failed", key, []byte(err.Error()))
			return nil, fmt.Errorf("doExec failed at getting value from db, err:%s", err.Error())
		}

		vs := strings.Split(string(v), "@")
		if len(vs) < 2 {
			GlobalMgr.notifier("sql driver hook exec get invalid value from db", key, []byte(""))
			return nil, fmt.Errorf("invalid recorded value from db found")
		}

		last_id, _ := strconv.Atoi(vs[0])
		rows_affected, _ := strconv.Atoi(vs[1])

		if len(vs) == 3 && len(vs[2]) > 0 {
			if vs[2] == driver.ErrSkip.Error() {
				err = driver.ErrSkip
			} else {
				err = fmt.Errorf("%s", vs[2])
			}
		}

		if err != nil {
			GlobalMgr.notifier("sql driver hook exec failed", key, []byte(err.Error()))
		} else {
			GlobalMgr.notifier("sql driver hook exec done", key, []byte(v))
		}

		r := &sqlHookResult{lastInsertId: int64(last_id), rowsAffected: int64(rows_affected)}
		return r, err
	}
}

type sqlPrecisionScale struct {
	Precision int64 `json:"precision"`
	Scale     int64 `json:"scale"`
	Ok        bool  `json:"hasScale"`
}

type sqlField struct {
	Length       int64             `json:"length"`
	HasLength    bool              `json:"hasLength"`
	Nullable     bool              `json:"nullable"`
	HasNullable  bool              `json:"hasNullable"`
	ScanType     int32             `json:"scanType"`
	DatabaseType string            `json:"databaseType"`
	Scale        sqlPrecisionScale `json:"scaleInfo"`
}

type sqlRowData struct {
	FieldInfo []sqlField
	Rows      [][]driver.Value
}

type sqlResultSet struct {
	Err         string      `json:"err"`
	CurRowIdx   int         `json:"-"`
	ColumnNames []string    `json:"columnNames"`
	RowData     *sqlRowData `json:"data"`
}

func (rd *sqlRowData) MarshalJSON() ([]byte, error) {
	d1, err1 := json.Marshal(rd.FieldInfo)
	if err1 != nil {
		return nil, fmt.Errorf("marshalling FieldInfo failed for sqlRowData")
	}

	rs := make([][]string, 0, len(rd.Rows))
	for i := range rd.Rows {
		vs := make([]string, 0, len(rd.Rows[i]))
		for j := range rd.Rows[i] {
			t, err := json.Marshal(rd.FieldInfo[j].ScanType)
			if err != nil {
				return nil, fmt.Errorf("marshaling scanType of %dth field failed, err:%s", j, err.Error())
			}

			var di interface{}
			if rd.FieldInfo[j].ScanType == scanTypeIdNullTime {
				// Time.Marshal() uses dfiiferent location layout from Time.Unmarshal()
				// this causes trouble
				di = rd.Rows[i][j]
			} else {
				di = rd.Rows[i][j]
			}

			v, err1 := json.Marshal(di)
			if err1 != nil {
				return nil, fmt.Errorf("marshaling field value at (%d,%d) failed, err:%s", i, j, err.Error())
			}

			fs, _ := json.Marshal([2][]byte{t, v})
			vs = append(vs, string(fs))
		}

		rs = append(rs, vs)
	}

	d2, _ := json.Marshal(rs)
	output := [2][]byte{d1, d2}
	return json.Marshal(&output)
}

func (rd *sqlRowData) UnmarshalJSON(data []byte) error {
	var dd [2][]byte
	err := json.Unmarshal(data, &dd)
	if err != nil {
		return fmt.Errorf("unmarshal sqlRowData failed, err:%s", err.Error())
	}

	err = json.Unmarshal(dd[0], &rd.FieldInfo)
	if err != nil {
		return fmt.Errorf("unmarshal sqlRowData.FieldInfo failed, err:%s", err.Error())
	}

	var rs [][]string
	err = json.Unmarshal(dd[1], &rs)
	if err != nil {
		return fmt.Errorf("unmarshal sqlRowData.Rows failed, err:%s", err.Error())
	}

	for i, _ := range rs {
		var row []driver.Value
		for j, _ := range rs[i] {
			var fs [2][]byte
			err = json.Unmarshal([]byte(rs[i][j]), &fs)
			if err != nil {
				return fmt.Errorf("unmarshal field(%d,%d) in sqlRowData.Rows failed, err:%s", i, j, err.Error())
			}

			var t int32
			err = json.Unmarshal(fs[0], &t)
			if err != nil {
				return fmt.Errorf("unmarshal TypeInfo at field (%d,%d) failed, err:%s", i, j, err.Error())
			}

			var v driver.Value
			if t != scanTypeIdNullTime {
				v = unmarshalByScanType(t, fs[1])
			} else {
				v = unmarshalByScanType(t, fs[1])
			}

			row = append(row, v)
		}

		rd.Rows = append(rd.Rows, row)
	}

	return nil
}

// implement driver.Rows interface representing data from db
// other interfaces implemented are listed as following, modeling mysql interface
// @@RowsColumnTypeScanType
// @@RowsColumnTypeDatabaseTypeName
// @@RowsColumnTypeLength
// @@RowsColumnTypeNullable
// @@RowsColumnTypePrecisionScale
// @@RowsNextResultSet does nothing for us, no need to implement it.
type sqlResultTable struct {
	rs        sqlResultSet
	tableName string
}

func (rows *sqlResultTable) Columns() []string {
	return rows.rs.ColumnNames
}

func (rows *sqlResultTable) Close() error {
	return nil
}

func (rows *sqlResultTable) Next(dest []driver.Value) error {
	if rows.rs.CurRowIdx >= len(rows.rs.RowData.Rows) {
		return io.EOF
	}

	copy(dest, rows.rs.RowData.Rows[rows.rs.CurRowIdx])
	rows.rs.CurRowIdx++
	return nil
}

func (rows *sqlResultTable) ColumnTypeLength(i int) (int64, bool) {
	return rows.rs.RowData.FieldInfo[i].Length, rows.rs.RowData.FieldInfo[i].HasLength
}

func (rows *sqlResultTable) ColumnTypeNullable(i int) (nullable, ok bool) {
	return rows.rs.RowData.FieldInfo[i].Nullable, rows.rs.RowData.FieldInfo[i].HasNullable
}

func (rows *sqlResultTable) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	s := &rows.rs.RowData.FieldInfo[i].Scale
	return s.Precision, s.Scale, s.Ok
}

func (rows *sqlResultTable) ColumnTypeDatabaseTypeName(i int) string {
	return rows.rs.RowData.FieldInfo[i].DatabaseType
}

func (rows *sqlResultTable) ColumnTypeScanType(i int) reflect.Type {
	return scanTypeFromId(rows.rs.RowData.FieldInfo[i].ScanType)
}

// implement driver.Result interface.
// representing result from executing db cmd
type sqlHookResult struct {
	lastInsertId int64
	rowsAffected int64
}

func (res *sqlHookResult) LastInsertId() (int64, error) {
	return res.lastInsertId, nil
}

func (res *sqlHookResult) RowsAffected() (int64, error) {
	return res.rowsAffected, nil
}

// implement driver.Tx interface.
// representing a rdms transaction.
type sqlHookTx struct {
	tx   driver.Tx
	conn *sqlHookConn
}

func (tx *sqlHookTx) Commit() (err error) {
	if GlobalMgr.ShouldRecord() {
		return tx.tx.Commit()
	}

	return nil
	//return fmt.Errorf("no tx commit is allowed in replay state")
}

func (tx *sqlHookTx) Rollback() (err error) {
	if GlobalMgr.ShouldRecord() {
		return tx.tx.Rollback()
	}

	return nil
	//return fmt.Errorf("no tx rollback is allowed in replay state")
}

// following interfaces are implemented:
// @@driver.Stmt
// @@driver.ColumnConverter.
// @@driver.StmtExecContext
// @@driver.StmtQueryContext
type sqlHookStmt struct {
	query string
	stmt  driver.Stmt
	conn  *sqlHookConn
}

func (stmt *sqlHookStmt) Close() error {
	if GlobalMgr.ShouldRecord() {
		return stmt.stmt.Close()
	}

	return nil
}

func (stmt *sqlHookStmt) NumInput() int {
	key := genSqlHookDataKey("NumInput", stmt.conn.dsn, stmt.query, "NumInput")
	if GlobalMgr.ShouldRecord() {
		ni := uint64(int64(stmt.stmt.NumInput()))
		bs := make([]byte, 8)
		binary.LittleEndian.PutUint64(bs, ni)
		GlobalMgr.StoreValue(key, []byte(bs))
		return int(ni)
	}

	bs, err := GlobalMgr.GetValue(key)
	if err != nil {
		GlobalMgr.notifier("replaying stmt.NumInput failed", key, []byte(err.Error()))
	}

	ni := binary.LittleEndian.Uint64(bs)
	return int(int64(ni))
}

func (stmt *sqlHookStmt) Exec(args []driver.Value) (driver.Result, error) {
	input := stringifySqlParam(args)
	key := genSqlHookDataKey("sqlHookStmt.Exec", stmt.conn.dsn, stmt.query, input)
	doer := func() (driver.Result, error) {
		return stmt.stmt.Exec(args)
	}

	return doExec(key, doer)
}

func (stmt *sqlHookStmt) Query(args []driver.Value) (driver.Rows, error) {
	input := stringifySqlParam(args)
	key := genSqlHookDataKey("sqlHookStmt.Query", stmt.conn.dsn, stmt.query, input)
	doer := func() (driver.Rows, error) {
		return stmt.stmt.Query(args)
	}

	return doQuery(key, doer)
}

func (stmt *sqlHookStmt) ColumnConverter(idx int) driver.ValueConverter {
	cv, ok := stmt.stmt.(driver.ColumnConverter)
	if !ok {
		return driver.DefaultParameterConverter
	}

	return cv.ColumnConverter(idx)
}

func (stmt *sqlHookStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	arr := make([]driver.Value, 0, len(args))
	values := make([]driver.Value, 0, len(args))

	for i := range args {
		arr = append(arr, args[i])
		values = append(values, args[i].Value)
	}

	input := stringifySqlParam(arr)
	key := genSqlHookDataKey("sqlHookStmt.QueryContext", stmt.conn.dsn, stmt.query, input)

	doer := func() (driver.Rows, error) {
		q, ok := stmt.stmt.(driver.StmtQueryContext)
		if ok {
			return q.QueryContext(ctx, args)
		}
		// fallback to Query()
		return stmt.stmt.Query(values)
	}

	return doQuery(key, doer)
}

func (stmt *sqlHookStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	arr := make([]driver.Value, 0, len(args))
	values := make([]driver.Value, 0, len(args))

	for i := range args {
		arr = append(arr, args[i])
		values = append(values, args[i].Value)
	}

	input := stringifySqlParam(arr)
	key := genSqlHookDataKey("sqlHookStmt.ExecContext", stmt.conn.dsn, stmt.query, input)

	doer := func() (driver.Result, error) {
		q, ok := stmt.stmt.(driver.StmtExecContext)
		if ok {
			return q.ExecContext(ctx, args)
		}
		// fallback to Exec()
		return stmt.stmt.Exec(values)
	}

	return doExec(key, doer)
}

// following interfaces are implemented:
// 	- driver.Conn/driver.Queryer/driver.SessionResetter
// 	- driver.NamedValueChecker/driver.ConnPrepareContext
// 	- driver.QueryerContext and driver.ExecerContext TODO:
type sqlHookConn struct {
	dsn      string
	origConn driver.Conn
	driver   *sqlHookDriver
}

func (conn *sqlHookConn) Ping(ctx context.Context) error {
	if !GlobalMgr.ShouldRecord() {
		return nil
	}

	p, ok := conn.origConn.(driver.Pinger)
	if !ok {
		return fmt.Errorf("underlying conn does not implement driver.Pinger interface")
	}

	return p.Ping(ctx)
}

func (conn *sqlHookConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	q, ok := conn.origConn.(driver.Queryer)
	if !ok {
		return nil, fmt.Errorf("underlying conn does not implement driver.Queryer interface")
	}

	input := stringifySqlParam(args)
	key := genSqlHookDataKey("sqlHookConn.Query", conn.dsn, query, input)
	doer := func() (driver.Rows, error) { return q.Query(query, args) }
	return doQuery(key, doer)
}

func (conn *sqlHookConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	q, ok := conn.origConn.(driver.Execer)
	if !ok {
		return nil, fmt.Errorf("underlying conn does not implement driver.Execer interface")
	}

	input := stringifySqlParam(args)
	key := genSqlHookDataKey("sqlHookConn.Exec", conn.dsn, query, input)
	doer := func() (driver.Result, error) { return q.Exec(query, args) }
	return doExec(key, doer)
}

func (conn *sqlHookConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	arr := make([]driver.Value, 0, len(args))
	values := make([]driver.Value, 0, len(args))

	for i := range args {
		arr = append(arr, args[i])
		values = append(values, args[i].Value)
	}

	input := stringifySqlParam(arr)
	key := genSqlHookDataKey("sqlHookConn.QueryContext", conn.dsn, query, input)

	doer := func() (driver.Rows, error) {
		q, ok := conn.origConn.(driver.QueryerContext)
		if ok {
			return q.QueryContext(ctx, query, args)
		}
		// fallback to Query()
		return conn.Query(query, values)
	}

	return doQuery(key, doer)
}

func (conn *sqlHookConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	arr := make([]driver.Value, 0, len(args))
	values := make([]driver.Value, 0, len(args))

	for i := range args {
		arr = append(arr, args[i])
		values = append(values, args[i].Value)
	}

	input := stringifySqlParam(arr)
	key := genSqlHookDataKey("sqlHookConn.ExecContext", conn.dsn, query, input)

	doer := func() (driver.Result, error) {
		q, ok := conn.origConn.(driver.ExecerContext)
		if ok {
			return q.ExecContext(ctx, query, args)
		}
		// fallback to Exec()
		return conn.Exec(query, values)
	}

	return doExec(key, doer)
}

func (c *sqlHookConn) Prepare(query string) (driver.Stmt, error) {
	stmt := &sqlHookStmt{query: query, conn: c}
	if GlobalMgr.ShouldRecord() {
		var err error
		stmt.stmt, err = c.origConn.Prepare(query)
		return stmt, err
	} else {
		return stmt, nil
	}
}

func (c *sqlHookConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	pc, ok := c.origConn.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}

	stmt := &sqlHookStmt{query: query, conn: c}
	if GlobalMgr.ShouldRecord() {
		var err error
		stmt.stmt, err = pc.PrepareContext(ctx, query)
		return stmt, err
	} else {
		return stmt, nil
	}
}

func (c *sqlHookConn) Close() error {
	if GlobalMgr.ShouldRecord() {
		return c.origConn.Close()
	}

	return nil
}

func (c *sqlHookConn) Begin() (driver.Tx, error) {
	tx := &sqlHookTx{conn: c}
	if GlobalMgr.ShouldRecord() {
		var err error
		tx.tx, err = c.origConn.Begin()
		return tx, err
	}

	return tx, nil
}

func (c *sqlHookConn) ResetSession(ctx context.Context) error {
	if !GlobalMgr.ShouldRecord() {
		return nil
	}

	rc, ok := c.origConn.(driver.SessionResetter)
	if !ok {
		return nil
	}

	return rc.ResetSession(ctx)
}

func (c *sqlHookConn) CheckNamedValue(nv *driver.NamedValue) (err error) {
	nvc, ok := c.origConn.(driver.NamedValueChecker)
	if !ok {
		var err error
		nv.Value, err = driver.DefaultParameterConverter.ConvertValue(nv.Value)
		return err
	}

	return nvc.CheckNamedValue(nv)
}

type sqlHookConnector struct {
	dsn    string
	driver *sqlHookDriver
}

func (c *sqlHookConnector) Driver() driver.Driver {
	return c.driver
}

func (c *sqlHookConnector) Connect(ctx context.Context) (driver.Conn, error) {
	GlobalMgr.notifier("calling connector of mysql driver hook", "", nil)

	conn := &sqlHookConn{dsn: c.dsn, driver: c.driver}
	if GlobalMgr.ShouldRecord() {
		var err error
		conn.origConn, err = c.driver.Open(c.dsn)
		return conn, err
	} else {
		return conn, nil
	}
}

// implement driver.Driver / driver.DriverContext interface
type sqlHookDriver struct {
	origDriver driver.Driver
}

func (d *sqlHookDriver) Open(dsn string) (driver.Conn, error) {
	conn := &sqlHookConn{dsn: dsn, driver: d}
	if GlobalMgr.ShouldRecord() {
		var err error
		conn.origConn, err = d.origDriver.Open(dsn)
		return conn, err
	} else {
		return conn, nil
	}
}

func (d *sqlHookDriver) OpenConnector(dsn string) (driver.Connector, error) {
	ct := &sqlHookConnector{dsn: dsn, driver: d}
	return ct, nil
}

////////////////////////////////database specific hook ///////////////////////////////////

// hook definitions.

//go:noinline
func mysqlOpenConnectorHook(d mysql.MySQLDriver, dsn string) (driver.Connector, error) {
	return &sqlHookConnector{dsn: dsn, driver: &sqlHookDriver{origDriver: d}}, nil
}

func mysqlOpenConnectorHookTrampoline(d mysql.MySQLDriver, dsn string) (driver.Connector, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if len(dsn) > 23 {
		panic("trampoline Http Post function is not allowed to be called")
	}

	return nil, fmt.Errorf("xxx")
}

// drivers are usually registered from init(), but the call order of init() from different files is undetermined.
// hence hooking Register() often comes too late. we should hook sql.Open() instead:
//  - if driver does not implement DriverContext, then hook sql.Open() will always work.
//  - the only problem is sql.OpenDB(), which should be handled by hooking corresponding OpenConnector() function.
// Note that: just hooking OpenDB() is not enough, since connector doesn't expose dsn, and dsn is a crucial key to identify a sql session.

//go:noinline
func sqlOpen(driverName, dsn string) (*sql.DB, error) {
	if driverName != "mysql" {
		return sqlOpenTrampoline(driverName, dsn)
	}

	return sqlOpenTrampoline(SqlHookMysqlDriver, dsn)
}

//go:noinline
func sqlOpenTrampoline(dn, dsn string) (*sql.DB, error) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if dn != dsn {
		panic("trampoline Http Post function is not allowed to be called")
	}

	return nil, fmt.Errorf("sss")
}

func HookMysqlDriver() error {
	registered := false
	for _, dr := range sql.Drivers() {
		if dr == SqlHookMysqlDriver {
			registered = true
			break
		}
	}

	if !registered {
		sql.Register(SqlHookMysqlDriver, &sqlHookDriver{origDriver: &mysql.MySQLDriver{}})
	}

	err := gohook.Hook(sql.Open, sqlOpen, sqlOpenTrampoline)
	if err != nil {
		return err
	}

	var d mysql.MySQLDriver
	openConnector := "OpenConnector"

	if _, ok := interface{}(d).(driver.DriverContext); !ok {
		return nil
	}

	err = gohook.HookMethod(d, openConnector, mysqlOpenConnectorHook, mysqlOpenConnectorHookTrampoline)
	if err != nil {
		gohook.UnHook(sql.Open)
		return err
	}

	return nil
}

func UnHookMysqlDriver() error {
	var dr mysql.MySQLDriver
	gohook.UnHook(sql.Open)

	if _, ok := interface{}(dr).(driver.DriverContext); !ok {
		return nil
	}

	return gohook.UnHookMethod(dr, "OpenConnector")
}

/*
func sqlRegisterHook(name string, driver driver.Driver) {
	if name == "mysql" {
		sqlRegisterHookTrampoline("mysql-origin", driver)
		sqlRegisterHookTrampoline("mysql", sqlHookDriver{driver: driver})
	}

	sqlRegisterHookTrampoline(name, driver)
}

//go:noinline
func sqlRegisterHookTrampoline(name string, driver driver.Driver) {
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")
	fmt.Printf("dummy function for regrestion testing")

	for i := 0; i < 100000; i++ {
		fmt.Printf("id:%d\n", i)
		go func() { fmt.Printf("hello world\n") }()
	}

	if name != "mysql" {
		panic("trampoline Http Post function is not allowed to be called")
	}
}
*/
