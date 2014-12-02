// Package for_sqlite provides a sqlite implementation of interfaces in
// huedb package.
package for_sqlite

import (
  "code.google.com/p/gosqlite/sqlite"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/appcommon/db/sqlite_db"
  "github.com/keep94/gofunctional3/functional"
  "github.com/keep94/gohue"
  "github.com/keep94/marvin/huedb"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/maybe"
  "strconv"
  "strings"
)

const (
  kSQLNamedColorsById = "select id, colors, description from named_colors where id = ?"
  kSQLNamedColors = "select id, colors, description from named_colors order by 1"
  kSQLAddNamedColors = "insert into named_colors (colors, description) values (?, ?)"
  kSQLUpdateNamedColors = "update named_colors set colors = ?, description = ? where id = ?"
  kSQLRemoveNamedColors = "delete from named_colors where id = ?"
)

type Store struct {
  db sqlite_db.Doer
}

func New(db *sqlite_db.Db) Store {
  return Store{db}
}

func ConnNew(conn *sqlite.Conn) Store {
  return Store{sqlite_db.NewSqliteDoer(conn)}
}

func (s Store) NamedColorsById(
    t db.Transaction, id int64, namedColors *ops.NamedColors) error {
  return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
    return sqlite_db.ReadSingle(
        conn,
        &rawNamedColors{},
        huedb.ErrNoSuchId,
        namedColors,
        kSQLNamedColorsById,
        id)
  })
}

func (s Store) NamedColors(
    t db.Transaction, consumer functional.Consumer) error {
  return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
    return sqlite_db.ReadMultiple(
        conn,
        &rawNamedColors{},
        consumer,
        kSQLNamedColors)
  })
}

func (s Store) AddNamedColors(
    t db.Transaction, namedColors *ops.NamedColors) error {
  return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
    return sqlite_db.AddRow(
        conn,
        &rawNamedColors{},
        namedColors,
        &namedColors.Id,
        kSQLAddNamedColors)
  })
}

func (s Store) UpdateNamedColors(
    t db.Transaction, namedColors *ops.NamedColors) error {
  return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
    return sqlite_db.UpdateRow(
        conn,
        &rawNamedColors{},
        namedColors,
        kSQLUpdateNamedColors)
  })
}

func (s Store) RemoveNamedColors(t db.Transaction, id int64) error {
  return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
    return conn.Exec(kSQLRemoveNamedColors, id)
  })
}

type rawNamedColors struct {
  *ops.NamedColors
  colors string
}

func (r *rawNamedColors) Ptrs() []interface{} {
  return []interface{}{&r.Id, &r.colors, &r.Description}
}

func (r *rawNamedColors) Values() []interface{} {
  return []interface{}{r.colors, r.Description, r.Id}
}

func (r *rawNamedColors) Pair(ptr interface{}) {
  r.NamedColors = ptr.(*ops.NamedColors)
}

func (r *rawNamedColors) Unmarshall() error {
  if !strings.HasPrefix(r.colors, "0|") && r.colors != "0" {
    return huedb.ErrBadLightColors
  }
  marshalled := strings.Split(r.colors, "|")
  marshalledLen := len(marshalled)
  lightColors := make(ops.LightColors, (marshalledLen - 1) / 4)
  for idx := 1; idx < marshalledLen; idx += 4 {
    lightId, err := strconv.Atoi(marshalled[idx])
    if err != nil {
      return err
    }
    var ix int
    if ix, err = strconv.Atoi(marshalled[idx + 1]); err != nil {
      return err
    }
    var iy int
    if iy, err = strconv.Atoi(marshalled[idx + 2]); err != nil {
      return err
    }
    var ibrightness int
    if ibrightness, err = strconv.Atoi(marshalled[idx + 3]); err != nil {
      return err
    }
    if lightId < 0 {
      return huedb.ErrBadLightColors
    }
    var theColor gohue.MaybeColor
    if ix != -1 {
      x := float64(ix) / 10000.0
      y := float64(iy) / 10000.0
      if x < 0.0 || x > 1.0 || y < 0.0 || y > 1.0 {
        return huedb.ErrBadLightColors
      }
      theColor.Set(gohue.NewColor(x, y))
    }
    var theBrightness maybe.Uint8
    if ibrightness != -1 {
      if ibrightness < 0 || ibrightness > 255 {
        return huedb.ErrBadLightColors
      }
      theBrightness.Set(uint8(ibrightness))
    }
    lightColors[lightId] = ops.ColorBrightness{theColor, theBrightness}
  }
  if len(lightColors) == 0 {
    r.Colors = nil
  } else {
    r.Colors = lightColors
  }
  return nil
}

func (r *rawNamedColors) Marshall() error {
  marshalled := make([]string, 4 * len(r.Colors) + 1)
  marshalled[0] = "0"
  var idx = 1
  for lightId, colorBrightness := range r.Colors {
    if lightId < 0 {
      return huedb.ErrBadLightColors
    }
    var ix, iy int
    if colorBrightness.Color.Valid {
      x := colorBrightness.Color.X()
      y := colorBrightness.Color.Y()
      if x < 0.0 || x > 1.0 || y < 0.0 || y > 1.0 {
        return huedb.ErrBadLightColors
      }
      ix = int(x * 10000.0 + 0.5)
      iy = int(y * 10000.0 + 0.5)
    } else {
      ix = -1
      iy = 0
    }
    var iBrightness int
    if colorBrightness.Brightness.Valid {
      iBrightness = int(colorBrightness.Brightness.Value)
    } else {
      iBrightness = -1
    }
    marshalled[idx] = strconv.Itoa(lightId)
    idx++
    marshalled[idx] = strconv.Itoa(ix)
    idx++
    marshalled[idx] = strconv.Itoa(iy)
    idx++
    marshalled[idx] = strconv.Itoa(iBrightness)
    idx++
  }
  r.colors = strings.Join(marshalled, "|")
  return nil
}

