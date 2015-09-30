package for_sqlite_test

import (
  "github.com/keep94/appcommon/db/sqlite_db"
  "github.com/keep94/gosqlite/sqlite"
  "github.com/keep94/marvin/huedb/fixture"
  "github.com/keep94/marvin/huedb/for_sqlite"
  "github.com/keep94/marvin/huedb/sqlite_setup"
  "testing"
)

func TestNamedColorsById(t *testing.T) {
  db := openDb(t)
  defer closeDb(t, db)
  fixture.NamedColorsById(t, for_sqlite.New(db))
}

func TestNamedColors(t *testing.T) {
  db := openDb(t)
  defer closeDb(t, db)
  fixture.NamedColors(t, for_sqlite.New(db))
}

func TestUpdateNamedColors(t *testing.T) {
  db := openDb(t)
  defer closeDb(t, db)
  fixture.UpdateNamedColors(t, for_sqlite.New(db))
}

func TestRemoveNamedColors(t *testing.T) {
  db := openDb(t)
  defer closeDb(t, db)
  fixture.RemoveNamedColors(t, for_sqlite.New(db))
}

func closeDb(t *testing.T, db *sqlite_db.Db) {
  if err := db.Close(); err != nil {
    t.Errorf("Error closing database: %v", err)
  }
}

func openDb(t *testing.T) *sqlite_db.Db {
  conn, err := sqlite.Open(":memory:")
  if err != nil {
    t.Fatalf("Error opening database: %v", err)
  }
  db := sqlite_db.New(conn)
  err = db.Do(func(conn *sqlite.Conn) error {
    return sqlite_setup.SetUpTables(conn)
  })
  if err != nil {
    t.Fatalf("Error creating tables: %v", err)
  }
  return db
}

