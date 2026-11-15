{{define "common"}}You check one thing about an English sentence that a program generated from a meaning. Answer only the question asked; other checks cover everything else. A person reviews your answer afterwards; when you're unsure, say so with low confidence.

How to read a sample: the meaning says what the sentence is about. The sample's tags set its tense (past, present, future), whether it affirms or denies the meaning (positive or negative), and whether it's a statement or a question. So "This ladder wasn't old." is a correct sample of "holder: ladder, property: age = old" with the tags past, negative, statement. Tags for roles say who or what fills them, including words like "the", "this", "that".

Put the words your answer is about in "words" (empty if none), and say why in "why", under 12 words.{{end}}

{{define "form"}}{{template "common" .}}

Question: is any word in the sample spelled or inflected wrong? For example "gived", "childs", "a umbrella", "more tall". Ignore everything except the form of each word.{{end}}

{{define "grammar"}}{{template "common" .}}

Question: is the sample broken English: wrong word order, agreement, or a missing or extra word? Ignore whether it's true, sensible or natural; only whether it's grammatical.{{end}}

{{define "filler"}}{{template "common" .}}

Question: is something in the sample the wrong kind of thing for its role, so the sentence can't literally be true? For example "The idea is tall" (an idea has no height), "The rock gave Lee a book" (a rock can't give). Answer yes only when it's impossible, not merely unusual.{{end}}

{{define "meaning"}}{{template "common" .}}

Question: does the sample's literal meaning differ from its tags? Yes if it says something the tags don't (an extra fact, a different relation, a different person or thing), or leaves out something they say. A negative tag denying the meaning is not a difference. Positive or negative says whether the sentence affirms or denies; a question with the positive tag asks without "not" ("Is Kai shorter than Noor?"), which is not a difference either. Don't count what a reader might infer; only what the words say.{{end}}

{{define "change"}}{{template "common" .}}

The meaning is a state, something that holds over time.

Question: does the sample's tense suggest the fact is different at some other time? "Will be X" suggests it isn't X now; "was X" suggests it no longer is. Present tense usually doesn't. Answer "answer" yes or no.

If yes, write in "then" the opposite fact the sample suggests, at the time it suggests it, in plain words (for "Ana will be taller than Lee": "now Ana is not taller than Lee"). Then answer "can_change": could the fact go from "then" to what the sample says, given what these things are and how they change? Think it through: a person can grow taller than another; but everyone ages at the same rate, so the older of two people stays the older one; an apple only gets older. If "answer" is no, leave "then" empty and set "can_change" to "yes".{{end}}

{{define "implies"}}{{template "common" .}}

Question: would an ordinary reader assume something extra from the sample, beyond what its words say? For example "won't give" suggesting a refusal, or "gives" in the present suggesting a habit.

Don't count anything the literal meaning already says or entails: a negative means the positive is false ("isn't shorter" means the same or taller); "her chair" says she owns it; a question asks whether something is so. Don't count what's already recorded for it. Answer yes only when the extra assumption is one most readers would make.{{if .Notes}}

Earlier notes the person wrote. If one describes exactly what this sample implies, use its wording in "why"; otherwise write your own:
{{range .Notes}}- {{.}}
{{end}}{{end}}{{end}}

{{define "context"}}{{template "common" .}}

Question: would the sample be confusing as the first sentence of a conversation, even with a scene in front of both people, so that it needs an earlier sentence? Names, "the", "this", "that" and possessives are all fine to open with: the listener can know the people, or see the things. Answer yes only when it can't stand first, for example "He did too." or "Not that one either." Say in "why" what would need to come before it.{{if .Notes}}

Earlier notes the person wrote. If one describes exactly what this sample needs, use its wording in "why"; otherwise write your own:
{{range .Notes}}- {{.}}
{{end}}{{end}}{{end}}

{{define "user"}}Meaning: {{.Frame}}: {{.Explainer}}
Roles:
{{range .Roles}}- {{.}}
{{end}}
The author wrote this sentence for the meaning (it's known to be good): "{{.Authored}}"

Sample to check: "{{.Sample}}"
Its tags: {{.Tags}}
{{if .Known}}Already recorded for this sample: {{.Known}}
{{end}}{{end}}
