// Package sqlite_setup sets up a sqlite database for Hue Web App
package sqlite_setup

import (
  "code.google.com/p/gosqlite/sqlite"
)

// SetUpTables creates all needed tables in database.
func SetUpTables(conn *sqlite.Conn) error {
  err := conn.Exec("create table if not exists named_colors (id INTEGER PRIMARY KEY AUTOINCREMENT, description TEXT, colors TEXT)")
  if err != nil {
    return err
  }
  return nil
}
