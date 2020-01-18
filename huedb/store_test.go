package huedb_test

import (
  "bytes"
  "errors"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/appcommon/db/sqlite_db"
  "github.com/keep94/goconsume"
  "github.com/keep94/gohue"
  "github.com/keep94/gosqlite/sqlite"
  "github.com/keep94/marvin/dynamic"
  "github.com/keep94/marvin/huedb"
  "github.com/keep94/marvin/huedb/for_sqlite"
  "github.com/keep94/marvin/huedb/sqlite_setup"
  "github.com/keep94/marvin/lights"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/maybe"
  "github.com/keep94/tasks"
  "log"
  "reflect"
  "strconv"
  "testing"
  "time"
)

var (
  kNilEncodedAtTimeTask = &huedb.EncodedAtTimeTask{}
  kEncodeNotSupported = errors.New("huedb: Encode not supported")
  kDecodeNotSupported = errors.New("huedb: Decode not supported")
  kDbError = errors.New("huedb: Some database error.")
)

const (
  kIdDoesNotSupportEncode = 101
  kIdDoesNotSupportDecode = 109
)

var (
  kColorMap1 = ops.LightColors{
      2: {gohue.NewMaybeColor(gohue.NewColor(0.35, 0.52)), maybe.NewUint8(99)},
      7: {gohue.NewMaybeColor(gohue.NewColor(0.51, 0.29)), maybe.NewUint8(113)},
  }
  kColorMap2 = ops.LightColors{
      3: {gohue.NewMaybeColor(gohue.NewColor(0.41, 0.43)), maybe.NewUint8(20)},
      5: {gohue.NewMaybeColor(gohue.NewColor(0.62, 0.28)), maybe.NewUint8(222)},
  }
  kFakeStore = fakeNamedColorsRunner{
      {
       Id: 2,
       Colors: kColorMap1,
       Description: "Foo",
      },
      {
       Id: 4,
       Colors: kColorMap2,
       Description: "Bar",
      },
  }
  kDescriptionMap = huedb.DescriptionMap{10004: "Baz"}
  kExpectedHueTasks = ops.HueTaskList{
      {
       Id: 10002,
       HueAction: ops.StaticHueAction(kColorMap1),
       Description: "Foo",
      },
      {
       Id: 10004,
       HueAction: ops.StaticHueAction(kColorMap2),
       Description: "Baz",
      },
  }
)

func TestHueTasks(t *testing.T) {
  tasks, err := huedb.HueTasks(huedb.FixDescriptionsRunner(
      kFakeStore, kDescriptionMap))
  if err != nil {
    t.Fatalf("Got error %v", err)
  }
  if !reflect.DeepEqual(kExpectedHueTasks, tasks) {
    t.Errorf("Exepcted %v, got %v", kExpectedHueTasks, tasks)
  }
}

func TestHueTaskById(t *testing.T) {
  task := huedb.HueTaskById(huedb.FixDescriptionByIdRunner(
      fakeNamedColorsByIdRunner{kFakeStore[1]}, kDescriptionMap), 10004)
  if !reflect.DeepEqual(kExpectedHueTasks[1], task) {
    t.Errorf("Expected %v, got %v", kExpectedHueTasks[1], task)
  }
}

func TestHueTaskById2(t *testing.T) {
  task := huedb.HueTaskById(huedb.FixDescriptionByIdRunner(
      fakeNamedColorsByIdRunner{kFakeStore[0]}, kDescriptionMap), 10002)
  if !reflect.DeepEqual(kExpectedHueTasks[0], task) {
    t.Errorf("Expected %v, got %v", kExpectedHueTasks[0], task)
  }
}

func TestHueTaskByIdError(t *testing.T) {
  task := huedb.HueTaskById(
      fakeNamedColorsByIdRunner{kFakeStore[1]}, 10003)
  verifyErrorTask(t, task, 10003)
}

func TestHueTaskByIdError2(t *testing.T) {
  task := huedb.HueTaskById(nil, 10003)
  verifyErrorTask(t, task, 10003)
}

func TestActionEncoder(t *testing.T) {
  fakeStore := fakeDynamicHueTaskStore{
      35: &dynamic.HueTask{Id: 35, Factory: fakeSpecificActionEncoder(135)},
      36: &dynamic.HueTask{Id: 36, Factory: badFactory{}},
  }
  ae := huedb.NewActionEncoder(fakeStore)
  if actual, err := ae.Encode(10007, intAction(52)); actual != "" || err != nil {
    t.Errorf("Expected empty string and no error, got %s with %v", actual, err)
  }
  if _, err := ae.Encode(37, intAction(52)); err == nil {
    t.Error("Expected an error, bad id.")
  }
  if _, err := ae.Encode(36, intAction(52)); err == nil {
    t.Error("Expected an error, bad factory.")
  }
  if actual, err := ae.Encode(35, intAction(52)); actual != "187" || err != nil {
    t.Errorf("Expected '187' and no error, got %s with %v", actual, err)
  }
}

func TestActionDecoder(t *testing.T) {
  fakeStore := fakeDynamicHueTaskStore{
      42: &dynamic.HueTask{Id: 42, Factory: fakeSpecificActionEncoder(142)},
      43: &dynamic.HueTask{Id: 43, Factory: badFactory{}},
      44: &dynamic.HueTask{Id: 44, Factory: fakeSpecificActionEncoder(kIdDoesNotSupportDecode)},
  }
  fakeDbStore := fakeNamedColorsByIdRunner{kFakeStore[0]}
  ad := huedb.NewActionDecoder(fakeStore, fakeDbStore)
  actual, err := ad.Decode(10002, "")
  expected := ops.StaticHueAction(kColorMap1)
  if err != nil || !reflect.DeepEqual(expected, actual) {
    t.Error("Got error or wrong hue action from dbStore.")
  }
  _, err = ad.Decode(10003, "")
  if err == nil {
    t.Error("Expectd error getting hue action from dbStore.")
  }
  actual, err = ad.Decode(42, "180")
  if int(actual.(intAction)) != 38 || err != nil {
    t.Errorf("Expected 38 with no error, got %v with %v", actual, err)
  }
  _, err = ad.Decode(43, "180")
  if err == nil {
    t.Error("Expectd error factory does not implement SpecificActionDecoder.")
  }
  _, err = ad.Decode(44, "180")
  if err == nil {
    t.Error("Expectd error decoding.")
  }
  _, err = ad.Decode(45, "180")
  if err == nil {
    t.Error("Expected error bad hue task id.")
  }
}

func TestAtTimeTaskStore(t *testing.T) {
  var fakeStore fakeEncodedAtTimeTaskStore
  var fakeEncoder fakeActionEncoder
  buffer := bytes.NewBuffer(nil)
  logger := log.New(buffer, "", 0)
  store := huedb.NewAtTimeTaskStore(
      fakeEncoder, fakeEncoder, &fakeStore, "default", logger)
  verifyAtTimeTaskStoreNormal(t, store)
  if len(buffer.Bytes()) > 0 {
    t.Errorf("No logs expected: %s", string(buffer.Bytes()))
  }
  // Just to be sure encoding of action works.
  if fakeStore[0].Action != "162" {
    t.Errorf("Expected encoded action 162, got %s", fakeStore[0].Action)
  }
  // AtTimeTaskStores with different group Ids should not interfere with
  // each other
  store2 := huedb.NewAtTimeTaskStore(
      fakeEncoder, fakeEncoder, &fakeStore, "second", logger)
  verifyAtTimeTaskStoreNormal(t, store2)
}

func TestAtTimeTaskStoreErrors(t *testing.T) {
  fakeStore := fakeEncodedAtTimeTaskStoreWithErrors{
      &huedb.EncodedAtTimeTask{Id: 1, Action: "35"},
      &huedb.EncodedAtTimeTask{Id: 2, Action: "36"}}
  var fakeEncoder fakeActionEncoder
  buffer := bytes.NewBuffer(nil)
  logger := log.New(buffer, "", 0)
  store := huedb.NewAtTimeTaskStore(
      fakeEncoder, fakeEncoder, fakeStore, "default", logger)
  first := &ops.AtTimeTask{
      Id: "firstId",
      H: &ops.HueTask{
          Id: 31,
          HueAction: intAction(131),
      },
  }
  store.Add(first)
  logLength := len(buffer.Bytes())
  if logLength == 0 {
    t.Error("Expected logs")
  }
  oldLength := logLength
  store.Remove("someId")
  logLength = len(buffer.Bytes())
  if logLength <= oldLength {
    t.Error("Expected logs to grow.")
  }
  oldLength = logLength
  if len(store.All()) != 0 {
    t.Error("Expected empty store.")
  }
  logLength = len(buffer.Bytes())
  if logLength <= oldLength {
    t.Error("Expected logs to grow.")
  }
}

func TestAtTimeTaskStoreEncodeErrors(t *testing.T) {
  var fakeStore fakeEncodedAtTimeTaskStore
  var fakeEncoder fakeActionEncoder
  buffer := bytes.NewBuffer(nil)
  logger := log.New(buffer, "", 0)
  store := huedb.NewAtTimeTaskStore(
      fakeEncoder, fakeEncoder, &fakeStore, "default", logger)
  first := &ops.AtTimeTask{
      Id: "firstId",
      H: &ops.HueTask{
          Id: 31,
          HueAction: intAction(131),
      },
  }
  second := &ops.AtTimeTask{
      Id: "secondId",
      H: &ops.HueTask{
          Id: 32,
          HueAction: intAction(132),
      },
  }
  third := &ops.AtTimeTask{
      Id: "thirdId",
      H: &ops.HueTask{
          Id: 33,
          HueAction: intAction(133),
      },
  }
  badDecode := &ops.AtTimeTask{
      Id: "badDecode",
      H: &ops.HueTask{
          Id: kIdDoesNotSupportDecode,
          HueAction: intAction(999),
      },
  }
  badDecode2 := &ops.AtTimeTask{
      Id: "badDecode2",
      H: &ops.HueTask{
          Id: kIdDoesNotSupportDecode,
          HueAction: intAction(998),
      },
  }
  store.Add(first)
  store.Add(second)
  store.Add(badDecode)
  store.Add(badDecode2)
  store.Add(third)

  // There should be no errors yet.
  if len(buffer.Bytes()) > 0 {
    t.Error("Expected no logs")
  }

  // Now there should be 5 entries in the store.
  if out := fakeStore.Size(); out != 5 {
    t.Errorf("Expected 5 entries in store, got %d", out)
  }

  // Only 3 of the 5 could be read
  if out := len(store.All()); out != 3 {
    t.Errorf("Expected 3 entries, got %d", out)
  }

  // All should have deleted the two entries that could not be read
  if out := fakeStore.Size(); out != 3 {
    t.Errorf("Expected 3 entries in store, got %d", out)
  }

  // There should be logs explaining the decoding errors
  logLength := len(buffer.Bytes())
  if logLength == 0 {
    t.Error("Expected logs.")
  }

  badEncode := &ops.AtTimeTask{
      Id: "badEncode",
      H: &ops.HueTask{
          Id: kIdDoesNotSupportEncode,
          HueAction: intAction(777),
      },
  }
  oldLength := logLength
  store.Add(badEncode)

  // If Encoding a task causes an error, it shouldn't be added to the
  // database. size should still be 3.
  if out := fakeStore.Size(); out != 3 {
    t.Errorf("Expected 3 entries in store, got %d", out)
  }

  // The encoding error should have been logged.
  logLength = len(buffer.Bytes())
  if logLength <= oldLength {
    t.Error("Expected logs to grow")
  }
}

func TestAttimeTaskStoreSqlite(t *testing.T) {
  var fakeEncoder fakeActionEncoder
  buffer := bytes.NewBuffer(nil)
  logger := log.New(buffer, "", 0)
  db := openDb(t)
  defer closeDb(t, db)
  dbStore := for_sqlite.New(db)
  store := huedb.NewAtTimeTaskStore(
      fakeEncoder, fakeEncoder, dbStore, "default", logger)
  verifyAtTimeTaskStoreNormal(t, store)

  // AtTimeTaskStores with different group Ids shouldn't interfere with
  // each other
  store2 := huedb.NewAtTimeTaskStore(
      fakeEncoder, fakeEncoder, dbStore, "second", logger)
  verifyAtTimeTaskStoreNormal(t, store2)

  if len(buffer.Bytes()) > 0 {
    t.Errorf("No logs expected, got: %s", string(buffer.Bytes()))
  }
  dbStore.ClearEncodedAtTimeTasks(nil)
  if len(store.All()) > 0 {
    t.Error("Expected no at time tasks.")
  }
}

func verifyErrorTask(t *testing.T, h *ops.HueTask, id int) {
  err := tasks.Run(tasks.TaskFunc(func(e *tasks.Execution) {
    h.Do(nil, nil, e)
  }))
  if err != huedb.ErrNoSuchId {
    t.Errorf("Expected huedb.ErrNoSuchId, got %v", err)
  }
  if h.Id != id {
    t.Errorf("Expected Id %d, got %d", id, h.Id)
  }
  if h.Description != "Error" {
    t.Errorf("Expected Description 'Error', got '%s'", h.Description)
  }
}

func verifyAtTimeTaskStoreNormal(t *testing.T, store *huedb.AtTimeTaskStore) {
  now := time.Unix(1300000000, 0)
  first := &ops.AtTimeTask{
      Id: "firstId",
      H: &ops.HueTask{
          Id: 31,
          HueAction: intAction(131),
          Description: "First Description",
      },
      Ls: nil,
      StartTime: now.Add(17 * time.Minute),
  }
  second := &ops.AtTimeTask{
      Id: "secondId",
      H: &ops.HueTask{
          Id: 41,
          HueAction: intAction(141),
          Description: "Second Description",
      },
      Ls: lights.New(1, 4),
      StartTime: now.Add(23 * time.Minute),
  }
  third := &ops.AtTimeTask{
      Id: "thirdId",
      H: &ops.HueTask{
          Id: 31,
          HueAction: intAction(131),
          Description: "Third Description",
      },
      Ls: lights.New(2, 5),
      StartTime: now.Add(11 * time.Minute),
  }
  if len(store.All()) > 0 {
    t.Error("Expected nothing in store.")
  }
  store.Add(first)
  store.Add(second)

  expected := []*ops.AtTimeTask{first, second}
  actual := store.All()
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
  store.Add(third)

  expected = []*ops.AtTimeTask{first, second, third}
  actual = store.All()
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
  store.Remove("secondId")

  expected = []*ops.AtTimeTask{first, third}
  actual = store.All()
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
  // A noop
  store.Remove("someBadId")

  expected = []*ops.AtTimeTask{first, third}
  actual = store.All()
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
}

type fakeNamedColorsRunner []*ops.NamedColors

func (f fakeNamedColorsRunner) NamedColors(
    t db.Transaction, consumer goconsume.Consumer) error {
  for i := range f {
    if !consumer.CanConsume() {
      break
    }
    consumer.Consume(f[i])
  }
  return nil
}

type fakeNamedColorsByIdRunner struct {
  ptr *ops.NamedColors
}

func (f fakeNamedColorsByIdRunner) NamedColorsById(
    t db.Transaction, id int64, nc *ops.NamedColors) error {
  if id != f.ptr.Id {
    return huedb.ErrNoSuchId
  }
  *nc = *f.ptr
  return nil
}

type fakeEncodedAtTimeTaskStoreWithErrors []*huedb.EncodedAtTimeTask

func (f fakeEncodedAtTimeTaskStoreWithErrors) AddEncodedAtTimeTask(
    t db.Transaction, task *huedb.EncodedAtTimeTask) error {
  return kDbError
}

func (f fakeEncodedAtTimeTaskStoreWithErrors) RemoveEncodedAtTimeTaskByScheduleId(
    t db.Transaction, groupId, scheduleId string) error {
  return kDbError
}

func (f fakeEncodedAtTimeTaskStoreWithErrors) EncodedAtTimeTasks(
    t db.Transaction, groupId string, consumer goconsume.Consumer) error {
  for i := range f {
    if !consumer.CanConsume() {
      break
    }
    consumer.Consume(f[i])
  }
  return kDbError
}

type fakeEncodedAtTimeTaskStore []*huedb.EncodedAtTimeTask

func (f *fakeEncodedAtTimeTaskStore) AddEncodedAtTimeTask(
    t db.Transaction, task *huedb.EncodedAtTimeTask) error {
  task.Id = int64(len(*f) + 1)
  stored := *task
  *f = append(*f, &stored)
  return nil
}

func (f fakeEncodedAtTimeTaskStore) RemoveEncodedAtTimeTaskByScheduleId(
    t db.Transaction, groupId, scheduleId string) error {
  for i := range f {
    if f[i].ScheduleId == scheduleId && f[i].GroupId == groupId {
      f[i] = kNilEncodedAtTimeTask
      return nil
    }
  }
  return nil
}

func (f fakeEncodedAtTimeTaskStore) Size() (result int) {
  for i := range f {
    if f[i].Id != 0 {
      result++
    }
  }
  return result
}

func (f fakeEncodedAtTimeTaskStore) EncodedAtTimeTasks(
    t db.Transaction, groupId string, consumer goconsume.Consumer) error {
  for i := range f {
    if !consumer.CanConsume() {
      break
    }
    if f[i].Id == 0 || f[i].GroupId != groupId {
      continue
    }
    consumer.Consume(f[i])
  }
  return nil
}

type fakeActionEncoder struct {
}

func (f fakeActionEncoder) Encode(
    id int, action ops.HueAction) (string, error) {
  if id == kIdDoesNotSupportEncode {
    return "", kEncodeNotSupported
  }
  return strconv.Itoa(int(action.(intAction)) + id), nil
}

func (f fakeActionEncoder) Decode(
    id int, encoded string) (action ops.HueAction, err error) {
  if id == kIdDoesNotSupportDecode {
    err = kDecodeNotSupported
    return
  }
  var aid int
  if aid, err = strconv.Atoi(encoded); err != nil {
    return
  }
  action = intAction(aid - id)
  return
}

type intAction int

func (i intAction) Do(
    ctx ops.Context, lightSet lights.Set, e *tasks.Execution) {
}

func (i intAction) UsedLights(lightSet lights.Set) lights.Set {
  return lightSet
}

func closeDb(t *testing.T, db *sqlite_db.Db) {
  if err := db.Close(); err != nil {
    t.Errorf("Error closing database: %v", err)
  }
}

type fakeDynamicHueTaskStore map[int]*dynamic.HueTask

func (f fakeDynamicHueTaskStore) ById(id int) *dynamic.HueTask {
  return f[id]
}

type fakeSpecificActionEncoder int

func (f fakeSpecificActionEncoder) Encode(a ops.HueAction) string {
  return strconv.Itoa(int(a.(intAction)) + int(f))
}

func (f fakeSpecificActionEncoder) Decode(s string) (ops.HueAction, error) {
  var decoder fakeActionEncoder
  return decoder.Decode(int(f), s)
}

func (f fakeSpecificActionEncoder) Params() dynamic.NamedParamList {
  return nil
}

func (f fakeSpecificActionEncoder) New(values []interface{}) ops.HueAction {
  return nil
}

type badFactory struct {
}

func (b badFactory) Params() dynamic.NamedParamList {
  return nil
}

func (b badFactory) New(values []interface{}) ops.HueAction {
  return nil
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
