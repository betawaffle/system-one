// Package author is the writing loop's logic (design/synthetic-data/03, 04,
// 08): the grammar's content types, alignment of a sentence with its meaning,
// building a template from it, and generating samples from templates.
package author

import (
	"fmt"
	"sort"
	"strings"

	"ganglion/internal/grammar"
	"ganglion/internal/realize"
)

// Content types stored in the grammar (09/Z4).

type Type struct {
	Name      string `json:"name"`
	Parent    string `json:"parent"`
	Explainer string `json:"explainer"`
}

type Attribute struct {
	Name      string   `json:"name"`
	AppliesTo []string `json:"applies_to"`
	// Scale lists values low to high; Words gives each value's adjective,
	// by holder type with "*" as the default (06/P4).
	Scale []string                     `json:"scale"`
	Words map[string]map[string]string `json:"words"`
}

type Role struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // an entity type, or "property" for an attribute value (06/P1)
	Generic   string `json:"generic"`
	Explainer string `json:"explainer"`
	Optional  bool   `json:"optional"`
}

type Frame struct {
	Name      string   `json:"name"`
	Title     string   `json:"title"`
	Explainer string   `json:"explainer"`
	Roles     []Role   `json:"roles"`
	Verbs     []string `json:"verbs"`
	Converse  []string `json:"converse"`
}

type Entity struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Noun     string `json:"noun,omitempty"`
	Pronouns string `json:"pronouns"`
	Coerced  string `json:"coerced,omitempty"` // treated as this type on purpose (02/L6c)
}

// Display is how an entity is written in tags and the meaning card.
func (e Entity) Display() string {
	if e.Name != "" {
		return e.Name
	}
	return e.Noun
}

// Value fills a property role: an attribute's value, or for comparisons the
// attribute and a direction.
type Value struct {
	Attribute string `json:"attribute"`
	Value     string `json:"value,omitempty"` // has-property: "old"
	Dir       string `json:"dir,omitempty"`   // compare: more, less, same
}

// Filler is what fills one role in a meaning.
type Filler struct {
	Entity string `json:"entity,omitempty"` // entity id, e.g. "entity:mara"
	Value  *Value `json:"value,omitempty"`
}

// Meaning is what the user writes for (02/L5).
type Meaning struct {
	Frame     string            `json:"frame"` // frame id
	Roles     map[string]Filler `json:"roles"`
	Features  realize.Features  `json:"features"`
	Modifiers map[string]string `json:"modifiers,omitempty"`
	Joke      string            `json:"joke,omitempty"`
	Why       string            `json:"why,omitempty"` // why Next picked it (08/W8)
}

// World is a loaded grammar: everything alignment and generation need.
type World struct {
	Types      map[string]Type
	Attributes map[string]Attribute
	Frames     map[string]Frame
	Entities   map[string]Entity
	Templates  map[string]Template
}

// Load reads the current heads of every kind.
func Load(g *grammar.Store) (*World, error) {
	w := &World{Types: map[string]Type{}, Attributes: map[string]Attribute{}, Frames: map[string]Frame{}, Entities: map[string]Entity{}, Templates: map[string]Template{}}
	for _, v := range g.Heads("") {
		var err error
		switch v.Kind {
		case "type":
			var x Type
			_, err = g.Get(v.ID, &x)
			w.Types[v.ID] = x
		case "attribute":
			var x Attribute
			_, err = g.Get(v.ID, &x)
			w.Attributes[v.ID] = x
		case "frame":
			var x Frame
			_, err = g.Get(v.ID, &x)
			w.Frames[v.ID] = x
		case "entity":
			var x Entity
			_, err = g.Get(v.ID, &x)
			w.Entities[v.ID] = x
		case "template":
			var x Template
			_, err = g.Get(v.ID, &x)
			w.Templates[v.ID] = x
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", v.ID, err)
		}
	}
	return w, nil
}

// IsA reports whether type t is want or below it in the type tree (06/Y1).
func (w *World) IsA(t, want string) bool {
	for i := 0; t != "" && i < 32; i++ {
		if t == want {
			return true
		}
		t = w.Types["type:"+t].Parent
	}
	return false
}

// Fits reports whether an entity can fill a role, counting deliberate coercion.
func (w *World) Fits(e Entity, roleType string) bool {
	return w.IsA(e.Type, roleType) || (e.Coerced != "" && w.IsA(e.Coerced, roleType))
}

// EntitiesOf lists entity ids that fit a role type, sorted, without coerced ones.
func (w *World) EntitiesOf(roleType string) []string {
	var out []string
	for id, e := range w.Entities {
		if e.Coerced == "" && w.IsA(e.Type, roleType) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Word is the adjective for an attribute value on a holder type (06/P4).
func (a Attribute) Word(value, holderType string, w *World) string {
	ws := a.Words[value]
	for t := holderType; t != ""; t = w.Types["type:"+t].Parent {
		if x, ok := ws[t]; ok {
			return x
		}
	}
	return ws["*"]
}

// Ends are the adjectives at the low and high ends of a scale, for comparisons.
func (a Attribute) Ends(holderType string, w *World) (low, high string) {
	if len(a.Scale) == 0 {
		return "", ""
	}
	return a.Word(a.Scale[0], holderType, w), a.Word(a.Scale[len(a.Scale)-1], holderType, w)
}

// IsName reports whether a word is an entity's name, possibly possessive.
func (w *World) IsName(word string) bool {
	if word == "I" {
		return true
	}
	word = strings.TrimSuffix(word, "'s")
	for _, e := range w.Entities {
		if e.Name == word {
			return true
		}
	}
	return false
}
