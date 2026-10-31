package provider

import "testing"

func TestTemplateDataEqual(t *testing.T) {
	const info = "___INFO___\n\n"
	const code = "\n\n___SANDBOXED_JS_FOR_WEB_TEMPLATE___\n\n"
	for _, test := range []struct {
		name, left, right string
		equal             bool
	}{
		{"identical", "raw template", "raw template", true},
		{"JSON escapes", info + `{"help":"x > y & z < q 日本 😀"}`, info + `{"help":"x \u003e y \u0026 z \u003c q \u65e5\u672c \ud83d\ude00"}`, true},
		{"JSON formatting", info + `{"a":1,"b":2}`, info + "{\n  \"b\": 2, \"a\": 1\n}\n", true},
		{"ToS header", "___TERMS_OF_SERVICE___\n\nTerms here.\n\n" + info + `{}`, info + `{}`, true},
		{"changed help", info + `{"help":"before"}`, info + `{"help":"after"}`, false},
		{"literal escape", info + `{"help":"\\u003e"}`, info + `{"help":">"}`, false},
		{"changed code", info + `{}` + code + `const x = ">";`, info + `{}` + code + `const x = "\u003e";`, false},
		{"invalid JSON", info + `{"help":">",}`, info + `{"help":"\u003e",}`, false},
		{"trailing JSON", info + `{} true`, info + `{}`, false},
		{"missing section", info + `{}` + code + "run();", info + `{}`, false},
		{"unknown section", "___NOTES___\n>\n", "___NOTES___\n\\u003e\n", false},
		{"YAML tests", "___TESTS___\nscenarios: []\n", "___TESTS___\nscenarios: [ ]\n", false},
		{"large numbers", info + `{"n":9007199254740992}`, info + `{"n":9007199254740993}`, false},
		{"plain text", "before", "after", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := templateDataEqual(test.left, test.right); got != test.equal {
				t.Fatalf("templateDataEqual() = %t, want %t", got, test.equal)
			}
			if got := templateDataEqual(test.right, test.left); got != test.equal {
				t.Fatalf("reverse templateDataEqual() = %t, want %t", got, test.equal)
			}
		})
	}
}

func TestTemplateDataJSONSections(t *testing.T) {
	for _, section := range []string{"INFO", "TEMPLATE_PARAMETERS", "WEB_PERMISSIONS", "SERVER_PERMISSIONS"} {
		t.Run(section, func(t *testing.T) {
			prefix := "___" + section + "___\n\n"
			if !templateDataEqual(prefix+`[{"help":">"}]`, prefix+`[{"help":"\u003e"}]`) {
				t.Fatal("JSON escaping should compare equal")
			}
			if templateDataEqual(prefix+`[{"help":"before"}]`, prefix+`[{"help":"after"}]`) {
				t.Fatal("changed JSON values should compare unequal")
			}
		})
	}
}
