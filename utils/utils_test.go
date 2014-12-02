package utils_test

import (
  "github.com/keep94/marvin/lights"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/marvin/utils"
  "github.com/keep94/tasks"
  "testing"
  "time"
)

func TestTaskCollection(t *testing.T) {
  doNothing := tasks.TaskFunc(func(e *tasks.Execution) {})
  // Make some Execution instances
  e1 := tasks.Start(doNothing)
  e2 := tasks.Start(doNothing)
  e3 := tasks.Start(doNothing)
  e4 := tasks.Start(doNothing)

  htw1 := &utils.HueTaskWrapper{
      H: &ops.HueTask{Id: 17}, Ls: lights.New(1, 3)}
  htw2 := &utils.HueTaskWrapper{
      H: &ops.HueTask{Id: 25}, Ls: lights.New(2)}
  htw3 := &utils.HueTaskWrapper{
      H: &ops.HueTask{Id: 31}, Ls: lights.New(3, 4)}
  htw4 := &utils.HueTaskWrapper{
      H: &ops.HueTask{Id: 49}, Ls: lights.New(5, 6)}
  htwAll := &utils.HueTaskWrapper{H: &ops.HueTask{Id: 50}}

  coll := &utils.TaskCollection{}

  // Test adding
  coll.Add(htw1, e1)
  coll.Add(htw2, e2)

  // Test FindByTaskId
  verifyExecution(t, e2, coll.FindByTaskId("25:2"))
  verifyExecution(t, e1, coll.FindByTaskId("17:1,3"))
  verifyExecution(t, nil, coll.FindByTaskId("18:5"))

  // Test conflicts
  verifyConflicts(t, coll.Conflicts(nil), e1, e2)
  verifyConflicts(t, coll.Conflicts(htw4))
  verifyConflicts(t, coll.Conflicts(htwAll), e1, e2)
  verifyConflicts(t, coll.Conflicts(htw3), e1)

  // Test Remove
  coll.Add(htw4, e4)
  coll.Remove(htw1)
  coll.Add(htw3, e3)
  verifyTasks(t, coll, htw2, htw4, htw3)
  verifyConflicts(t, coll.Conflicts(nil), e2, e4, e3)
  coll.Remove(htw3)
  verifyTasks(t, coll, htw2, htw4)
  verifyConflicts(t, coll.Conflicts(nil), e2, e4)
  coll.Remove(htw2)
  coll.Remove(htw2)
  coll.Remove(htw4)
  coll.Remove(htw4)
  verifyTasks(t, coll)
  verifyConflicts(t, coll.Conflicts(nil))

  // Test All lights
  coll.Add(htwAll, e1)
  verifyConflicts(t, coll.Conflicts(htw4), e1)
  verifyExecution(t, e1, coll.FindByTaskId("50:All"))
}

func TestTimerTaskWrapper(t *testing.T) {
  now := time.Unix(1300000000, 0)
  task := &utils.TimerTaskWrapper{
      H: &ops.HueTask{Id: 21},
      Ls: lights.New(5, 7),
      StartTime: now.Add(time.Hour + 5 * time.Minute + 53 * time.Second)}
  assertStrEqual(t, "21:1300003953:5,7", task.TaskId())

  // One second added to display clock
  assertStrEqual(t, "1:05:54", task.TimeLeftStr(now))
  assertStrEqual(
      t,
      "1:00:00",
       task.TimeLeftStr(now.Add(5 * time.Minute + 54 * time.Second)))
  assertStrEqual(
      t,
      "59:59",
       task.TimeLeftStr(now.Add(5 *time.Minute + 55 * time.Second)))
  assertStrEqual(t, "5:54", task.TimeLeftStr(now.Add(time.Hour)))
  assertStrEqual(
      t,
      "1:00",
       task.TimeLeftStr(now.Add(time.Hour + 4 * time.Minute + 54 * time.Second)))
  assertStrEqual(
      t,
      "0:59",
       task.TimeLeftStr(now.Add(time.Hour + 4 * time.Minute + 55 * time.Second)))
  assertStrEqual(
      t,
      "0:01",
       task.TimeLeftStr(now.Add(time.Hour + 5 * time.Minute + 53 * time.Second)))
  assertStrEqual(
      t,
      "0:00",
       task.TimeLeftStr(now.Add(time.Hour + 5 * time.Minute + 54 * time.Second)))
  assertStrEqual(
      t,
      "0:00",
       task.TimeLeftStr(now.Add(time.Hour + 5 * time.Minute + 55 * time.Second)))
}

func TestStartNoLights(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.Start(newHueTaskFalse(5), lights.All)
  verifyHueTaskIds(t, te.Tasks())
}

func TestMaybeStartNoLights(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.MaybeStart(newHueTaskFalse(5), lights.All)
  verifyHueTaskIds(t, te.Tasks())
}

func TestMaybeStart(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.MaybeStart(newHueTask(5), lights.All)
  te.MaybeStart(newHueTask(6), lights.All)
  te.MaybeStart(newHueTask(7), lights.New(1, 2))
  verifyHueTaskIds(t, te.Tasks(), 5)
}

func TestMaybeStart2(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.MaybeStart(newHueTask(5), lights.New(1, 2))
  te.MaybeStart(newHueTask(6), lights.New(2, 3))
  te.MaybeStart(newHueTask(7), lights.New(1, 3))
  te.MaybeStart(newHueTask(8), lights.All)
  verifyHueTaskIds(t, te.Tasks(), 5, 6)
  verifyHueTaskLights(t, te.Tasks(), "1,2", "3")
}

func TestMaybeStartUsedLights(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.MaybeStart(newHueTask(5), lights.New(1, 2))
  te.MaybeStart(newHueTask10(6), lights.New(2, 3))
  te.MaybeStart(newHueTask10(7), lights.New(4))
  verifyHueTaskIds(t, te.Tasks(), 5, 6)
  verifyHueTaskLights(t, te.Tasks(), "1,2", "3,10")
}

func TestMaybeStartUsedLights2(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.MaybeStart(newHueTaskAll(5), lights.New(1, 2))
  te.MaybeStart(newHueTask(6), lights.New(2, 3))
  verifyHueTaskIds(t, te.Tasks(), 5)
  verifyHueTaskLights(t, te.Tasks(), "All")
}

func TestMaybeStartUsedLights3(t *testing.T) {
  te := utils.NewMultiExecutor(nil, nil)
  defer te.Close()
  te.MaybeStart(newHueTask(5), lights.New(1, 2))
  te.MaybeStart(newHueTaskAll(6), lights.New(3))
  verifyHueTaskIds(t, te.Tasks(), 5)
  verifyHueTaskLights(t, te.Tasks(), "1,2")
}

func TestFutureTime(t *testing.T) {
  now := time.Date(2014, 11, 7, 16, 43, 0, 0, time.Local)
  future1644 := utils.FutureTime(now, 16, 44)
  future1643 := utils.FutureTime(now, 16, 43)
  future1700 := utils.FutureTime(now, 17, 0)
  if out := future1644.Sub(now); out != time.Minute {
    t.Errorf("Expected 1 minute, got %v", out)
  }
  if out := future1700.Sub(now); out != 17 * time.Minute {
    t.Errorf("Expected 17 minutes, got %v", out)
  }
  if out := future1643.Sub(now); out != 24 * time.Hour {
    t.Errorf("Expected 24 hours, got %v", out)
  }
}

func assertStrEqual(t *testing.T, expected, actual string) {
  if expected != actual {
    t.Errorf("Expected %s, got %s", expected, actual)
  }
}

func verifyExecution(t *testing.T, expected *tasks.Execution, actual *tasks.Execution) {
  if expected != actual {
    t.Error("Returned execution is wrong.")
  }
}

func verifyTasks(t *testing.T, coll *utils.TaskCollection, expected... *utils.HueTaskWrapper) {
  var actual []*utils.HueTaskWrapper
  coll.Tasks(&actual)
  if len(actual) != len(expected) {
    t.Errorf("Expected length %d, got %d", len(expected), len(actual))
    return
  }
  for i := range expected {
    if actual[i] != expected[i] {
      t.Error("Tasks don't match.")
    }
  }
}

func verifyConflicts(t *testing.T, actual []*tasks.Execution, expected... *tasks.Execution) {
  if len(actual) != len(expected) {
    t.Errorf("Expected length %d, got %d", len(expected), len(actual))
    return
  }
  for i := range expected {
    if actual[i] != expected[i] {
      t.Error("Executions don't match.")
    }
  }
}

func verifyHueTaskIds(
    t *testing.T, tasks []*utils.HueTaskWrapper, expected... int) {
  if len(tasks) != len(expected) {
    t.Errorf("Expected length %d, got %d", len(expected), len(tasks))
    return
  }
  for i := range expected {
    if tasks[i].H.Id != expected[i] {
      t.Error("verifyHueTaskIds: Ids don't match")
    }
  }
}

func verifyHueTaskLights(
    t *testing.T, tasks []*utils.HueTaskWrapper, expected... string) {
  if len(tasks) != len(expected) {
    t.Errorf("Expected length %d, got %d", len(expected), len(tasks))
    return
  }
  for i := range expected {
    if tasks[i].Ls.String() != expected[i] {
      t.Error("verifyHueTaskLights: lights don't match")
    }
  }
}

func newHueTask(id int) *ops.HueTask {
  return newHueTaskWithAction(id, longHueAction{})
}

func newHueTask10(id int) *ops.HueTask {
  return newHueTaskWithAction(id, longHueAction10{})
}

func newHueTaskAll(id int) *ops.HueTask {
  return newHueTaskWithAction(id, longHueActionAll{})
}

func newHueTaskFalse(id int) *ops.HueTask {
  return newHueTaskWithAction(id, longHueActionFalse{})
}

func newHueTaskWithAction(id int, a ops.HueAction) *ops.HueTask {
  return &ops.HueTask{Id: id, HueAction: a}
}

type longAction struct {
}

func (l longAction) Do(
    c ops.Context, lightSet lights.Set, e *tasks.Execution) {
  e.Sleep(time.Hour)
}

type longHueAction struct {
  longAction
}

func (l longHueAction) UsedLights(
    lightSet lights.Set) lights.Set {
  return lightSet
}

type longHueAction10 struct {
  longAction
}

func (l longHueAction10) UsedLights(
    lightSet lights.Set) lights.Set {
  return lightSet.Add(lights.New(10))
}

type longHueActionAll struct {
  longAction
}

func (l longHueActionAll) UsedLights(
    lightSet lights.Set) lights.Set {
  return lights.All
}

type longHueActionFalse struct {
  longAction
}

func (l longHueActionFalse) UsedLights(
    lightSet lights.Set) lights.Set {
  return lights.None
}

