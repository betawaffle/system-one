package lexicon

// Our own tables of common irregular forms. WordNet's exception lists say which
// base an inflected form belongs to but not which slot it fills (gave is past,
// given is the participle), so for common verbs we say it outright. These are
// facts of English written here, not copied from a share-alike source (05/D75).

// irregularVerbs: base -> past, past participle.
var irregularVerbs = map[string][2]string{
	"arise": {"arose", "arisen"}, "awake": {"awoke", "awoken"}, "be": {"was", "been"}, "bear": {"bore", "borne"},
	"beat": {"beat", "beaten"}, "become": {"became", "become"}, "begin": {"began", "begun"}, "bend": {"bent", "bent"},
	"bet": {"bet", "bet"}, "bind": {"bound", "bound"}, "bite": {"bit", "bitten"}, "bleed": {"bled", "bled"},
	"blow": {"blew", "blown"}, "break": {"broke", "broken"}, "breed": {"bred", "bred"}, "bring": {"brought", "brought"},
	"build": {"built", "built"}, "burn": {"burned", "burned"}, "burst": {"burst", "burst"}, "buy": {"bought", "bought"},
	"catch": {"caught", "caught"}, "choose": {"chose", "chosen"}, "cling": {"clung", "clung"}, "come": {"came", "come"},
	"cost": {"cost", "cost"}, "creep": {"crept", "crept"}, "cut": {"cut", "cut"}, "deal": {"dealt", "dealt"},
	"dig": {"dug", "dug"}, "do": {"did", "done"}, "draw": {"drew", "drawn"}, "dream": {"dreamed", "dreamed"},
	"drink": {"drank", "drunk"}, "drive": {"drove", "driven"}, "eat": {"ate", "eaten"}, "fall": {"fell", "fallen"},
	"feed": {"fed", "fed"}, "feel": {"felt", "felt"}, "fight": {"fought", "fought"}, "find": {"found", "found"},
	"flee": {"fled", "fled"}, "fling": {"flung", "flung"}, "fly": {"flew", "flown"}, "forbid": {"forbade", "forbidden"},
	"forget": {"forgot", "forgotten"}, "forgive": {"forgave", "forgiven"}, "freeze": {"froze", "frozen"}, "get": {"got", "gotten"},
	"give": {"gave", "given"}, "go": {"went", "gone"}, "grind": {"ground", "ground"}, "grow": {"grew", "grown"},
	"hang": {"hung", "hung"}, "have": {"had", "had"}, "hear": {"heard", "heard"}, "hide": {"hid", "hidden"},
	"hit": {"hit", "hit"}, "hold": {"held", "held"}, "hurt": {"hurt", "hurt"}, "keep": {"kept", "kept"},
	"kneel": {"knelt", "knelt"}, "know": {"knew", "known"}, "lay": {"laid", "laid"}, "lead": {"led", "led"},
	"lean": {"leaned", "leaned"}, "leap": {"leaped", "leaped"}, "learn": {"learned", "learned"}, "leave": {"left", "left"},
	"lend": {"lent", "lent"}, "let": {"let", "let"}, "lie": {"lay", "lain"}, "light": {"lit", "lit"},
	"lose": {"lost", "lost"}, "make": {"made", "made"}, "mean": {"meant", "meant"}, "meet": {"met", "met"},
	"pay": {"paid", "paid"}, "prove": {"proved", "proven"}, "put": {"put", "put"}, "quit": {"quit", "quit"},
	"read": {"read", "read"}, "ride": {"rode", "ridden"}, "ring": {"rang", "rung"}, "rise": {"rose", "risen"},
	"run": {"ran", "run"}, "say": {"said", "said"}, "see": {"saw", "seen"}, "seek": {"sought", "sought"},
	"sell": {"sold", "sold"}, "send": {"sent", "sent"}, "set": {"set", "set"}, "shake": {"shook", "shaken"},
	"shed": {"shed", "shed"}, "shine": {"shone", "shone"}, "shoot": {"shot", "shot"}, "show": {"showed", "shown"},
	"shrink": {"shrank", "shrunk"}, "shut": {"shut", "shut"}, "sing": {"sang", "sung"}, "sink": {"sank", "sunk"},
	"sit": {"sat", "sat"}, "sleep": {"slept", "slept"}, "slide": {"slid", "slid"}, "sling": {"slung", "slung"},
	"speak": {"spoke", "spoken"}, "speed": {"sped", "sped"}, "spend": {"spent", "spent"}, "spin": {"spun", "spun"},
	"spit": {"spat", "spat"}, "split": {"split", "split"}, "spread": {"spread", "spread"}, "spring": {"sprang", "sprung"},
	"stand": {"stood", "stood"}, "steal": {"stole", "stolen"}, "stick": {"stuck", "stuck"}, "sting": {"stung", "stung"},
	"stink": {"stank", "stunk"}, "stride": {"strode", "stridden"}, "strike": {"struck", "struck"}, "string": {"strung", "strung"},
	"strive": {"strove", "striven"}, "swear": {"swore", "sworn"}, "sweep": {"swept", "swept"}, "swell": {"swelled", "swollen"},
	"swim": {"swam", "swum"}, "swing": {"swung", "swung"}, "take": {"took", "taken"}, "teach": {"taught", "taught"},
	"tear": {"tore", "torn"}, "tell": {"told", "told"}, "think": {"thought", "thought"}, "throw": {"threw", "thrown"},
	"tread": {"trod", "trodden"}, "understand": {"understood", "understood"}, "upset": {"upset", "upset"}, "wake": {"woke", "woken"},
	"wear": {"wore", "worn"}, "weave": {"wove", "woven"}, "weep": {"wept", "wept"}, "win": {"won", "won"},
	"wind": {"wound", "wound"}, "withdraw": {"withdrew", "withdrawn"}, "wring": {"wrung", "wrung"}, "write": {"wrote", "written"},
	"receive": {"received", "received"}, "hand": {"handed", "handed"},
}

// irregularPres3: third-person present forms the rules get wrong.
var irregularPres3 = map[string]string{"be": "is", "have": "has", "do": "does", "go": "goes"}

// irregularNouns: plurals missing from or wrong in the exception lists.
var irregularNouns = map[string]string{
	"person": "people", "child": "children", "man": "men", "woman": "women", "foot": "feet", "tooth": "teeth",
	"mouse": "mice", "goose": "geese", "ox": "oxen", "sheep": "sheep", "fish": "fish", "deer": "deer",
	"leaf": "leaves", "knife": "knives", "wife": "wives", "life": "lives", "half": "halves", "shelf": "shelves",
	"wolf": "wolves", "loaf": "loaves", "potato": "potatoes", "tomato": "tomatoes", "hero": "heroes", "echo": "echoes",
	"piano": "pianos", "photo": "photos", "radio": "radios", "zoo": "zoos", "roof": "roofs", "chef": "chefs",
	"bus": "buses", "box": "boxes", "lens": "lenses", "species": "species", "series": "series",
}

// irregularAdj: comparative, superlative.
var irregularAdj = map[string][2]string{
	"good": {"better", "best"}, "bad": {"worse", "worst"}, "far": {"farther", "farthest"},
	"little": {"less", "least"}, "many": {"more", "most"}, "much": {"more", "most"},
}

// uncountable nouns: no plural in ordinary use.
var uncountable = map[string]bool{
	"advice": true, "information": true, "rice": true, "furniture": true, "luggage": true, "news": true,
	"equipment": true, "knowledge": true, "music": true, "water": true, "bread": true, "money": true,
}
