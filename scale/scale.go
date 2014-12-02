// Package scale handles color scales and other types of scales
package scale

import (
  "github.com/keep94/gohue"
  "sort"
)

// CEntry represents an entry in a color scale
type CEntry struct {
  Value float64
  Color gohue.Color
}

// Color represents a color scale. Entries must be sorted by Value in
// ascending order.
type Color []CEntry


// Get converts x to a color. The returned color corresponds to the
// smallest value greater than or equal to x. If there are no such values,
// Get() returns the last color in this scale.
func (c Color) Get(x float64) gohue.Color {
  idx := c.search(x)
  if idx == len(c) {
    return c[idx - 1].Color
  }
  return c[idx].Color
}

// Interpolate works like Get except that it interpolates between the colors
// if x falls between two values in this scale.
func (c Color) Interpolate(x float64) gohue.Color {
  idx := c.search(x)
  if idx == len(c) {
    return c[idx - 1].Color
  }
  if idx == 0 {
    return c[0].Color
  }
  ratio := (x - c[idx - 1].Value) / (c[idx].Value - c[idx - 1].Value)
  return c[idx - 1].Color.Blend(c[idx].Color, ratio)
}

func (c Color) search(x float64) int {
  return sort.Search(len(c), func(i int) bool {
    return c[i].Value >= x
  })
}
