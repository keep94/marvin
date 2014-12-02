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

// Recurring represents recurring time with an ID and description
type Recurring struct {
  Id int
  recurring.R
  Description string
}

// BackgroundRunner runs a single task in the background
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

// ScheduledTask represents a scheduled task.
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

// ScheduledTaskList represents a list of scheduled tasks.
type ScheduledTaskList []*ScheduledTask

// ToMap returns this ScheduledTaskList as a map keyed by Id
func (l ScheduledTaskList) ToMap() map[int]*ScheduledTask {
  result := make(map[int]*ScheduledTask, len(l))
  for _, st := range l {
    result[st.Id] = st
  }
  return result
}
 
// MultiExecutor executes hue tasks.
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

// Start starts a hue tasks for a suggested set of lights.
func (m *MultiExecutor) Start(
    h *ops.HueTask, lightSet lights.Set) *tasks.Execution {
  usedLights := h.UsedLights(lightSet)
  if usedLights.IsNone() {
    return nil
  }
  return m.me.Start(
      &HueTaskWrapper{H: h, Ls: usedLights, c: m.c, log: m.hlog})
}

// Pause pauses this executor waiting for all tasks to actually stop.
func (m *MultiExecutor) Pause() {
  m.me.Pause()
}

// Resume resumes this executor.
func (m *MultiExecutor) Resume() {
  m.me.Resume()
}

// Tasks returns the current HueTasks being run
func (m *MultiExecutor) Tasks() []*HueTaskWrapper {
  var result []*HueTaskWrapper
  m.me.Tasks().(*TaskCollection).Tasks(&result)
  return result
}

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

// MultiTimer schedules hue tasks to run at certain times.
type MultiTimer struct {
  executor *MultiExecutor
  scheduler *tasks.MultiExecutor
}

// NewMultiTimer creates a new MultiTimer. executor is the MultiExecutor
// to which this instance will send hue tasks.
func NewMultiTimer(executor *MultiExecutor) *MultiTimer {
  return &MultiTimer{
      executor: executor,
      scheduler: tasks.NewMultiExecutor(&TaskCollection{})}
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
  m.scheduler.Start(
      &TimerTaskWrapper{
          H: h,
          Ls: usedLights,
          Executor: m.executor,
          StartTime: startTime})
}

// Scheduled returns the tasks scheduled to be run.
func (m *MultiTimer) Scheduled() []*TimerTaskWrapper {
  var result []*TimerTaskWrapper
  m.scheduler.Tasks().(*TaskCollection).Tasks(&result)
  return result
}

// Cancel cancels scheduled task with given task ID
func (m *MultiTimer) Cancel(taskId string) {
  e := m.scheduler.Tasks().(*TaskCollection).FindByTaskId(taskId)
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
      err = tasks.Run(tasks.TaskFunc(func(e *tasks.Execution) {
        ops.StaticHueAction(lightColors).Do(s.context, s.AllLights, e)
      }))
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

  // Runs the task when it is time
  Executor *MultiExecutor

  // The time to start
  StartTime time.Time
}

func (t *TimerTaskWrapper) Do(e *tasks.Execution) {
  d := t.StartTime.Sub(e.Now())
  if d <= 0 {
    return
  }
  if e.Sleep(d) {
    t.Executor.Start(t.H, t.Ls)
  }
}

func (t *TimerTaskWrapper) ConflictsWith(other Task) bool {
  return false
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

