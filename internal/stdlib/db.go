package stdlib

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// packageDB is SQL databases via database/sql.
//
//	conn := db.open("sqlite:./app.db")?
//	conn := db.open("postgres://user:pass@localhost:5432/app")?
//	conn := db.open("mysql://user:pass@tcp(localhost:3306)/app")?
func packageDB(env *runtime.Env) runtime.Value {
	p := pkg()

	// db.open(dsn, opts?) -> Result[conn]
	set(p, "open", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("db.open(dsn)", "db"), nil
		}
		driver, dsn, err := parseSQLDSN(args[0].String())
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		sqlDB, err := sql.Open(driver, dsn)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		// apply opts
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)
		if len(args) >= 2 {
			if n := mapGetInt(args[1], "max_open", 0); n > 0 {
				sqlDB.SetMaxOpenConns(int(n))
			}
			if n := mapGetInt(args[1], "max_idle", 0); n > 0 {
				sqlDB.SetMaxIdleConns(int(n))
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			_ = sqlDB.Close()
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(wrapSQLConn(env, sqlDB)), nil
	}, 2)

	// db.drivers() -> list of supported scheme names
	set(p, "drivers", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.List(
			runtime.Str("sqlite"),
			runtime.Str("postgres"),
			runtime.Str("postgresql"),
			runtime.Str("mysql"),
		), nil
	}, 0)

	return p
}

func parseSQLDSN(raw string) (driver, dsn string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", fmt.Errorf("empty DSN")
	}
	// sqlite:./file.db  sqlite:///abs  file:./x.db
	switch {
	case strings.HasPrefix(s, "sqlite:"):
		path := strings.TrimPrefix(s, "sqlite:")
		path = strings.TrimPrefix(path, "//")
		if path == "" {
			path = ":memory:"
		}
		return "sqlite", path, nil
	case strings.HasPrefix(s, "file:"):
		return "sqlite", strings.TrimPrefix(s, "file:"), nil
	case s == ":memory:" || strings.HasPrefix(s, ":memory:"):
		return "sqlite", s, nil
	case strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://"):
		// pgx stdlib accepts postgres:// URLs
		return "pgx", s, nil
	case strings.HasPrefix(s, "mysql://"):
		// convert mysql://user:pass@host:port/db → user:pass@tcp(host:port)/db
		return "mysql", mysqlDSNFromURL(s), nil
	case strings.Contains(s, "@tcp(") || strings.HasPrefix(s, "user:"):
		return "mysql", s, nil
	default:
		// bare path → sqlite
		if !strings.Contains(s, "://") {
			return "sqlite", s, nil
		}
		return "", "", fmt.Errorf("unknown DSN scheme (want sqlite:, postgres://, mysql://)")
	}
}

func mysqlDSNFromURL(u string) string {
	// mysql://user:pass@host:3306/db?params → user:pass@tcp(host:3306)/db?params
	u = strings.TrimPrefix(u, "mysql://")
	// split query
	q := ""
	if i := strings.IndexByte(u, '?'); i >= 0 {
		q = u[i:]
		u = u[:i]
	}
	// user:pass@host:port/path
	at := strings.LastIndex(u, "@")
	if at < 0 {
		return u + q
	}
	userinfo := u[:at]
	rest := u[at+1:]
	slash := strings.IndexByte(rest, '/')
	hostport, dbpath := rest, ""
	if slash >= 0 {
		hostport = rest[:slash]
		dbpath = rest[slash:]
	}
	if !strings.Contains(hostport, "(") {
		hostport = "tcp(" + hostport + ")"
	}
	return userinfo + "@" + hostport + dbpath + q
}

func wrapSQLConn(env *runtime.Env, db *sql.DB) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("db."+name, arity, fn)
	}

	// conn.query(sql, params?) -> Result[[map]]
	put("query", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.query(sql, params?)", "db"), nil
		}
		q := args[0].String()
		params := sqlArgs(optArg(args, 1))
		ctx, cancel := context.WithTimeout(env.Context(), 30*time.Second)
		defer cancel()
		rows, err := db.QueryContext(ctx, q, params...)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		defer rows.Close()
		out, err := rowsToMaps(rows)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(runtime.List(out...)), nil
	})

	// conn.query_one(sql, params?) -> Result[map|null]
	put("query_one", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.query_one(sql, params?)", "db"), nil
		}
		q := args[0].String()
		params := sqlArgs(optArg(args, 1))
		ctx, cancel := context.WithTimeout(env.Context(), 30*time.Second)
		defer cancel()
		rows, err := db.QueryContext(ctx, q, params...)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		defer rows.Close()
		out, err := rowsToMaps(rows)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		if len(out) == 0 {
			return runtime.Ok(runtime.Null()), nil
		}
		return runtime.Ok(out[0]), nil
	})

	// conn.exec(sql, params?) -> Result[{rows_affected, last_id}]
	put("exec", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.exec(sql, params?)", "db"), nil
		}
		q := args[0].String()
		params := sqlArgs(optArg(args, 1))
		ctx, cancel := context.WithTimeout(env.Context(), 30*time.Second)
		defer cancel()
		res, err := db.ExecContext(ctx, q, params...)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		ra, _ := res.RowsAffected()
		li, _ := res.LastInsertId()
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"rows_affected", "last_id"}
		mo.Vals["rows_affected"] = runtime.Int(ra)
		mo.Vals["last_id"] = runtime.Int(li)
		return runtime.Ok(m), nil
	})

	// conn.close()
	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := db.Close(); err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	// conn.ping()
	put("ping", 0, func(args []runtime.Value) (runtime.Value, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(runtime.Bool(true)), nil
	})

	// conn.begin() -> Result[tx]
	put("begin", 0, func(args []runtime.Value) (runtime.Value, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(wrapSQLTx(tx)), nil
	})

	// conn.tx(fn) -> Result  runs fn(tx); commit if fn returns Ok/non-Err, else rollback
	put("tx", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.tx(fn)", "db"), nil
		}
		fn := args[0]
		if fn.Kind != runtime.KindFunc && fn.Kind != runtime.KindBuiltin {
			return errRes("conn.tx: need function", "db"), nil
		}
		if env.Call == nil {
			return errRes("runtime Call not configured", "db"), nil
		}
		ctx, cancel := context.WithTimeout(env.Context(), 60*time.Second)
		defer cancel()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		txVal := wrapSQLTx(tx)
		ret, err := env.Call(fn, []runtime.Value{txVal})
		if err != nil {
			_ = tx.Rollback()
			return errRes(err.Error(), "db"), nil
		}
		if ret.Kind == runtime.KindResult {
			ro := ret.Obj.(*runtime.ResultObj)
			if !ro.Ok {
				_ = tx.Rollback()
				return ret, nil
			}
			if err := tx.Commit(); err != nil {
				return errRes(err.Error(), "db"), nil
			}
			return runtime.Ok(ro.Val), nil
		}
		if err := tx.Commit(); err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(ret), nil
	})

	return m
}

func wrapSQLTx(tx *sql.Tx) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("db.tx."+name, arity, fn)
	}
	put("query", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tx.query(sql, params?)", "db"), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := tx.QueryContext(ctx, args[0].String(), sqlArgs(optArg(args, 1))...)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		defer rows.Close()
		out, err := rowsToMaps(rows)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(runtime.List(out...)), nil
	})
	put("query_one", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tx.query_one(sql, params?)", "db"), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := tx.QueryContext(ctx, args[0].String(), sqlArgs(optArg(args, 1))...)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		defer rows.Close()
		out, err := rowsToMaps(rows)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		if len(out) == 0 {
			return runtime.Ok(runtime.Null()), nil
		}
		return runtime.Ok(out[0]), nil
	})
	put("exec", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("tx.exec(sql, params?)", "db"), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := tx.ExecContext(ctx, args[0].String(), sqlArgs(optArg(args, 1))...)
		if err != nil {
			return errRes(err.Error(), "db"), nil
		}
		ra, _ := res.RowsAffected()
		li, _ := res.LastInsertId()
		out := runtime.NewMap()
		omo := out.Obj.(*runtime.MapObj)
		omo.Keys = []string{"rows_affected", "last_id"}
		omo.Vals["rows_affected"] = runtime.Int(ra)
		omo.Vals["last_id"] = runtime.Int(li)
		return runtime.Ok(out), nil
	})
	put("commit", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := tx.Commit(); err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("rollback", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := tx.Rollback(); err != nil {
			return errRes(err.Error(), "db"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	return m
}

func sqlArgs(v runtime.Value) []any {
	if v.Kind != runtime.KindList {
		if v.Kind == runtime.KindNull {
			return nil
		}
		return []any{valueToGo(v)}
	}
	lo := v.Obj.(*runtime.ListObj)
	out := make([]any, len(lo.Items))
	for i, it := range lo.Items {
		out[i] = valueToGo(it)
	}
	return out
}

func rowsToMaps(rows *sql.Rows) ([]runtime.Value, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []runtime.Value
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for i, c := range cols {
			mo.Keys = append(mo.Keys, c)
			mo.Vals[c] = sqlVal(raw[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func sqlVal(v any) runtime.Value {
	if v == nil {
		return runtime.Null()
	}
	switch x := v.(type) {
	case []byte:
		return bytesVal(x)
	case int64:
		return runtime.Int(x)
	case float64:
		return runtime.Float(x)
	case bool:
		return runtime.Bool(x)
	case string:
		return stringVal(x)
	case time.Time:
		return runtime.Str(x.UTC().Format(time.RFC3339))
	default:
		return runtime.Str(fmt.Sprint(x))
	}
}

// tryParseJSON attempts to parse s as a JSON object or array.
// Returns the parsed Weft value and true, or zero value and false.
func tryParseJSON(s string) (runtime.Value, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return runtime.Value{}, false
	}
	if (s[0] == '{' && s[len(s)-1] == '}') || (s[0] == '[' && s[len(s)-1] == ']') {
		var raw any
		if err := json.Unmarshal([]byte(s), &raw); err == nil {
			return goToValue(raw), true
		}
	}
	return runtime.Value{}, false
}

// bytesVal handles []byte from SQL drivers (PostgreSQL JSONB, blobs).
func bytesVal(b []byte) runtime.Value {
	if len(b) == 0 {
		return runtime.Str("")
	}
	if v, ok := tryParseJSON(string(b)); ok {
		return v
	}
	return runtime.Str(string(b))
}

// stringVal handles string from SQL drivers (SQLite TEXT, etc.).
// JSON/JSONB stored in TEXT columns is auto-parsed so scripts can
// access fields directly: row.data.name instead of json.parse(row.data)?.name
func stringVal(s string) runtime.Value {
	if v, ok := tryParseJSON(s); ok {
		return v
	}
	return runtime.Str(s)
}
