package render

import "strings"

// ModelName turns a model id such as "claude-opus-5-5" into "Opus 5.5".
// It finds "claude-" anywhere in the id (so provider prefixes like
// "us.anthropic." are skipped), ignores suffixes such as "[1m]", "@20250805",
// "-v1:0" and date segments, and returns display when the id has no
// "claude-" part.
func ModelName(id, display string) string {
	i := strings.Index(id, "claude-")
	if i < 0 {
		return display
	}
	core := id[i+len("claude-"):]
	for _, cut := range []string{"[", "@", ":"} {
		if j := strings.Index(core, cut); j >= 0 {
			core = core[:j]
		}
	}

	family := ""
	var nums []string
	for _, p := range strings.Split(core, "-") {
		switch {
		case p == "":
		case isDigits(p):
			if len(p) <= 2 {
				nums = append(nums, p)
			}
		case family == "":
			family = p
		}
	}
	if family == "" {
		return display
	}
	name := strings.ToUpper(family[:1]) + family[1:]
	if len(nums) > 2 {
		nums = nums[:2]
	}
	if len(nums) > 0 {
		name += " " + strings.Join(nums, ".")
	}
	return name
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
