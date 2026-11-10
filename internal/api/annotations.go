package api

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Annotation is one annotation a call sends: a named value Sanshain stores
// with the provided version (Scope "version") or with the system (Scope
// "system"). Needs Sanshain Service 2.13.0 or later.
type Annotation struct {
	Scope string `json:"scope"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SetEnvPrefix names the environment variables that fill annotation
// placeholders: SANSHAIN_SET_ID fills {{ID}} when no --set ID=… does.
const SetEnvPrefix = "SANSHAIN_SET_"

// AnnotationsDroppedWarning is printed, once per client, when an older
// service refused the annotations and the call was repeated without them.
const AnnotationsDroppedWarning = "Sanshain Service before 2.13.0 does not take annotations; sending without them"

// placeholder is {{NAME}} with NAME of ASCII letters, digits and _ only;
// anything else between double braces is literal text.
var placeholder = regexp.MustCompile(`\{\{([A-Za-z0-9_]+)\}\}`)

// FillAnnotations fills the {{NAME}} placeholders of the declared
// annotations: the --set value for NAME first, else the environment variable
// SANSHAIN_SET_NAME (exact case). A placeholder counts as filled only with a
// non-empty value; an empty --set does not fall through to the environment.
// An entry with an unfilled placeholder is left out, with a warning naming it
// and its first unfilled placeholder; the others are kept, in order.
func FillAnnotations(declared []Annotation, sets map[string]string, getenv func(string) string) ([]Annotation, []string) {
	var filled []Annotation
	var warnings []string
	for _, a := range declared {
		value, missing := fillPlaceholders(a.Value, sets, getenv)
		if missing != "" {
			warnings = append(warnings, fmt.Sprintf(
				"annotation '%s' left out: {{%s}} is not set (--set %s=… or %s%s)",
				a.Name, missing, missing, SetEnvPrefix, missing))
			continue
		}
		a.Value = value
		filled = append(filled, a)
	}
	return filled, warnings
}

// fillPlaceholders returns template with every placeholder filled, and the
// name of the first one without a value ("" when all were filled).
func fillPlaceholders(template string, sets map[string]string, getenv func(string) string) (string, string) {
	missing := ""
	filled := placeholder.ReplaceAllStringFunc(template, func(match string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}")
		value, set := sets[name]
		if !set {
			value = getenv(SetEnvPrefix + name)
		}
		if value == "" && missing == "" {
			missing = name
		}
		return value
	})
	return filled, missing
}

// annotationsFieldRefused reports whether a refusal names annotations as the
// unknown field. Matched exactly: the refusals of 2.9–2.12 list every field
// they know after "expected one of", so the bare word is not enough.
func annotationsFieldRefused(body string) bool {
	return strings.Contains(body, "unknown field `annotations`")
}

// annotationsToSend is what a call may send of annotations: all of them on a
// provide, only the system ones on any other call, and none once the service
// refused them.
func (c *SanshainClient) annotationsToSend(annotations []Annotation, provide bool) []Annotation {
	if c.annotationsRefused {
		return nil
	}
	if provide {
		return annotations
	}
	var system []Annotation
	for _, a := range annotations {
		if a.Scope == "system" {
			system = append(system, a)
		}
	}
	return system
}

// refusedAnnotations reports whether a failed call that sent annotations was
// an older service refusing them by name with the given status. From then on
// this client sends none, and the first refusal prints a warning.
func (c *SanshainClient) refusedAnnotations(sent []Annotation, status, refusedStatus int, body string) bool {
	if len(sent) == 0 || status != refusedStatus || !annotationsFieldRefused(body) {
		return false
	}
	c.refuseAnnotations()
	return true
}

// refuseAnnotations stops this client sending annotations, warning the first
// time. The warning is a plain log line: strict mode does not fail on it.
func (c *SanshainClient) refuseAnnotations() {
	if !c.annotationsRefused {
		c.annotationsRefused = true
		fmt.Fprintln(os.Stderr, "⚠ "+AnnotationsDroppedWarning)
	}
}
