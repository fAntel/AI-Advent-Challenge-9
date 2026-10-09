package main

type task struct {
	Difficulty string `json:"difficulty"`
	Prompt     string `json:"prompt"`
}

var tasks = []task{
	{"easy", "Explain the difference between a stack and a queue, with one practical example of each. Answer in English, briefly."},
	{"medium", "Write a Go function Balanced(s string) bool that checks balanced brackets ()[]{}. Ignore all other characters. Include a table with input and expected output for empty input, nested brackets, mismatched types, and unclosed brackets. Explain time and space complexity in one sentence. Answer in English; keep the entire answer under 400 words."},
	{"hard", `Diagnose the two correctness bugs in this Go LRU cache: reads fail to refresh recency, and updates to existing keys corrupt bookkeeping. Provide a corrected implementation, explain both bugs briefly, and show two short operation sequences with expected results demonstrating the fixes. Do not write test functions or repeat the original code. Assume capacity is positive. Answer in English; keep the entire answer under 600 words, including code.

package cache
import "container/list"
type entry struct { key, value int }
type LRU struct { capacity int; order *list.List; items map[int]*list.Element }
func New(capacity int) *LRU {
    return &LRU{capacity: capacity, order: list.New(), items: make(map[int]*list.Element)}
}
func (c *LRU) Get(key int) (int, bool) {
    if e, ok := c.items[key]; ok { return e.Value.(entry).value, true }
    return 0, false
}
func (c *LRU) Put(key, value int) {
    e := c.order.PushFront(entry{key, value})
    c.items[key] = e
    if c.order.Len() > c.capacity {
        oldest := c.order.Back()
        delete(c.items, oldest.Value.(entry).key)
        c.order.Remove(oldest)
    }
}`},
}
