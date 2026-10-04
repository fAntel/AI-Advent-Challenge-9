package minichat

import (
	agent "advent-agent"
	"encoding/json"
	"time"
)

type Settings struct {
	Generation agent.Settings  `json:"generation"`
	Retrieval  agent.RAGConfig `json:"retrieval"`
}

func Defaults(rag agent.RAGConfig) Settings {
	return Settings{Generation: agent.NormalizeRAG(agent.Settings{Model: agent.FlashModel, Reasoning: "none", RAG: true}), Retrieval: rag}
}

type Fact struct {
	Value string `json:"value"`
	Turn  string `json:"turn"`
	Quote string `json:"quote"`
}
type Memory struct {
	Goal        Fact            `json:"goal"`
	Constraints map[string]Fact `json:"constraints"`
	Definitions map[string]Fact `json:"definitions"`
}
type Change struct {
	Kind    string `json:"kind"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Turn    string `json:"turn"`
	Quote   string `json:"quote"`
	Replace bool   `json:"replace"`
}
type Extraction struct {
	Platform string   `json:"platform"`
	Query    string   `json:"query"`
	Changes  []Change `json:"changes"`
}
type Call struct {
	Purpose  string           `json:"purpose"`
	Messages []agent.Message  `json:"messages"`
	Response agent.Completion `json:"response"`
	Error    string           `json:"error,omitempty"`
}
type Revision struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
	Memory Memory `json:"memory"`
}
type Turn struct {
	ID          string                `json:"id"`
	Question    string                `json:"question"`
	State       string                `json:"state"`
	Error       string                `json:"error,omitempty"`
	Before      Memory                `json:"memory_before"`
	After       Memory                `json:"memory_after"`
	Extraction  *Extraction           `json:"extraction,omitempty"`
	Conflicts   []string              `json:"conflicts,omitempty"`
	Preparation *agent.RAGPreparation `json:"preparation,omitempty"`
	Evidence    *agent.Evidence       `json:"evidence,omitempty"`
	Result      *agent.RAGResult      `json:"result,omitempty"`
	Answer      string                `json:"answer,omitempty"`
	Calls       []Call                `json:"calls,omitempty"`
}
type Session struct {
	Version   int        `json:"version"`
	ID        string     `json:"id"`
	Created   time.Time  `json:"created"`
	Updated   time.Time  `json:"updated"`
	Settings  Settings   `json:"settings"`
	Memory    Memory     `json:"memory"`
	Revisions []Revision `json:"revisions,omitempty"`
	Turns     []*Turn    `json:"turns"`
}

func cloneMemory(m Memory) Memory {
	var c Memory
	b, _ := json.Marshal(m)
	_ = json.Unmarshal(b, &c)
	return c
}
func (s *Session) Pending() *Turn {
	if len(s.Turns) == 0 {
		return nil
	}
	t := s.Turns[len(s.Turns)-1]
	if t.State == "completed" || t.State == "discarded" {
		return nil
	}
	return t
}
func (s *Session) Recent() []agent.Message {
	var pairs [][]agent.Message
	for _, t := range s.Turns {
		if t.State == "completed" {
			pairs = append(pairs, []agent.Message{{Role: "user", Content: t.Question}, {Role: "assistant", Content: t.Answer}})
		}
	}
	if len(pairs) > 4 {
		pairs = pairs[len(pairs)-4:]
	}
	var out []agent.Message
	for _, pair := range pairs {
		out = append(out, pair...)
	}
	return out
}
func (s *Session) Questions() []string {
	out := []string{}
	for _, t := range s.Turns {
		if t.State != "discarded" {
			out = append(out, t.Question)
		}
	}
	return out
}
