package author

import (
	"fmt"
	"sort"

	"ganglion/internal/realize"
)

// Next picks a meaning to write for (08/W8, D93): the frame with the fewest
// templates, with distinctive fillers. Every fifth pick is a deliberate joke
// when the grammar has a coerced entity (02/L8, D61). n varies the choice.
func Next(w *World, n int, frameID string) (Meaning, error) {
	counts := map[string]int{}
	for _, t := range w.Templates {
		counts[t.Means.Frame]++
	}
	var frames []string
	for id := range w.Frames {
		frames = append(frames, id)
	}
	if len(frames) == 0 {
		return Meaning{}, fmt.Errorf("no frames")
	}
	sort.Slice(frames, func(i, j int) bool {
		if counts[frames[i]] != counts[frames[j]] {
			return counts[frames[i]] < counts[frames[j]]
		}
		return frames[i] < frames[j]
	})
	joke := frameID == "" && n%5 == 4
	id := frameID
	if id == "" {
		id = frames[n%len(frames)]
		if n%len(frames) != 0 && counts[id] > counts[frames[0]] {
			id = frames[0]
		}
	}
	f, ok := w.Frames[id]
	if !ok {
		return Meaning{}, fmt.Errorf("unknown frame %s", id)
	}
	m := Meaning{Frame: id, Roles: map[string]Filler{}, Features: realize.Features{Tense: "present", Aspect: "simple", Polarity: "positive", Mood: "statement"}}
	if id == "frame:give" {
		m.Features.Tense = "past"
	}
	m.Why = fmt.Sprintf("%s has %d template%s so far, the fewest.", f.Title, counts[id], map[bool]string{true: "", false: "s"}[counts[id] == 1])
	if frameID != "" {
		m.Why = "Picked by hand."
	}
	used := map[string]bool{}
	for i, r := range f.Roles {
		if r.Type == "property" {
			continue
		}
		pool := w.EntitiesOf(r.Type)
		// people when the frame allows them, so names make alignment exact (03/A1)
		var people []string
		for _, e := range pool {
			if w.Entities[e].Name != "" {
				people = append(people, e)
			}
		}
		if r.Name != "thing" && r.Name != "holder" && len(people) > 0 {
			pool = people
		} else if r.Name == "thing" || r.Name == "holder" {
			var things []string
			for _, e := range pool {
				if w.Entities[e].Name == "" {
					things = append(things, e)
				}
			}
			if len(things) > 0 {
				pool = things
			}
		}
		for k := 0; k < len(pool); k++ {
			e := pool[(n*7+i*3+k)%len(pool)]
			if !used[e] {
				m.Roles[r.Name] = Filler{Entity: e}
				used[e] = true
				break
			}
		}
	}
	for _, r := range f.Roles {
		if r.Type != "property" {
			continue
		}
		holder := holderType(m, w)
		var attrs []string
		for aid, a := range w.Attributes {
			for _, t := range a.AppliesTo {
				if w.IsA(holder, t) {
					attrs = append(attrs, aid)
					break
				}
			}
		}
		sort.Strings(attrs)
		if len(attrs) == 0 {
			return Meaning{}, fmt.Errorf("no attribute fits %s", holder)
		}
		a := w.Attributes[attrs[n%len(attrs)]]
		if id == "frame:compare" {
			m.Roles[r.Name] = Filler{Value: &Value{Attribute: a.Name, Dir: "more"}}
		} else {
			m.Roles[r.Name] = Filler{Value: &Value{Attribute: a.Name, Value: a.Scale[len(a.Scale)-1]}}
		}
	}
	if joke {
		for eid, e := range w.Entities {
			if e.Coerced == "" {
				continue
			}
			if _, ok := w.Frames["frame:give"]; ok {
				m = Meaning{Frame: "frame:give", Features: realize.Features{Tense: "past", Aspect: "simple", Polarity: "positive", Mood: "statement"},
					Roles: map[string]Filler{"giver": {Entity: "entity:mara"}, "thing": {Entity: eid}, "recipient": {Entity: "entity:tom"}},
					Joke:  "A feeling treated as a physical object (sd/02/L6c).", Why: "Humour on purpose: a coerced type (sd/02/D61)."}
			}
			break
		}
	}
	return m, nil
}
