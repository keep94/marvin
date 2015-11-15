// Package utils contains common routines for the hue web application.
package utils

import (
  "fmt"
  "github.com/keep94/marvin/lights"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/tasks"
  "github.com/keep94/tasks/recurring"
  "html/template"
  "log"
  "reflect"
  "sync"
  "time"
)

// Recurring represents recurring time with an ID and description.
// These instances must be treated as immutable.
type Recurring struct {
  Id int
  recurring.R
  Description string
}

// BackgroundRunner runs a single task in the background.
// BackgroundRunner is safe to use with multiple goroutines.
type BackgroundRunner struct {
  task tasks.Task
  runner *tasks.SingleExecutor
}

func NewBackgroundRunner(task tasks.Task) *BackgroundRunner {
  return &BackgroundRunner{task: task, runner: tasks.NewSingleExecutor()}
}

// IsEnabled returns true if the task is running.
func (br *BackgroundRunner) IsEnabled() bool {
  _, e := br.runner.Current()
  return e != nil
}

// Enable runs the task.
func (br *BackgroundRunner) Enable() {
  if !br.IsEnabled() {
    br.runner.Start(br.task)
  }
}

// Disable stops the task.
func (br *BackgroundRunner) Disable() {
  _, e := br.runner.Current()
  if e != nil {
    e.End()
    <-e.Done()
  }
}

// FutureHueTask represents a future hue task.
type FutureHueTask interface {

  // Refresh returns the HueTask
  Refresh() *ops.HueTask

  // GetDescription returns the description.
  GetDescription() string
}

// ScheduledTask represents a scheduled task that runs periodically to
// operate hue lights.
// ScheduledTask instances should not be copied via assignment operator.
// Multiple goroutines can enable or disable this scheduled task via
// its BackgroundRunner. Other than that, ScheduledTask instances are
// to be treated as immutable.
type ScheduledTask struct {
  // ID of the scheduled task
  Id int
  // Description of the scheduled task
  Description string
  // Requested lights for scheduled task
  Lights lights.Set
  // When to run. nil means running always.
  Times *Recurring
  // If false this scheduled task won't interrupt already running tasks.
  HighPriority bool
  *BackgroundRunner
}

// HueTaskToScheduledTask creates a ScheduledTask from a FutureHueTask.
// id is the id of the new ScheduledTask.
// h is the FutureHueTask.
// lightSet is the lights h is to run on.
// r is when h should run.
// hiPriority is true if h should preempt other tasks when run.
// te is what runs h.
func HueTaskToScheduledTask(
    id int,
    h FutureHueTask,
    lightSet lights.Set,
    r *Recurring,
    hiPriority bool,
    te *MultiExecutor) *ScheduledTask {
  var atask tasks.Task
  if hiPriority {
    atask = tasks.TaskFunc(func(e *tasks.Execution) {
      te.Start(h.Refresh(), lightSet)
    })
  } else {
    atask = tasks.TaskFunc(func(e *tasks.Execution) {
      te.MaybeStart(h.Refresh(), lightSet)
    })
  }
  result := TaskToScheduledTask(id, h.GetDescription(), r, atask)
  result.Lights = lightSet
  result.HighPriority = hiPriority
  return result
}

// TaskToScheduledTask creates a ScheduledTask from an ordinary task.
// id is the id of the new HueTaskToScheduledTask.
// description is a description for task.
// r is when task should run. If nil, task runs all the time.
// task is the original task.
func TaskToScheduledTask(
    id int,
    description string,
    r *Recurring,
    task tasks.Task) *ScheduledTask {
  if r != nil {
    task = tasks.RecurringTask(task, r)
  }
  return &ScheduledTask{
      Id: id,
      Description: description,
      Times: r,
      BackgroundRunner: NewBackgroundRunner(task),
  }
}

// ScheduledTaskList represents an immutable list of scheduled tasks.
type ScheduledTaskList []*ScheduledTask

// ToMap returns this ScheduledTaskList as a map keyed by Id
func (l ScheduledTaskList) ToMap() map[int]*ScheduledTask {
  result := make(map[int]*ScheduledTask, len(l))
  for _, st := range l {
    result[st.Id] = st
  }
  return result
}
 
// MultiExecutor executes hue tasks while ensuring that no more than
// one task is controlling any given light. MultiExecutor is safe to use
// with multiple goroutines.
type MultiExecutor struct {
  me *tasks.MultiExecutor
  c ops.Context
  hlog *log.Logger
}

// NewMultiExecutor creates a new MultiExecutor instance.
// c is the connection to the hue bridge. c should implement all the methods
// beyond the Context interface that HueTask instances passed to Start
// and MaybeStart need. If a HueTask needs a method that c does not implement
// then it does nothing. hlog captures the start of each HueTask along with
// its ending or interruption.
func NewMultiExecutor(c ops.Context, hlog *log.Logger) *MultiExecutor {
  return &MultiExecutor{
      me: tasks.NewMultiExecutor(&TaskCollection{}),
      c: c,
      hlog: hlog,
  }
}

// MaybeStart is like Start but avoids interrupting running tasks by
// either not running h or by running h on a subset of the lights in
// lightSet.
func (m *MultiExecutor) MaybeStart(
    h *ops.HueTask, lightSet lights.Set) *tasks.Execution {
  runningTasks := m.Tasks()

  // If there are not running tasks, start this one.
  if len(runningTasks) == 0 {
    return m.Start(h, lightSet)
  }

  neededLights := h.UsedLights(lightSet)
  if neededLights.IsNone() {
    return nil
  }

  // There are running tasks, and this task uses all the lights.
  // Don't run this task.
  if neededLights.IsAll() {
    return nil
  }

  // Calculate lightsInUse. If a running task uses all
  // lights give up don't run this task.
  lightsInUse := make(lights.Set)
  for _, hueTaskWrapper := range runningTasks {
    if hueTaskWrapper.Ls.IsAll() {
      return nil
    }
    lightsInUse.MutableAdd(hueTaskWrapper.Ls)
  }

  neededAndAvailableLights := neededLights.Subtract(lightsInUse)

  // Oops no available lights that we need. Return without running task
  if neededAndAvailableLights.IsNone() {
    return nil
  }

  lightsThatWillBeUsed := h.UsedLights(neededAndAvailableLights)
  if lightsThatWillBeUsed.IsNone() {
    return nil
  }

  // Because of the axioms, lightsThatWillBeUsed is a subset of
  // neededLights. When we subtract the needed and available lights,
  // what we have left are the lights that are needed but not available.
  // We make sure this set is empty before running the task.
  if lightsThatWillBeUsed.Subtract(neededAndAvailableLights).IsNone() {
    return m.Start(h, lightsThatWillBeUsed)
  }
  return nil
}

// Start starts a task for a suggested set of lights. Start
// interrupts any running task using the lights that h needs before
// starting h. Start returns the execution of h.
func (m *MultiExecutor) Start(
    h *ops.HueTask, lightSet lights.Set) *tasks.Execution {
  usedLights := h.UsedLights(lightSet)
  if usedLights.IsNone() {
    return nil
  }
  return m.me.Start(
      &HueTaskWrapper{H: h, Ls: usedLights, c: m.c, log: m.hlog})
}

// Begin is a synonym for Start. Needed to implement HueTaskBeginner.
func (m *MultiExecutor) Begin(
    h *ops.HueTask, lightSet lights.Set) {
  m.Start(h, lightSet)
}

// Pause pauses this executor waiting for all tasks to actually stop.
// Pause() and Resume() must be called from the same goroutine.
// Calling Pause() and Resume() concurrently from different goroutines
// causes undefined behavior and may cause Pause() to block indefinitely.
func (m *MultiExecutor) Pause() {
  m.me.Pause()
}

// Resume resumes this executor.
// Pause() and Resume() must be called from the same goroutine.
// Calling Pause() and Resume() concurrently from different goroutines
// causes undefined behavior and may cause Pause() to block indefinitely.
func (m *MultiExecutor) Resume() {
  m.me.Resume()
}

// Tasks returns the current HueTasks being run
func (m *MultiExecutor) Tasks() []*HueTaskWrapper {
  var result []*HueTaskWrapper
  m.me.Tasks().(*TaskCollection).Tasks(&result)
  return result
}

// Stop stops a particular task. taskId is the ID of the task
// as returned by HueTaskWrapper.TaskId().
func (m *MultiExecutor) Stop(taskId string) {
  e := m.me.Tasks().(*TaskCollection).FindByTaskId(taskId)
  if e != nil {
    e.End()
    <-e.Done()
  }
}

// Close closes resources associated with this instance and interrupts all
// running tasks in this instance.
func (m *MultiExecutor) Close() error {
  return m.me.Close()
}

// Interface AtTimeTaskStore keeps persistent storage of all scheduled tasks
// in a MultiTimer.
type AtTimeTaskStore interface {
  // Returns all stored tasks
  All() []*ops.AtTimeTask

  // Removes a particular task by schedule Id
  Remove(scheduleId string)

  // Adds a new task
  Add(task *ops.AtTimeTask)
}

// Interface HueTaskBeginner can begin a hue task. MultiExecutor
// implements this interface.
type HueTaskBeginner interface {
  Begin(t *ops.HueTask, ls lights.Set)
}

// MultiTimer schedules hue tasks to run at certain times.
// MultiTimer is safe to use wit multiple goroutines.
type MultiTimer struct {
  executor HueTaskBeginner
  scheduler *tasks.MultiExecutor
  store AtTimeTaskStore
}

// NewMultiTimer creates a new MultiTimer. executor is the MultiExecutor
// to which this instance will send hue tasks.
func NewMultiTimer(executor HueTaskBeginner) *MultiTimer {
  return NewMultiTimerWithStoreAndClock(
      executor, nilAtTimeTaskStore{}, tasks.SystemClock())
}

// NewMultiTimerWithStore creates a new MultiTimer.
// executor is the MultiExecutor to which this instance will send hue tasks.
// store handles the persistent storage of tasks.
func NewMultiTimerWithStore(
    executor HueTaskBeginner, store AtTimeTaskStore) *MultiTimer {
  return NewMultiTimerWithStoreAndClock(
      executor, store, tasks.SystemClock())
}

// NewMultiTimerWithStoreAndClock provides a caller supplied clock for
// testing
func NewMultiTimerWithStoreAndClock(
    executor HueTaskBeginner,
    store AtTimeTaskStore,
    clock tasks.Clock) *MultiTimer {
  result := &MultiTimer{
      executor: executor,
      scheduler: tasks.NewMultiExecutorWithClock(&TaskCollection{}, clock),
      store: store}
  tasks := store.All()
  for i := range tasks {
    result.schedule(tasks[i].H, tasks[i].Ls, tasks[i].StartTime)
  }
  return result
}

func (m *MultiTimer) schedule(
    h *ops.HueTask, usedLights lights.Set, startTime time.Time) string {
  wrapper := &TimerTaskWrapper{
      H: h,
      Ls: usedLights,
      StartTime: startTime,
      executor: m.executor,
      store: m.store}
  m.scheduler.Start(wrapper)
  return wrapper.TaskId()
}

// Schedule schedules a hue task to be run.
// h is the hue task; lightSet is suggested set of lights for which the
// task should run;
// startTime is the time that the hue task should run.
func (m *MultiTimer) Schedule(
    h *ops.HueTask, lightSet lights.Set, startTime time.Time) {
  usedLights := h.UsedLights(lightSet)
  if usedLights.IsNone() {
    return
  }
  scheduleId := m.schedule(h, usedLights, startTime)
  m.store.Add(&ops.AtTimeTask{
      Id: scheduleId, H: h, Ls: usedLights, StartTime: startTime})
}

// Scheduled returns the tasks scheduled to be run.
func (m *MultiTimer) Scheduled() []*TimerTaskWrapper {
  var result []*TimerTaskWrapper
  m.scheduler.Tasks().(*TaskCollection).Tasks(&result)
  return result
}

// FindByScheduleId returns the execution that controls the scheduling of a
// task. scheduleId identifies the scheduling of the task and comes from
// TimerTaskWrapper.TaskId() which is different from the ID of a running task.
func (m *MultiTimer) FindByScheduleId(scheduleId string) *tasks.Execution {
  return m.scheduler.Tasks().(*TaskCollection).FindByTaskId(scheduleId)
}

// Cancel cancels a scheduled task. scheduleId comes from
// TimerTaskWrapper.TaskId() and identifies the scheduling of a task.
// This ID is different from the ID of a running task
func (m *MultiTimer) Cancel(taskId string) {
  e := m.FindByScheduleId(taskId)
  if e != nil {
    e.End()
    <-e.Done()
  }
}

// Interface LightReaderWriter can both read and update the state of lights
type LightReaderWriter interface {
  ops.Context
  ops.LightReader
}

// Stack consists of two MultiExecutors: the main one, Base, and an extra
// one Extra. Calling Push pauses Base, saves the state of the lights
// and resumes Extra. Then Extra can be used to run programs without
// messing up what was running in Base. Finally call Pop to pause Extra,
// restore the lights and resume Base as if no programs were ever run
// on Extra.
// Stack can be safely used with multiple goroutines.
type Stack struct {
  Base *MultiExecutor
  Extra *MultiExecutor
  // All the lights that this instance controls
  AllLights lights.Set
  context LightReaderWriter
  slog *log.Logger
  first chan struct{}
  second chan struct{}
  third chan struct{}
  fourth chan struct{}
}

// NewStack creates a new Stack instance. 
func NewStack(
    base, extra *MultiExecutor,
    context LightReaderWriter,
    allLights lights.Set,
    slog *log.Logger) *Stack {
  result := &Stack{
      Base: base,
      Extra: extra,
      AllLights: allLights,
      context: context,
      slog: slog,
      first: make(chan struct{}),
      second: make(chan struct{}),
      third: make(chan struct{}),
      fourth: make(chan struct{})}
  go result.loop()
  return result
}

func (s *Stack) Push() {
  var empty struct{}
  s.first <- empty
  <-s.second
}

func (s *Stack) Pop() {
  var empty struct{}
  s.third <- empty
  <-s.fourth
}

func (s *Stack) loop() {
  var empty struct{}
  for {
    <-s.first
    s.Base.Pause()

    // Be sure that commands that just finished running take effect before
    // taking the state of all the lights. By default, hue lights have a
    // 400ms fade in.
    time.Sleep(500 * time.Millisecond)
    lightColors, err := ops.Snapshot(s.context, s.AllLights)
    if err != nil {
      s.slog.Printf("ERROR: %v\n", err)
    }
    s.Extra.Resume()
    s.second <- empty
    <- s.third
    s.Extra.Pause()
    if lightColors != nil {
      err = ops.Restore(s.context, lightColors)
      if err != nil {
        s.slog.Printf("ERROR: %v\n", err)
      }
    }
    s.Base.Resume()  
    s.fourth <- empty
  }
}

// NewTemplate returns a new template instance. name is the name
// of the template; templateStr is the template string.
func NewTemplate(name, templateStr string) *template.Template {
  return template.Must(template.New(name).Parse(templateStr))
}

// Task represents a Task that works with TaskCollection
type Task interface {
  tasks.Task

  // Returns true if this instance conflicts with other.
  ConflictsWith(other Task) bool

  // Returns the task ID of this instance.
  TaskId() string
}

// TaskCollection represents running tasks and implements tasks.TaskCollection.
// It adds the Tasks method to get all running tasks and the FindByTaskId
// method to find the execution of a particular task.
type TaskCollection struct {
  rwmutex sync.RWMutex
  tasks []taskExecution
}

func (c *TaskCollection) Add(t tasks.Task, e *tasks.Execution) {
  task := t.(Task)
  c.rwmutex.Lock()
  defer c.rwmutex.Unlock()
  c.tasks = append(c.tasks, taskExecution{t: task, e: e})
}

func (c *TaskCollection) Remove(t tasks.Task) {
  task := t.(Task)
  c.rwmutex.Lock()
  defer c.rwmutex.Unlock()
  idx := -1
  for i := range c.tasks {
    if c.tasks[i].t == task {
      idx = i
      break
    }
  }
  if idx != -1 {
    copied := copy(c.tasks[idx:], c.tasks[idx + 1:])
    c.tasks = c.tasks[:idx + copied]
  }
}

func (c *TaskCollection) Conflicts(t tasks.Task) []*tasks.Execution {
  task, _ := t.(Task)
  c.rwmutex.RLock()
  defer c.rwmutex.RUnlock()
  result := make([]*tasks.Execution, len(c.tasks))
  idx := 0
  for i := range c.tasks {
    if task == nil || c.tasks[i].t.ConflictsWith(task) {
      result[idx] = c.tasks[i].e
      idx++
    }
  }
  return result[:idx]
}

// Gets all running tasks. aSlicePtr points to the slice to hold the
// running tasks.
func (c *TaskCollection) Tasks(aSlicePtr interface{}) {
  c.rwmutex.RLock()
  defer c.rwmutex.RUnlock()
  sliceValue := reflect.Indirect(reflect.ValueOf(aSlicePtr))
  sliceValue.Set(reflect.MakeSlice(
      sliceValue.Type(), len(c.tasks), len(c.tasks)))
  for i := range c.tasks {
    sliceValue.Index(i).Set(reflect.ValueOf(c.tasks[i].t))
  }
}

// FindByTaskId returns the execution of a particular task or nil if that
// task is not found.
func (c *TaskCollection) FindByTaskId(taskId string) *tasks.Execution {
  c.rwmutex.RLock()
  defer c.rwmutex.RUnlock()
  for i := range c.tasks {
    if c.tasks[i].t.TaskId() == taskId {
      return c.tasks[i].e
    }
  }
  return nil
}
  
// HueTaskWrapper represents a hue task bound to a context and a light set.
// Implements Task.
type HueTaskWrapper struct {
  // The hue task
  H *ops.HueTask

  // Empty set means all lights
  Ls lights.Set

  // The context
  c ops.Context

  // The log
  log *log.Logger
}

// Do performs the task
func (t *HueTaskWrapper) Do(e *tasks.Execution) {
  // This added for testing for when there is no log.
  if t.log == nil {
    t.H.Do(t.c, t.Ls, e)
    return
  }
  t.log.Printf("START: %s", t)
  t.H.Do(t.c, t.Ls, e)
  if err := e.Error(); err != nil {
    t.log.Printf("ERROR: %s: %v\n", t, err)
  } else if e.IsEnded() {
    t.log.Printf("INTERRUPTED: %s", t)
  } else {
    t.log.Printf("FINISH: %s", t)
  }
}

func (t *HueTaskWrapper) ConflictsWith(other Task) bool {
  ls := t.Ls
  otherLs := other.(*HueTaskWrapper).Ls
  return ls.OverlapsWith(otherLs)
}

// TaskId is a combination of the hue task Id and the light set.
func (t *HueTaskWrapper) TaskId() string {
  return fmt.Sprintf("%d:%s", t.H.Id, t.Ls)
}

func (t *HueTaskWrapper) String() string {
  return fmt.Sprintf("{%d, %s, %s}", t.H.Id, t.H.Description, t.Ls)
}

// TimerTaskWrapper represents a hue task bound to a light set to start at
// a particular time. Implements Task.
type TimerTaskWrapper struct {

  // The hue task
  H *ops.HueTask

  // Empty set means all lights
  Ls lights.Set

  // The time to start
  StartTime time.Time

  executor HueTaskBeginner

  store AtTimeTaskStore

}

func (t *TimerTaskWrapper) Do(e *tasks.Execution) {
  d := t.StartTime.Sub(e.Now())
  if d > 0 && e.Sleep(d) {
    t.executor.Begin(t.H, t.Ls)
  }
  t.store.Remove(t.TaskId())
}

func (t *TimerTaskWrapper) ConflictsWith(other Task) bool {
  otherTask := other.(*TimerTaskWrapper)
  // We compare unix times to ensure that tasks with the same task ID
  // conflict.
  return t.StartTime.Unix() == otherTask.StartTime.Unix() && t.Ls.OverlapsWith(otherTask.Ls)
}

// TaskId is combination of hue task Id, light set, and start time
func (t *TimerTaskWrapper) TaskId() string {
  return fmt.Sprintf("%d:%d:%s", t.H.Id, t.StartTime.Unix(), t.Ls)
}

// TimeLeft returns the time left before the hue task starts
func (t *TimerTaskWrapper) TimeLeft(now time.Time) time.Duration {
  return t.StartTime.Sub(now)
}

// TimeLeftStr returns the time left before the hue task starts as m:ss
func (t *TimerTaskWrapper) TimeLeftStr(now time.Time) string {
  d := t.TimeLeft(now) + time.Second
  if d < 0 {
    d = 0
  }
  if d >= time.Hour {
    return fmt.Sprintf(
        "%d:%02d:%02d",
        d / time.Hour,
        (d % time.Hour) / time.Minute,
        (d % time.Minute) / time.Second)
  }
  return fmt.Sprintf(
      "%d:%02d",
      d / time.Minute,
      (d % time.Minute) / time.Second)
}

// FutureTime returns hour:minute as a future time from now.
// The returned time is the closest hour:minute from now that is just after
// now. The returned time is in the same timezone as now.
// hour is 0-23; minute is 0-59.
func FutureTime(now time.Time, hour, minute int) time.Time {
  var result time.Time
  s := recurring.AtTime(hour, minute).ForTime(now)
  defer s.Close()
  s.Next(&result)
  return result
}

type taskExecution struct {
  t Task
  e *tasks.Execution
}

type nilAtTimeTaskStore struct {
}

func (n nilAtTimeTaskStore) All() []*ops.AtTimeTask {
  return nil
}

func (n nilAtTimeTaskStore) Remove(id string) {
}

func (n nilAtTimeTaskStore) Add(task *ops.AtTimeTask) {
}
