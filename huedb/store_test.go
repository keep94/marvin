package huedb_test

import (
  "github.com/keep94/appcommon/db"
  "github.com/keep94/gofunctional3/functional"
  "github.com/keep94/gohue"
  "github.com/keep94/marvin/huedb"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/maybe"
  "github.com/keep94/tasks"
  "reflect"
  "testing"
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

type fakeNamedColorsRunner []*ops.NamedColors

func (f fakeNamedColorsRunner) NamedColors(
    t db.Transaction, consumer functional.Consumer) error {
  return consumer.Consume(functional.NewStreamFromPtrs(f, nil))
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
