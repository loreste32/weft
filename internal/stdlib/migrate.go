package stdlib

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageMigrate: database schema migrations.
//
//	conn := db.open("sqlite:./app.db")?
//	migrate.run(conn, "migrations/")?        // apply pending migrations
//	migrate.status(conn, "migrations/")?     // show applied/pending
//	migrate.create("migrations/", "add_users") // create new migration file
func packageMigrate(env *runtime.Env) runtime.Value {
	p := pkg()

	// migrate.run(conn, dir) -> Result[int]  apply pending migrations, return count
	set(p, "run", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("migrate.run(conn, dir)", "migrate"), nil
		}
		conn := args[0]
		dir := args[1].String()
		n, err := runMigrations(conn, dir, env)
		if err != nil {
			return errRes(err.Error(), "migrate"), nil
		}
		return runtime.Ok(runtime.Int(int64(n))), nil
	}, 2)

	// migrate.status(conn, dir) -> Result[list]
	set(p, "status", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("migrate.status(conn, dir)", "migrate"), nil
		}
		conn := args[0]
		dir := args[1].String()
		status, err := migrationStatus(conn, dir, env)
		if err != nil {
			return errRes(err.Error(), "migrate"), nil
		}
		return runtime.Ok(status), nil
	}, 2)

	// migrate.create(dir, name) -> Result[str]  create migration file
	set(p, "create", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("migrate.create(dir, name)", "migrate"), nil
		}
		dir := args[0].String()
		name := args[1].String()
		path, err := createMigration(dir, name)
		if err != nil {
			return errRes(err.Error(), "migrate"), nil
		}
		return runtime.Ok(runtime.Str(path)), nil
	}, 2)

	return p
}

func ensureMigrationTable(conn runtime.Value, env *runtime.Env) error {
	execFn, ok := conn.Obj.(*runtime.MapObj).Vals["exec"]
	if !ok {
		return fmt.Errorf("conn has no exec method")
	}
	if env.Call == nil {
		return fmt.Errorf("runtime Call not configured")
	}
	_, err := env.Call(execFn, []runtime.Value{
		runtime.Str("CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY, applied_at TEXT)"),
	})
	return err
}

func getAppliedMigrations(conn runtime.Value, env *runtime.Env) (map[string]bool, error) {
	queryFn, ok := conn.Obj.(*runtime.MapObj).Vals["query"]
	if !ok {
		return nil, fmt.Errorf("conn has no query method")
	}
	if env.Call == nil {
		return nil, fmt.Errorf("runtime Call not configured")
	}
	result, err := env.Call(queryFn, []runtime.Value{
		runtime.Str("SELECT name FROM _migrations ORDER BY name"),
	})
	if err != nil {
		return nil, err
	}
	applied := map[string]bool{}
	// result is Result[list]
	if result.Kind == runtime.KindResult {
		ro := result.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return nil, fmt.Errorf("query failed")
		}
		if ro.Val.Kind == runtime.KindList {
			for _, row := range ro.Val.Obj.(*runtime.ListObj).Items {
				if row.Kind == runtime.KindMap {
					if n, ok := row.Obj.(*runtime.MapObj).Vals["name"]; ok {
						applied[n.String()] = true
					}
				}
			}
		}
	}
	return applied, nil
}

func runMigrations(conn runtime.Value, dir string, env *runtime.Env) (int, error) {
	if err := ensureMigrationTable(conn, env); err != nil {
		return 0, err
	}
	applied, err := getAppliedMigrations(conn, env)
	if err != nil {
		return 0, err
	}
	files, err := listMigrationFiles(dir)
	if err != nil {
		return 0, err
	}

	execFn := conn.Obj.(*runtime.MapObj).Vals["exec"]
	count := 0
	for _, f := range files {
		name := filepath.Base(f)
		if applied[name] {
			continue
		}
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			return count, err
		}
		// Execute each statement
		for _, stmt := range splitSQL(string(sqlBytes)) {
			if stmt == "" {
				continue
			}
			_, err := env.Call(execFn, []runtime.Value{runtime.Str(stmt)})
			if err != nil {
				return count, fmt.Errorf("migration %s: %w", name, err)
			}
		}
		// Record
		_, err = env.Call(execFn, []runtime.Value{
			runtime.Str("INSERT INTO _migrations (name, applied_at) VALUES (?, ?)"),
			runtime.List(runtime.Str(name), runtime.Str(time.Now().UTC().Format(time.RFC3339))),
		})
		if err != nil {
			return count, err
		}
		count++
		fmt.Printf("  applied: %s\n", name)
	}
	return count, nil
}

func migrationStatus(conn runtime.Value, dir string, env *runtime.Env) (runtime.Value, error) {
	if err := ensureMigrationTable(conn, env); err != nil {
		return runtime.Null(), err
	}
	applied, err := getAppliedMigrations(conn, env)
	if err != nil {
		return runtime.Null(), err
	}
	files, err := listMigrationFiles(dir)
	if err != nil {
		return runtime.Null(), err
	}

	var items []runtime.Value
	for _, f := range files {
		name := filepath.Base(f)
		status := "pending"
		if applied[name] {
			status = "applied"
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"name", "status"}
		mo.Vals["name"] = runtime.Str(name)
		mo.Vals["status"] = runtime.Str(status)
		items = append(items, m)
	}
	return runtime.List(items...), nil
}

func createMigration(dir, name string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", ts, name)
	path := filepath.Join(dir, filename)
	content := fmt.Sprintf("-- Migration: %s\n-- Created: %s\n\n", name, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func listMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func splitSQL(s string) []string {
	var stmts []string
	for _, stmt := range strings.Split(s, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

// unused import guards
var _ = sql.ErrNoRows
var _ = context.Background
