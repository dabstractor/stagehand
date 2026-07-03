package prompt

import "strings"

// Gitmoji reference table — compiled-in canonical data (PRD §9.19 FR-F3 / §17.8).
//
// Source: https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json
//
//	(carloscuesta/gitmoji, packages/gitmojis/src/gitmojis.json)
//
// Verified 2026-07-02 — 75 entries (FR-D5 discipline; Appendix E #16).
// NO network fetch, ever (FR-F3): this is a build-time constant. To refresh, re-fetch the JSON,
// regenerate the literal (see research/regenerate_table.py), and update gitmojiVerifiedCount /
// gitmojiVerifiedDate.
const (
	gitmojiSourceURL     = "https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json"
	gitmojiVerifiedCount = 75           // FR-D5: entry count at last verification — TestGitmojiTableCount pins len(GitmojiTable) == this.
	gitmojiVerifiedDate  = "2026-07-02" // FR-D5 / Appendix E #16: date the table was last verified against gitmojiSourceURL.
)

// Gitmoji is a single entry in the canonical gitmoji reference table (PRD §9.19 FR-F3 / §17.8).
// The prompt renders Emoji + Description; Name (the :shortcode: minus colons) is the STABLE key kept
// for maintenance — it is stable across gitmoji versions (emoji glyphs / description wording drift;
// the name does not) and makes the data self-documenting. It is never rendered into a prompt.
type Gitmoji struct {
	Emoji       string // the emoji character, e.g. "🎨"
	Description string // its meaning, e.g. "Improve structure / format of the code."
	Name        string // the stable :shortcode: key, e.g. "art"
}

// GitmojiTable is the canonical gitmoji reference table compiled into the binary (PRD §9.19 FR-F3 / §17.8).
//
// Source: <gitmojiSourceURL> (carloscuesta/gitmoji, packages/gitmojis/src/gitmojis.json).
// Verified <gitmojiVerifiedDate> — <gitmojiVerifiedCount> entries (FR-D5 discipline; Appendix E #16).
// NO network fetch, ever (FR-F3): this is a build-time constant. To refresh, re-fetch the JSON,
// regenerate the literal (see research/regenerate_table.py), and update gitmojiVerifiedCount /
// gitmojiVerifiedDate.
//
// READ-ONLY: treat as an immutable constant. Callers iterate / render (RenderGitmojiTable); do not mutate.
var GitmojiTable = []Gitmoji{
	{Emoji: "🎨", Description: "Improve structure / format of the code.", Name: "art"},
	{Emoji: "⚡️", Description: "Improve performance.", Name: "zap"},
	{Emoji: "🔥", Description: "Remove code or files.", Name: "fire"},
	{Emoji: "🐛", Description: "Fix a bug.", Name: "bug"},
	{Emoji: "🚑️", Description: "Critical hotfix.", Name: "ambulance"},
	{Emoji: "✨", Description: "Introduce new features.", Name: "sparkles"},
	{Emoji: "📝", Description: "Add or update documentation.", Name: "memo"},
	{Emoji: "🚀", Description: "Deploy stuff.", Name: "rocket"},
	{Emoji: "💄", Description: "Add or update the UI and style files.", Name: "lipstick"},
	{Emoji: "🎉", Description: "Begin a project.", Name: "tada"},
	{Emoji: "✅", Description: "Add, update, or pass tests.", Name: "white-check-mark"},
	{Emoji: "🔒️", Description: "Fix security or privacy issues.", Name: "lock"},
	{Emoji: "🔐", Description: "Add or update secrets.", Name: "closed-lock-with-key"},
	{Emoji: "🔖", Description: "Release / Version tags.", Name: "bookmark"},
	{Emoji: "🚨", Description: "Fix compiler / linter warnings.", Name: "rotating-light"},
	{Emoji: "🚧", Description: "Work in progress.", Name: "construction"},
	{Emoji: "💚", Description: "Fix CI Build.", Name: "green-heart"},
	{Emoji: "⬇️", Description: "Downgrade dependencies.", Name: "arrow-down"},
	{Emoji: "⬆️", Description: "Upgrade dependencies.", Name: "arrow-up"},
	{Emoji: "📌", Description: "Pin dependencies to specific versions.", Name: "pushpin"},
	{Emoji: "👷", Description: "Add or update CI build system.", Name: "construction-worker"},
	{Emoji: "📈", Description: "Add or update analytics or track code.", Name: "chart-with-upwards-trend"},
	{Emoji: "♻️", Description: "Refactor code.", Name: "recycle"},
	{Emoji: "➕", Description: "Add a dependency.", Name: "heavy-plus-sign"},
	{Emoji: "➖", Description: "Remove a dependency.", Name: "heavy-minus-sign"},
	{Emoji: "🔧", Description: "Add or update configuration files.", Name: "wrench"},
	{Emoji: "🔨", Description: "Add or update development scripts.", Name: "hammer"},
	{Emoji: "🌐", Description: "Internationalization and localization.", Name: "globe-with-meridians"},
	{Emoji: "✏️", Description: "Fix typos.", Name: "pencil2"},
	{Emoji: "💩", Description: "Write bad code that needs to be improved.", Name: "poop"},
	{Emoji: "⏪️", Description: "Revert changes.", Name: "rewind"},
	{Emoji: "🔀", Description: "Merge branches.", Name: "twisted-rightwards-arrows"},
	{Emoji: "📦️", Description: "Add or update compiled files or packages.", Name: "package"},
	{Emoji: "👽️", Description: "Update code due to external API changes.", Name: "alien"},
	{Emoji: "🚚", Description: "Move or rename resources (e.g.: files, paths, routes).", Name: "truck"},
	{Emoji: "📄", Description: "Add or update license.", Name: "page-facing-up"},
	{Emoji: "💥", Description: "Introduce breaking changes.", Name: "boom"},
	{Emoji: "🍱", Description: "Add or update assets.", Name: "bento"},
	{Emoji: "♿️", Description: "Improve accessibility.", Name: "wheelchair"},
	{Emoji: "💡", Description: "Add or update comments in source code.", Name: "bulb"},
	{Emoji: "🍻", Description: "Write code drunkenly.", Name: "beers"},
	{Emoji: "💬", Description: "Add or update text and literals.", Name: "speech-balloon"},
	{Emoji: "🗃️", Description: "Perform database related changes.", Name: "card-file-box"},
	{Emoji: "🔊", Description: "Add or update logs.", Name: "loud-sound"},
	{Emoji: "🔇", Description: "Remove logs.", Name: "mute"},
	{Emoji: "👥", Description: "Add or update contributor(s).", Name: "busts-in-silhouette"},
	{Emoji: "🚸", Description: "Improve user experience / usability.", Name: "children-crossing"},
	{Emoji: "🏗️", Description: "Make architectural changes.", Name: "building-construction"},
	{Emoji: "📱", Description: "Work on responsive design.", Name: "iphone"},
	{Emoji: "🤡", Description: "Mock things.", Name: "clown-face"},
	{Emoji: "🥚", Description: "Add or update an easter egg.", Name: "egg"},
	{Emoji: "🙈", Description: "Add or update a .gitignore file.", Name: "see-no-evil"},
	{Emoji: "📸", Description: "Add or update snapshots.", Name: "camera-flash"},
	{Emoji: "⚗️", Description: "Perform experiments.", Name: "alembic"},
	{Emoji: "🔍️", Description: "Improve SEO.", Name: "mag"},
	{Emoji: "🏷️", Description: "Add or update types.", Name: "label"},
	{Emoji: "🌱", Description: "Add or update seed files.", Name: "seedling"},
	{Emoji: "🚩", Description: "Add, update, or remove feature flags.", Name: "triangular-flag-on-post"},
	{Emoji: "🥅", Description: "Catch errors.", Name: "goal-net"},
	{Emoji: "💫", Description: "Add or update animations and transitions.", Name: "dizzy"},
	{Emoji: "🗑️", Description: "Deprecate code that needs to be cleaned up.", Name: "wastebasket"},
	{Emoji: "🛂", Description: "Work on code related to authorization, roles and permissions.", Name: "passport-control"},
	{Emoji: "🩹", Description: "Simple fix for a non-critical issue.", Name: "adhesive-bandage"},
	{Emoji: "🧐", Description: "Data exploration/inspection.", Name: "monocle-face"},
	{Emoji: "⚰️", Description: "Remove dead code.", Name: "coffin"},
	{Emoji: "🧪", Description: "Add a failing test.", Name: "test-tube"},
	{Emoji: "👔", Description: "Add or update business logic.", Name: "necktie"},
	{Emoji: "🩺", Description: "Add or update healthcheck.", Name: "stethoscope"},
	{Emoji: "🧱", Description: "Infrastructure related changes.", Name: "bricks"},
	{Emoji: "🧑‍💻", Description: "Improve developer experience.", Name: "technologist"},
	{Emoji: "💸", Description: "Add sponsorships or money related infrastructure.", Name: "money-with-wings"},
	{Emoji: "🧵", Description: "Add or update code related to multithreading or concurrency.", Name: "thread"},
	{Emoji: "🦺", Description: "Add or update code related to validation.", Name: "safety-vest"},
	{Emoji: "✈️", Description: "Improve offline support.", Name: "airplane"},
	{Emoji: "🦖", Description: "Code that adds backwards compatibility.", Name: "t-rex"},
}

// RenderGitmojiTable renders the PRD §17.8 "emoji + meaning" reference block: one line per entry,
// "<emoji> - <description>". It is the recommended way for the gitmoji format-mode scaffold (S3,
// P1.M2.T1.S3) to embed the table; S3 may also iterate GitmojiTable directly if a different separator
// is wanted (the two are additive). PURE (no I/O); returns a string with NO trailing newline (package
// convention — the caller owns inter-block newline placement, mirroring BuildSystemPrompt/BuildUserPayload).
func RenderGitmojiTable() string {
	var b strings.Builder
	for i, g := range GitmojiTable {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(g.Emoji)
		b.WriteString(" - ")
		b.WriteString(g.Description)
	}
	return b.String()
}
