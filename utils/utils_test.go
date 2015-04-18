package utils_test

import (
  "github.com/keep94/marvin/lights"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/marvin/utils"
  "github.com/keep94/tasks"
  "reflect"
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
  conflictingTask := &utils.TimerTaskWrapper{
      H: &ops.HueTask{Id: 23},
      Ls: lights.New(4, 7),
      StartTime: now.Add(time.Hour + 5 * time.Minute + 53 * time.Second)}
  notConflictingTask := &utils.TimerTaskWrapper{
      H: &ops.HueTask{Id: 23},
      Ls: lights.New(4),
      StartTime: now.Add(time.Hour + 5 * time.Minute + 53 * time.Second)}
  notConflictingTask2 := &utils.TimerTaskWrapper{
      H: &ops.HueTask{Id: 23},
      Ls: lights.New(4, 7),
      StartTime: now.Add(time.Hour + 5 * time.Minute + 54 * time.Second)}
  assertStrEqual(t, "21:1300003953:5,7", task.TaskId())
  if !task.ConflictsWith(conflictingTask) {
    t.Error("Expected tasks to conflict.")
  }
  if task.ConflictsWith(notConflictingTask) {
    t.Error("Expected tasks not to conflict.")
  }
  if task.ConflictsWith(notConflictingTask2) {
    t.Error("Expected tasks not to conflict.")
  }

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

func TestMultiTimerPersistence(t *testing.T) {
  now := time.Unix(1400000000, 0)
  storedAtTimeTasks := []*ops.AtTimeTask{
      {H: &ops.HueTask{Id: 21, HueAction: intAction(121), Description: "Foo" },
       Ls: nil,
       StartTime: now.Add(10 * time.Minute),
      },
      {H: &ops.HueTask{Id: 22, HueAction: intAction(122), Description: "Baz" },
       Ls: nil,
       StartTime: now.Add(-1 * time.Second),
      },
      {H: &ops.HueTask{Id: 25, HueAction: intAction(125), Description: "Bar"  },
       Ls: lights.New(2, 4),
       StartTime: now.Add( 15 * time.Minute),
      },
  }
  storeActivity := make(chan interface{}, 10)
  beginnerActivity := make(chan interface{}, 10)
  defer close(storeActivity)
  defer close(beginnerActivity)
  clock := tasks.NewFakeClock(now)
  store := &atTimeTaskStore{
          Tasks: storedAtTimeTasks, Activity: storeActivity}
  beginner := hueTaskBeginner{beginnerActivity}
  mt := utils.NewMultiTimerWithStoreAndClock(beginner, store, clock)
  task22ScheduleId := "22:1399999999:All"
  scheduleOfTaskId22 := mt.FindByScheduleId(task22ScheduleId)
  store.VerifyRemoved(t, task22ScheduleId, true)
  if scheduleOfTaskId22 != nil {
      <-scheduleOfTaskId22.Done()
  }
  expectedAtTimeTasks := []*ops.AtTimeTask{
      {H: &ops.HueTask{Id: 21, HueAction: intAction(121), Description: "Foo" },
       Ls: nil,
       StartTime: now.Add(10 * time.Minute),
      },
      {H: &ops.HueTask{Id: 25, HueAction: intAction(125), Description: "Bar"  },
       Ls: lights.New(2, 4),
       StartTime: now.Add( 15 * time.Minute),
      },
  }
  verifyScheduled(t, expectedAtTimeTasks, mt.Scheduled())

  // Schedule another task that conflicts with an existing task
  mt.Schedule(
      &ops.HueTask{Id: 27, HueAction: intAction(127), Description: "Baz" },
      lights.New(1, 4),
      now.Add(10 * time.Minute))
  store.VerifyRemoved(t, "21:1400000600:All", false)
  store.VerifyAdded(t, &ops.AtTimeTask{
      Id: "27:1400000600:1,4",
      H: &ops.HueTask{Id: 27, HueAction: intAction(127), Description: "Baz" },
      Ls: lights.New(1, 4),
      StartTime: now.Add(10 * time.Minute)}, false)
  scheduleOfTaskId27 := mt.FindByScheduleId("27:1400000600:1,4")
  beginner.VerifyNoInteraction(t)
  expectedAtTimeTasks = []*ops.AtTimeTask{
      {H: &ops.HueTask{Id: 25, HueAction: intAction(125), Description: "Bar"  },
       Ls: lights.New(2, 4),
       StartTime: now.Add( 15 * time.Minute),
      },
      {H: &ops.HueTask{Id: 27, HueAction: intAction(127), Description: "Baz"  },
       Ls: lights.New(1, 4),
       StartTime: now.Add( 10 * time.Minute),
      },
  }
  verifyScheduled(t, expectedAtTimeTasks, mt.Scheduled())
  clock.Advance(10 * time.Minute)
  beginner.Verify(
      t,
      &ops.HueTask{Id: 27, HueAction: intAction(127), Description: "Baz"},
      lights.New(1, 4))
  store.VerifyRemoved(t, "27:1400000600:1,4", true)

  // Block until scheduling is complete before verifying task list
  <-scheduleOfTaskId27.Done()
  expectedAtTimeTasks = []*ops.AtTimeTask{
      {H: &ops.HueTask{Id: 25, HueAction: intAction(125), Description: "Bar"  },
       Ls: lights.New(2, 4),
       StartTime: now.Add( 15 * time.Minute),
      },
  }
  verifyScheduled(t, expectedAtTimeTasks, mt.Scheduled())

  // This should be a noop
  mt.Cancel("NoSuchTaskId")

  store.VerifyNoInteraction(t)
  beginner.VerifyNoInteraction(t)
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

type intAction int

func (i intAction) Do(
    c ops.Context, lightSet lights.Set, e *tasks.Execution) {
}

func (l intAction) UsedLights(
    lightSet lights.Set) lights.Set {
  return lightSet
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

type hueTaskBeginner struct {
  Activity chan interface{}
}

func (b hueTaskBeginner) Begin(h *ops.HueTask, ls lights.Set) {
  b.Activity <- h
  b.Activity <- ls
}

func (b hueTaskBeginner) Verify(
    t *testing.T, expectedH *ops.HueTask, expectedLs lights.Set) {
  h := nextActivity(b.Activity, true).(*ops.HueTask)
  ls := nextActivity(b.Activity, true).(lights.Set)
  if !reflect.DeepEqual(expectedH, h) {
    t.Errorf("Expected task %v, got %v", expectedH, h)
  }
  if !reflect.DeepEqual(expectedLs, ls) {
    t.Errorf("Expected light set %v, got %v", expectedLs, ls)
  }
}

func (b hueTaskBeginner) VerifyNoInteraction(t *testing.T) {
  h := nextActivity(b.Activity, false)
  ls := nextActivity(b.Activity, false)
  if h != nil || ls != nil {
    t.Error("Expected no interaction.")
  }
}

type atTimeTaskStore struct {
  Tasks []*ops.AtTimeTask
  Activity chan interface{}
}

func (s *atTimeTaskStore) All() []*ops.AtTimeTask {
  result := make([]*ops.AtTimeTask, len(s.Tasks))
  copy(result, s.Tasks)
  return result
}

func (s *atTimeTaskStore) Add(t *ops.AtTimeTask) {
  s.Activity <- t
}

func (s *atTimeTaskStore) Remove(id string) {
  s.Activity <- id
}

func (s *atTimeTaskStore) VerifyNoInteraction(t *testing.T) {
  if activity := nextActivity(s.Activity, false); activity != nil {
    t.Errorf("Expected no interaction, got %v", activity)
  }
}

func (s *atTimeTaskStore) VerifyAdded(
    t *testing.T, expected *ops.AtTimeTask, wait bool) {
  result := nextActivity(s.Activity, wait)
  actual, ok := result.(*ops.AtTimeTask)
  if !ok {
    t.Errorf("Expected %v added.", expected)
    return
  }
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v added, got %v", expected, actual)
  }
}

func (s *atTimeTaskStore) VerifyRemoved(
    t *testing.T, expected string, wait bool) {
  result := nextActivity(s.Activity, wait)
  actual, ok := result.(string)
  if !ok {
    t.Errorf("Expected %s removed.", expected)
    return
  }
  if actual != expected {
    t.Errorf("Expected %s removed, got %s", expected, actual)
  }
}

func verifyScheduled(
    t *testing.T,
    expected []*ops.AtTimeTask,
    actual []*utils.TimerTaskWrapper) {
  alen := len(actual)
  elen := len(expected)
  if alen != elen {
    t.Errorf("Expected length %d, got %d", elen, alen)
    return
  }
  for i := range expected {
    if !reflect.DeepEqual(expected[i].H, actual[i].H) {
      t.Errorf(
          "At index %d, expected %v, got %v", i, expected[i].H, actual[i].H)
    }
    if !reflect.DeepEqual(expected[i].Ls, actual[i].Ls) {
      t.Errorf(
          "At index %d, expected %s, got %s", i, expected[i].Ls, actual[i].Ls)
    }
    if !reflect.DeepEqual(expected[i].StartTime, actual[i].StartTime) {
      t.Errorf(
          "At index %d, expected %s, got %s",
          i, expected[i].StartTime, actual[i].StartTime)
    }
  }
}

func nextActivity(activity <-chan interface{}, wait bool) interface{} {
  if wait {
    return <-activity
  } else {
    select {
      case result := <-activity:
        return result
      default:
        return nil
    }
  }
}

