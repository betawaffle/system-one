package author

import (
	"fmt"
	"sort"
	"strings"

	"ganglion/internal/realize"
)

// Next picks a meaning to write for (08/W8, D93): the frame with the fewest
// templates, with distinctive fillers. Every fifth pick is a deliberate joke
// when the grammar has a coerced entity (02/L8, D61). n varies the choice.
func Next(w *World, n int, frameID string, used map[string]bool) (Meaning, error) {
	return NextSkipping(w, n, frameID, used, nil)
}

// NextSkipping is Next, leaving out frames the user has had enough of for now
// (08/W8c), unless every frame is skipped or one is asked for by hand.
func NextSkipping(w *World, n int, frameID string, used map[string]bool, skip map[string]bool) (Meaning, error) {
	// skip meanings already written for, so Next keeps offering new ones
	var first Meaning
	var err error
	for k := 0; k < 97; k++ {
		m, e := nextAt(w, n+k, frameID, skip)
		if e != nil {
			if k == 0 {
				err = e
			}
			continue
		}
		if k == 0 {
			first = m
		}
		if !used[Signature(m)] {
			return m, nil
		}
	}
	return first, err
}

// Signature identifies a meaning by frame and fillers, ignoring features.
func Signature(m Meaning) string {
	var parts []string
	for role, fl := range m.Roles {
		v := fl.Entity
		if fl.Value != nil {
			v = fl.Value.Attribute + "=" + fl.Value.Value + fl.Value.Dir
		}
		parts = append(parts, role+":"+v)
	}
	sort.Strings(parts)
	return m.Frame + "|" + strings.Join(parts, ",")
}

func nextAt(w *World, n int, frameID string, skip map[string]bool) (Meaning, error) {
	counts := map[string]int{}
	for _, t := range w.Templates {
		counts[t.Means.Frame]++
	}
	var frames []string
	for id := range w.Frames {
		if !skip[id] {
			frames = append(frames, id)
		}
	}
	if len(frames) == 0 { // everything skipped: skip nothing
		for id := range w.Frames {
			frames = append(frames, id)
		}
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
	stride := 1
	// shuffle the walk so consecutive presses differ in every role, not just
	// the first: step through all combinations by a large prime
	total := 8 // attribute choices times scale ends, at most
	for _, r := range f.Roles {
		if !ValueRole(r.Type) {
			total *= max(1, len(w.EntitiesOf(r.Type)))
		}
	}
	step := 7919
	for gcd(step, total) != 1 {
		step += 2
	}
	n = (n*step + 1) % total
	for _, r := range f.Roles {
		if ValueRole(r.Type) {
			continue
		}
		pool := w.EntitiesOf(r.Type)
		if r.Name == "place" { // somewhere a relation fits, by type and size (06/F6a, F6e)
			pool = placeable(pool, w, w.Entities[m.Roles["thing"].Entity])
		}
		// people when the frame allows them, so names make alignment exact (03/A1)
		var people []string
		for _, e := range pool {
			if w.Entities[e].Name != "" {
				people = append(people, e)
			}
		}
		if r.Name != "thing" && r.Name != "holder" && r.Name != "place" && len(people) > 0 {
			pool = people
		} else if r.Name == "thing" || r.Name == "holder" {
			var things []string
			for _, e := range pool {
				// things, not people; and not places, which aren't in or on anything
				if w.Entities[e].Name == "" && !w.IsA(w.Entities[e].Type, "place") {
					things = append(things, e)
				}
			}
			if len(things) > 0 {
				pool = things
			}
		}
		// each role takes its own digit of n, so successive n walk through
		// every combination of fillers instead of a short cycle
		pick := (n / stride) % len(pool)
		stride *= len(pool)
		for k := 0; k < len(pool); k++ {
			e := pool[(pick+k)%len(pool)]
			if !used[e] {
				m.Roles[r.Name] = Filler{Entity: e}
				used[e] = true
				break
			}
		}
	}
	for _, r := range f.Roles {
		if !ValueRole(r.Type) {
			continue
		}
		holder := holderType(m, w)
		var attrs []string
		for aid, a := range w.Attributes {
			if a.Kind == KindFor(r.Type) && a.Applies(holder, w) {
				attrs = append(attrs, aid)
			}
		}
		sort.Strings(attrs)
		if len(attrs) == 0 {
			return Meaning{}, fmt.Errorf("no attribute fits %s", holder)
		}
		a := w.Attributes[attrs[(n/stride)%len(attrs)]]
		stride *= len(attrs)
		end := (n / stride) % 2 // both ends of the scale: old and new, tall and short
		stride *= 2
		if r.Type == "relation" { // any value with a word for this place (06/F6a)
			var vals []string
			th, pl := w.Entities[m.Roles["thing"].Entity], w.Entities[m.Roles["place"].Entity]
			for _, v := range a.Scale {
				if a.HasWord(v, holder, w) && w.FitsIn(th, pl, v) {
					vals = append(vals, v)
				}
			}
			if len(vals) == 0 {
				return Meaning{}, fmt.Errorf("no relation fits %s and %s", th.Noun, pl.Noun)
			}
			m.Roles[r.Name] = Filler{Value: &Value{Attribute: a.Name, Value: vals[((n/stride)*2+end)%len(vals)]}}
		} else if id == "frame:compare" {
			// more, less, and the same ("as old as"), which Next never offered
			m.Roles[r.Name] = Filler{Value: &Value{Attribute: a.Name, Dir: []string{"more", "less", "same"}[(end+2*((n/stride)%2))%3]}}
			stride *= 2
		} else {
			m.Roles[r.Name] = Filler{Value: &Value{Attribute: a.Name, Value: a.Scale[[]int{len(a.Scale) - 1, 0}[end]]}}
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

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// placeable keeps entities that some relation fits, by type and, when the
// thing is known, by size: places, containers, surfaces.
func placeable(pool []string, w *World, thing Entity) []string {
	var out []string
	for _, id := range pool {
		pl := w.Entities[id]
		if !w.Fits(pl, "physical") {
			continue
		}
		fits := false
		for _, a := range w.Attributes {
			if a.Kind != "relation" || !a.Applies(pl.Type, w) {
				continue
			}
			for _, v := range a.Scale {
				if a.HasWord(v, pl.Type, w) && (thing.Type == "" || w.FitsIn(thing, pl, v)) {
					fits = true
				}
			}
		}
		if fits {
			out = append(out, id)
		}
	}
	return out
}
