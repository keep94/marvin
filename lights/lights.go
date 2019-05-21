// Package lights provides ways to represent a set of lights
package lights

import (
  "errors"
  "sort"
  "strconv"
  "strings"
)

var (
  // None represents no lights.
  None = make(Set, 0)

  // All represents all lights.
  All Set = nil
)

// Set represents a set of positive light Ids. nil represents all lights;
// An empty map or a map containing only false values represents no lights.
// Callers should treat Set instances as immutable.
type Set map[int]bool

// New builds a new Set.
func New(lightIds... int) Set {
  lightSet := make(Set, len(lightIds))
  for i := range lightIds {
    lightSet[lightIds[i]] = true
  }
  return lightSet
}

// Unlike Parse, InvString is the exact inverse of String.
func InvString(s string) (result Set, err error) {
  if s == "All" {
    return nil, nil
  }
  if s == "None" {
    return None, nil
  }
  return Parse(s)
}

// Parse parses comma separated light Ids as a Set.
// An empty string or a string with just spaces parses as all lights.
// Currently Parse will never return an instance representing no lights.
func Parse(s string) (result Set, err error) {
  s = strings.TrimSpace(s)
  if len(s) == 0 {
    return
  }
  parts := strings.Split(s, ",")
  for i := range parts {
    parts[i] = strings.TrimSpace(parts[i])
  }
  lightSet := make(Set, len(parts))
  for i := range parts {
    var light int
    if light, err = strconv.Atoi(parts[i]); err != nil {
      return
    }
    if light <= 0 {
      err = errors.New("Only positive light Ids allowed.")
      return
    }
    lightSet[light] = true
  }
  result = lightSet
  return 
}

// Slice returns this instance as a slice of light ids sorted in
// ascending order and true. If this instance represents all lights,
// returns an empty slice and true. If this instance represents no lights,
// returns an empty slice and false.
func (l Set) Slice() (result []int, ok bool) {
  if l == nil {
    return make([]int, 0), true
  }
  result = make([]int, len(l))
  idx := 0
  for i := range l {
    if l[i] {
      result[idx] = i
      idx++
    }
  }
  result = result[:idx]
  sort.Ints(result)
  ok = len(result) > 0
  return
}

// OverlapsWith returns true if this instance and other share common lights
func (l Set) OverlapsWith(other Set) bool {
  if l == nil {
    return !other.IsNone()
  }
  if other == nil {
    return !l.IsNone()
  }
  if len(l) > len(other) {
    l, other = other, l
  }
  for i := range l {
    if l[i] && other[i] {
      return true
    }
  }
  return false
}

// Intersect returns the intersection of this instance and other.
func (l Set) Intersect(other Set) Set {
  if l == nil {
    return other
  }
  if other == nil {
    return l
  }
  if len(l) > len(other) {
    l, other = other, l
  }
  result := make(Set, len(l))
  for i := range l {
    if l[i] && other[i] {
      result[i] = true
    }
  }
  return result
}
  

// Subtract returns the light ids that are in this instance but not other.
// Subtract panics if this instance represents all lights.
func (l Set) Subtract(other Set) Set {
  if l == nil {
    panic("Cannot subtract from All lights.")
  }
  if other == nil {
    return None
  }
  result := make(Set, len(l))
  for i := range l {
    if l[i] && !other[i] {
      result[i] = true
    }
  }
  return result
}

// IsAll returns true if this instance represents all lights.
func (l Set) IsAll() bool {
  return l == nil
}

// IsNone returns true if this instance has no lights.
func (l Set) IsNone() bool {
  if l == nil {
    return false
  }
  for i := range l {
    if l[i] {
      return false
    }
  }
  return true
}

// Add returns the union of this instance and other.
func (l Set) Add(other Set) Set {
  if l == nil || other == nil {
    return nil
  }
  result := make(Set, len(l) + len(other))
  return result.mutableAdd(l).mutableAdd(other)
}

// String returns the lights comma separated in ascending order or
// "All" if this instance represents all lights or "None" if this instance
// represents no lights..
func (l Set) String() string {
  if l == nil {
    return "All"
  }
  intSlice, ok := l.Slice()
  if !ok {
    return "None"
  }
  stringSlice := make([]string, len(intSlice))
  for i := range intSlice {
    stringSlice[i] = strconv.Itoa(intSlice[i])
  }
  return strings.Join(stringSlice, ",")
}

func (l Set) mutableAdd(other Set) Set {
  if other == nil {
    panic("MutableAdd cannot take All lights as parameter.")
  }
  for i := range other {
    if other[i] {
      l[i] = true
    }
  }
  return l
}

// Builder builds Set instances. The zero value is an empty Builder
// ready for use.
type Builder struct {
  set Set
  readOnly bool
}

// NewBuilder returns a new instance with other as its initial contents.
func NewBuilder(other Set) *Builder {
  return &Builder{set: other, readOnly: true}
}

// Clear changes this instance to contain no lights.
func (b *Builder) Clear() *Builder {
  b.set = make(Set)
  b.readOnly = false
  return b
}

// AddOne adds one light to this instance.
func (b *Builder) AddOne(light int) *Builder {
  b.makeWritable()
  b.set[light] = true
  return b
}

// Add adds the lights in other to this instance.
func (b *Builder) Add(other Set) *Builder {
  b.makeWritable()
  b.set.mutableAdd(other)
  return b
}

// Build returns the lights in this instance as a Set.
func (b *Builder) Build() Set {
  b.lazyInit()
  b.readOnly = true
  return b.set
}

func (b *Builder) lazyInit() {
  if b.set == nil {
    b.Clear()
  }
}

func (b *Builder) makeWritable() {
  b.lazyInit()
  if b.readOnly {
    writableSet := make(Set, len(b.set))
    b.set = writableSet.mutableAdd(b.set)
    b.readOnly = false
  }
}

// Map represents a map of virtual light Ids to physical light ids.
// When a light fails, its replacement will be given a new id.
// This data structure allows a light to keep the same virtual id
// in the marvin system  even after replacing it.
// The key is the virtual light Id, the value is the physical light id.
// If there is no mapping for a virtual light id, it maps to itself.
// Map instances are to be treated as immutable.
type Map map[int]int

// Convert converts a virtual light Id to a physical light id.
func (m Map) Convert(virtualId int) int {
  result, ok := m[virtualId]
  if !ok {
    return virtualId
  }
  return result
}
