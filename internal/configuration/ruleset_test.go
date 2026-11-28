package configuration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// TestDocumentRulesets は、宣言の並びを宣言の順のまま読むことと、ASCII の大文字小文字を区別しない名前の重複の拒否を確かめる。
//
//trace:checks FR-001
func TestDocumentRulesets(t *testing.T) {
	declaration := func(name string) RulesetDeclaration {
		return RulesetDeclaration{Name: mustRulesetName(t, name), Desired: setting.RulesetValue(baseRuleset())}
	}
	cases := []struct {
		name       string
		src        string
		want       []RulesetDeclaration
		wantPath   string
		wantReason string
	}{
		{name: "one ruleset is read", src: rulesetSequence("protect"), want: []RulesetDeclaration{declaration("protect")}},
		{name: "rulesets are read in the declared order", src: rulesetSequence("zeta", "alpha"), want: []RulesetDeclaration{declaration("zeta"), declaration("alpha")}},
		{name: "empty sequence is rejected", src: "[]\n", wantPath: "R", wantReason: "must not be empty"},
		{name: "mapping is rejected as not a sequence", src: rulesetBase, wantPath: "R", wantReason: "must be a sequence"},
		{name: "null is rejected", src: "~\n", wantPath: "R", wantReason: "must not be null or empty"},
		{name: "invalid ruleset is rejected at its index", src: rulesetSequence("a") + "- name: b\n", wantPath: "R[1]", wantReason: `missing key "target"`},
		{name: "declaration of absence alone is read as a sequence of one", src: "- name: legacy\n  state: absent\n", want: []RulesetDeclaration{{Name: mustRulesetName(t, "legacy"), Desired: setting.Absent()}}},
		{name: "declaration of absence is read in the declared order beside a value", src: rulesetSequence("protect") + "- name: legacy\n  state: absent\n", want: []RulesetDeclaration{declaration("protect"), {Name: mustRulesetName(t, "legacy"), Desired: setting.Absent()}}},
		{name: "absence after a value of the same name is rejected at the later declaration", src: rulesetSequence("protect") + "- name: protect\n  state: absent\n", wantPath: "R[1]", wantReason: `duplicate name "protect"; same as R[0] (compared ignoring letter case)`},
		{name: "value after an absence of the same name is rejected at the later declaration", src: "- name: protect\n  state: absent\n" + rulesetSequence("protect"), wantPath: "R[1]", wantReason: `duplicate name "protect"; same as R[0] (compared ignoring letter case)`},
		{name: "same name is rejected at the later ruleset", src: rulesetSequence("a", "b", "a"), wantPath: "R[2]", wantReason: `duplicate name "a"; same as R[0] (compared ignoring letter case)`},
		{name: "name differing only in letter case is rejected as a duplicate", src: rulesetSequence("protect", "Protect"), wantPath: "R[1]", wantReason: `duplicate name "Protect"; same as R[0] (compared ignoring letter case)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.rulesets(doc.root, "R")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("rulesets() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("rulesets() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("rulesets() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("rulesets() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rulesets() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentRuleset は、state を書かない要素は 5 つの値のキーがすべて必須であることと、state: absent の組み合わせ、誤りを示す順序、name・target・enforcement の判定を確かめる。
// 拒否のケースは、誤りを示す順序を確かめるものを除き、受理される入力 (rulesetBase と absentBase) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001 INV-001
func TestDocumentRuleset(t *testing.T) {
	with := func(old, replacement string) string { return strings.Replace(rulesetBase, old, replacement, 1) }
	withEnforcement := func(e setting.RulesetEnforcement) setting.Ruleset {
		r := baseRuleset()
		r.Enforcement = e
		return r
	}
	declaration := func(name string, r setting.Ruleset) RulesetDeclaration {
		return RulesetDeclaration{Name: mustRulesetName(t, name), Desired: setting.RulesetValue(r)}
	}
	const absentBase = "name: legacy\nstate: absent\n"
	cases := []struct {
		name       string
		src        string
		want       RulesetDeclaration
		wantPath   string
		wantReason string
	}{
		{name: "name and state absent are read as the declaration of absence", src: absentBase, want: RulesetDeclaration{Name: mustRulesetName(t, "legacy"), Desired: setting.Absent()}},
		{name: "state absent with rules of a wrong content is rejected at rules without reading the content", src: absentBase + "rules:\n  - type: creation\n", wantPath: "R.rules", wantReason: `rules is not allowed when state is "absent"`},
		{name: "state absent with several value keys is rejected at the first in the key order, not the written order", src: absentBase + "rules: []\nenforcement: active\n", wantPath: "R.enforcement", wantReason: `enforcement is not allowed when state is "absent"`},
		{name: "state present is rejected at the state", src: "name: legacy\nstate: present\n", wantPath: "R.state", wantReason: `must be absent; got "present"`},
		{name: "state present with all five value keys is rejected at the state, before the value keys", src: rulesetBase + "state: present\n", wantPath: "R.state", wantReason: `must be absent; got "present"`},
		{name: "name against the naming rule with state absent is rejected at the name", src: "name: Protéct\nstate: absent\n", wantPath: "R.name", wantReason: `must consist of printable ASCII characters; got "Protéct"`},
		{name: "all five keys are read into the name and the value", src: rulesetBase, want: declaration("protect", baseRuleset())},
		{name: "enforcement disabled is accepted", src: with("enforcement: active", "enforcement: disabled"), want: declaration("protect", withEnforcement(setting.RulesetEnforcementDisabled))},
		{name: "name with inner space is accepted keeping its spelling", src: with("name: protect", `name: "Default branch"`), want: declaration("Default branch", baseRuleset())},
		{name: "missing name is rejected", src: strings.TrimPrefix(rulesetBase, "name: protect\n"), wantPath: "R", wantReason: `missing key "name"`},
		{name: "missing target is rejected", src: with("target: branch\n", ""), wantPath: "R", wantReason: `missing key "target"`},
		{name: "missing enforcement is rejected", src: with("enforcement: active\n", ""), wantPath: "R", wantReason: `missing key "enforcement"`},
		{name: "missing conditions is rejected", src: with("conditions:\n  ref_name:\n    include: [\"~DEFAULT_BRANCH\"]\n    exclude: []\n", ""), wantPath: "R", wantReason: `missing key "conditions"`},
		{name: "missing bypass_actors is rejected without suggesting an empty sequence", src: with("bypass_actors:\n  - actor_type: RepositoryRole\n    actor_id: 5\n    bypass_mode: always\n", ""), wantPath: "R", wantReason: `missing key "bypass_actors"`},
		{name: "missing rules is rejected", src: with("rules:\n  - type: deletion\n", ""), wantPath: "R", wantReason: `missing key "rules"`},
		{name: "first missing key is reported when several are missing", src: "name: protect\nrules: []\n", wantPath: "R", wantReason: `missing key "target"`},
		{name: "id key is rejected and the expected keys include state", src: rulesetBase + "id: 1\n", wantPath: "R", wantReason: `unknown key "id"; expected name, target, enforcement, conditions, bypass_actors, rules, state`},
		{name: "state written twice is rejected as a duplicate key", src: absentBase + "state: absent\n", wantPath: "R", wantReason: `duplicate key "state"`},
		{name: "non-ASCII name is rejected", src: with("name: protect", "name: Protéct"), wantPath: "R.name", wantReason: `must consist of printable ASCII characters; got "Protéct"`},
		{name: "number name is rejected", src: with("name: protect", "name: 42"), wantPath: "R.name", wantReason: "must be a string"},
		{name: "target tag is rejected", src: with("target: branch", "target: tag"), wantPath: "R.target", wantReason: `must be branch; got "tag"`},
		{name: "non-string target is rejected", src: with("target: branch", "target: 1"), wantPath: "R.target", wantReason: "must be a string"},
		{name: "enforcement evaluate is rejected with the reason for personal accounts", src: with("enforcement: active", "enforcement: evaluate"), wantPath: "R.enforcement", wantReason: `must be active or disabled; got "evaluate"; GitHub rejects evaluate for repositories owned by personal accounts`},
		{name: "unknown enforcement is rejected", src: with("enforcement: active", "enforcement: strict"), wantPath: "R.enforcement", wantReason: `must be active or disabled; got "strict"`},
		{name: "non-string enforcement is rejected", src: with("enforcement: active", "enforcement: true"), wantPath: "R.enforcement", wantReason: "must be a string"},
		{name: "invalid conditions is rejected at conditions", src: with("exclude: []", "exclude: [main]"), wantPath: "R.conditions.ref_name.exclude[0]", wantReason: "must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/"},
		{name: "invalid bypass_actors is rejected at bypass_actors", src: with("actor_id: 5", "actor_id: 3"), wantPath: "R.bypass_actors[0].actor_id", wantReason: "must be 2, 4, or 5"},
		{name: "invalid rules is rejected at rules", src: with("- type: deletion", "- type: creation"), wantPath: "R.rules[0].type", wantReason: "must be pull_request, deletion, required_status_checks, or non_fast_forward"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.ruleset(doc.root, "R")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("ruleset() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("ruleset() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("ruleset() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("ruleset() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ruleset() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentRuleset_MissingKeyDoesNotSuggestAbsence は、state を書かず値のキーが欠けた要素の誤りが、state: absent を案内しないことを確かめる。
// 案内すると、キーを書き忘れた利用者を削除の宣言へ誘導するため。
//
//trace:checks INV-001
func TestDocumentRuleset_MissingKeyDoesNotSuggestAbsence(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantReason string
	}{
		{name: "name alone", src: "name: protect\n", wantReason: `missing key "target"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.ruleset(doc.root, "R")
			if err == nil {
				t.Fatalf("ruleset() = %+v, want error", got)
			}
			if !strings.Contains(err.Error(), tc.wantReason) {
				t.Errorf("ruleset() error = %q, want it to contain %q", err, tc.wantReason)
			}
			if strings.Contains(err.Error(), "state") || strings.Contains(err.Error(), "absent") {
				t.Errorf("ruleset() error = %q, want it not to mention state or absent", err)
			}
		})
	}
}

// TestDocumentRulesetErrorOrder は、複数の項目が同時に誤っているとき、キーの順に先の項目の誤りを示すことを確かめる。
// 各ケースは、隣り合う二つの項目だけを誤らせる。
//
//trace:checks FR-001
func TestDocumentRulesetErrorOrder(t *testing.T) {
	const invalid = `name: ""
target: tag
enforcement: evaluate
conditions:
  ref_name:
    include: [main]
    exclude: []
bypass_actors:
  - actor_type: Team
    bypass_mode: always
rules:
  - type: creation
`
	cases := []struct {
		name     string
		src      string
		wantPath string
	}{
		{name: "name is shown before target", src: invalid, wantPath: "R.name"},
		{name: "target is shown before enforcement", src: strings.Replace(invalid, `name: ""`, "name: protect", 1), wantPath: "R.target"},
		{name: "enforcement is shown before conditions", src: strings.Replace(strings.Replace(invalid, `name: ""`, "name: protect", 1), "target: tag", "target: branch", 1), wantPath: "R.enforcement"},
		{name: "conditions is shown before bypass_actors", src: strings.Replace(strings.Replace(strings.Replace(invalid, `name: ""`, "name: protect", 1), "target: tag", "target: branch", 1), "enforcement: evaluate", "enforcement: active", 1), wantPath: "R.conditions.ref_name.include[0]"},
		{name: "bypass_actors is shown before rules", src: strings.Replace(strings.Replace(strings.Replace(strings.Replace(invalid, `name: ""`, "name: protect", 1), "target: tag", "target: branch", 1), "enforcement: evaluate", "enforcement: active", 1), "include: [main]", "include: []", 1), wantPath: "R.bypass_actors[0].actor_type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			_, err := doc.ruleset(doc.root, "R")
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
				t.Errorf("ruleset() error = %v, want it to start with %q", err, tc.wantPath+" (")
			}
		})
	}
}

// TestDocumentRulesetConditions は、conditions が ref_name だけを持ち、ref_name が include と exclude の両方を持つことを確かめる。
//
//trace:checks FR-001
func TestDocumentRulesetConditions(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		want       setting.RulesetConditions
		wantPath   string
		wantReason string
	}{
		{
			name: "include and exclude are read into their own fields",
			src:  "ref_name:\n  include: [\"~ALL\"]\n  exclude: [refs/heads/release/*]\n",
			want: setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~ALL"}, Exclude: []string{"refs/heads/release/*"}}},
		},
		{
			name: "empty include and exclude are read as non-nil empty",
			src:  "ref_name:\n  include: []\n  exclude: []\n",
			want: setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{}, Exclude: []string{}}},
		},
		{name: "empty conditions is rejected", src: "{}\n", wantPath: "C", wantReason: "must not be empty"},
		{name: "null conditions is rejected", src: "~\n", wantPath: "C", wantReason: "must not be null or empty"},
		{name: "conditions that is not a mapping is rejected", src: "[a]\n", wantPath: "C", wantReason: "must be a mapping"},
		{name: "unknown key in conditions is rejected", src: "ref_name:\n  include: []\n  exclude: []\ntag_name: {}\n", wantPath: "C", wantReason: `unknown key "tag_name"; expected ref_name`},
		{name: "empty ref_name is rejected", src: "ref_name: {}\n", wantPath: "C.ref_name", wantReason: "must not be empty"},
		{name: "missing include is rejected", src: "ref_name:\n  exclude: []\n", wantPath: "C.ref_name", wantReason: `missing key "include"`},
		{name: "missing exclude is rejected", src: "ref_name:\n  include: []\n", wantPath: "C.ref_name", wantReason: `missing key "exclude"`},
		{name: "unknown key in ref_name is rejected", src: "ref_name:\n  include: []\n  exclude: []\n  other: []\n", wantPath: "C.ref_name", wantReason: `unknown key "other"; expected include, exclude`},
		{name: "invalid include pattern is rejected at include", src: "ref_name:\n  include: [main]\n  exclude: []\n", wantPath: "C.ref_name.include[0]", wantReason: "must be ~DEFAULT_BRANCH"},
		{name: "invalid exclude pattern is rejected at exclude", src: "ref_name:\n  include: []\n  exclude: [main]\n", wantPath: "C.ref_name.exclude[0]", wantReason: "must be ~DEFAULT_BRANCH"},
		{name: "include is shown before exclude", src: "ref_name:\n  include: [a]\n  exclude: [b]\n", wantPath: "C.ref_name.include[0]", wantReason: "must be ~DEFAULT_BRANCH"},
		{
			name: "same pattern in include and exclude is accepted",
			src:  "ref_name:\n  include: [\"~ALL\"]\n  exclude: [\"~ALL\"]\n",
			want: setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~ALL"}, Exclude: []string{"~ALL"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.rulesetConditions(doc.root, "C")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("rulesetConditions() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("rulesetConditions() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("rulesetConditions() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("rulesetConditions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rulesetConditions() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentRefNamePatterns は、パターンの形と、空・空白の誤りと重複を示す順序を確かめる。
//
//trace:checks FR-001
func TestDocumentRefNamePatterns(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		want       []string
		wantReason string
	}{
		{name: "three forms are read in declared order", src: "[\"~DEFAULT_BRANCH\", \"~ALL\", refs/heads/main]\n", want: []string{"~DEFAULT_BRANCH", "~ALL", "refs/heads/main"}},
		{name: "wildcard pattern under refs/heads/ is accepted", src: "[refs/heads/release/*]\n", want: []string{"refs/heads/release/*"}},
		{name: "refs/heads/ alone is accepted", src: "[refs/heads/]\n", want: []string{"refs/heads/"}},
		{name: "branch name without refs/heads/ is rejected", src: "[main]\n", wantReason: `v[0] (line 1, column 2): must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/; got "main"`},
		{name: "tag ref is rejected", src: "[refs/tags/v1]\n", wantReason: `v[0] (line 1, column 2): must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/; got "refs/tags/v1"`},
		{name: "lowercase default branch keyword is rejected", src: "[\"~default_branch\"]\n", wantReason: `must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/; got "~default_branch"`},
		{name: "lowercase all keyword is rejected", src: "[\"~all\"]\n", wantReason: `must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/; got "~all"`},
		{name: "uppercase refs prefix is rejected", src: "[Refs/heads/main]\n", wantReason: "must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/"},
		{name: "refs/heads without trailing slash is rejected", src: "[refs/heads]\n", wantReason: "must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/"},
		{name: "empty string is rejected as empty rather than as a wrong form", src: "[\"\"]\n", wantReason: `v[0] (line 1, column 2): must not be empty or have leading or trailing whitespace; got ""`},
		{name: "wrong form is shown before the duplicate", src: "[a, a]\n", wantReason: `v[0] (line 1, column 2): must be ~DEFAULT_BRANCH`},
		{name: "duplicate is reported at its own item before a wrong form in a later item", src: "[\"~ALL\", \"~ALL\", main]\n", wantReason: `v[1] (line 1, column 10): duplicate pattern "~ALL"; same as v[0]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.refNamePatterns(doc.root, "v")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("refNamePatterns() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("refNamePatterns() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("refNamePatterns() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestDocumentBypassActors は、Bypass の主体の並びを宣言の順のまま読むことと、同じ主体の重複の拒否を確かめる。
//
//trace:checks FR-001
func TestDocumentBypassActors(t *testing.T) {
	role5 := setting.BypassActor{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways}
	user := setting.BypassActor{ActorType: setting.BypassActorUser, ActorID: 583231, BypassMode: setting.BypassModePullRequest}
	deployKey := setting.BypassActor{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeAlways}
	const roleItem = "- actor_type: RepositoryRole\n  actor_id: 5\n  bypass_mode: always\n"
	const roleExemptItem = "- actor_type: RepositoryRole\n  actor_id: 5\n  bypass_mode: exempt\n"
	const userItem = "- actor_type: User\n  actor_id: 583231\n  bypass_mode: pull_request\n"
	const deployKeyItem = "- actor_type: DeployKey\n  bypass_mode: always\n"
	cases := []struct {
		name       string
		src        string
		want       []setting.BypassActor
		wantReason string
	}{
		{name: "actors are read in declared order", src: userItem + roleItem + deployKeyItem, want: []setting.BypassActor{user, role5, deployKey}},
		{name: "empty sequence is read as non-nil empty", src: "[]\n", want: []setting.BypassActor{}},
		{name: "same actor is rejected naming the earlier index", src: userItem + roleItem + roleItem, wantReason: "v[2] (line 7, column 3): duplicate bypass actor RepositoryRole 5; same as v[1]"},
		{name: "same actor with a different mode is rejected", src: roleItem + roleExemptItem, wantReason: "v[1] (line 4, column 3): duplicate bypass actor RepositoryRole 5; same as v[0]"},
		{name: "two deploy keys are rejected", src: deployKeyItem + deployKeyItem, wantReason: "v[1] (line 3, column 3): duplicate bypass actor DeployKey; same as v[0]"},
		{name: "null is rejected", src: "~\n", wantReason: "must not be null"},
		{name: "mapping is rejected as not a sequence", src: "actor_type: User\n", wantReason: "must be a sequence"},
		{name: "invalid actor is rejected at its index", src: roleItem + "- actor_type: Team\n  bypass_mode: always\n", wantReason: `v[1].actor_type (line 4, column 15): must be RepositoryRole, User, Integration, or DeployKey; got "Team"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.bypassActors(doc.root, "v")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("bypassActors() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("bypassActors() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("bypassActors() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestDocumentBypassActor は、主体の種類ごとの actor_id と bypass_mode の組み合わせと、誤りを示す順序を確かめる。
// 拒否のケースは、受理される入力 (RepositoryRole の 5) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentBypassActor(t *testing.T) {
	const base = "actor_type: RepositoryRole\nactor_id: 5\nbypass_mode: always\n"
	actor := func(actorType setting.BypassActorType, id int64, mode setting.BypassMode) setting.BypassActor {
		return setting.BypassActor{ActorType: actorType, ActorID: id, BypassMode: mode}
	}
	cases := []struct {
		name       string
		src        string
		want       setting.BypassActor
		wantPath   string
		wantReason string
	}{
		{name: "repository role 5 is read", src: base, want: actor(setting.BypassActorRepositoryRole, 5, setting.BypassModeAlways)},
		{name: "repository role 2 is read", src: strings.Replace(base, "actor_id: 5", "actor_id: 2", 1), want: actor(setting.BypassActorRepositoryRole, 2, setting.BypassModeAlways)},
		{name: "repository role 4 is read", src: strings.Replace(base, "actor_id: 5", "actor_id: 4", 1), want: actor(setting.BypassActorRepositoryRole, 4, setting.BypassModeAlways)},
		{name: "pull_request mode is read", src: strings.Replace(base, "always", "pull_request", 1), want: actor(setting.BypassActorRepositoryRole, 5, setting.BypassModePullRequest)},
		{name: "exempt mode is read", src: strings.Replace(base, "always", "exempt", 1), want: actor(setting.BypassActorRepositoryRole, 5, setting.BypassModeExempt)},
		{name: "user with a positive id is read", src: "actor_type: User\nactor_id: 583231\nbypass_mode: exempt\n", want: actor(setting.BypassActorUser, 583231, setting.BypassModeExempt)},
		{name: "integration with a positive id is read", src: "actor_type: Integration\nactor_id: 1\nbypass_mode: pull_request\n", want: actor(setting.BypassActorIntegration, 1, setting.BypassModePullRequest)},
		{name: "deploy key without actor_id is read with id 0", src: "actor_type: DeployKey\nbypass_mode: always\n", want: actor(setting.BypassActorDeployKey, 0, setting.BypassModeAlways)},
		{name: "repository role 3 is rejected", src: strings.Replace(base, "actor_id: 5", "actor_id: 3", 1), wantPath: "A.actor_id", wantReason: `must be 2, 4, or 5 when actor_type is "RepositoryRole"; got 3`},
		{name: "repository role 0 is rejected", src: strings.Replace(base, "actor_id: 5", "actor_id: 0", 1), wantPath: "A.actor_id", wantReason: `must be 2, 4, or 5 when actor_type is "RepositoryRole"; got 0`},
		{name: "user with id 0 is rejected", src: "actor_type: User\nactor_id: 0\nbypass_mode: always\n", wantPath: "A.actor_id", wantReason: "must be a positive integer; got 0"},
		{name: "integration with id 0 is rejected", src: "actor_type: Integration\nactor_id: 0\nbypass_mode: always\n", wantPath: "A.actor_id", wantReason: "must be a positive integer; got 0"},
		{name: "deploy key with actor_id is rejected", src: "actor_type: DeployKey\nactor_id: 1\nbypass_mode: always\n", wantPath: "A.actor_id", wantReason: `actor_id is not allowed when actor_type is "DeployKey"`},
		{name: "deploy key with pull_request mode is rejected", src: "actor_type: DeployKey\nbypass_mode: pull_request\n", wantPath: "A.bypass_mode", wantReason: `must be always when actor_type is "DeployKey"; got "pull_request"`},
		{name: "deploy key with exempt mode is rejected", src: "actor_type: DeployKey\nbypass_mode: exempt\n", wantPath: "A.bypass_mode", wantReason: `must be always when actor_type is "DeployKey"; got "exempt"`},
		{name: "repository role without actor_id is rejected", src: strings.Replace(base, "actor_id: 5\n", "", 1), wantPath: "A", wantReason: `actor_type "RepositoryRole" requires actor_id`},
		{name: "user without actor_id is rejected", src: "actor_type: User\nbypass_mode: always\n", wantPath: "A", wantReason: `actor_type "User" requires actor_id`},
		{name: "integration without actor_id is rejected", src: "actor_type: Integration\nbypass_mode: always\n", wantPath: "A", wantReason: `actor_type "Integration" requires actor_id`},
		{name: "team is rejected", src: "actor_type: Team\nactor_id: 1\nbypass_mode: always\n", wantPath: "A.actor_type", wantReason: `must be RepositoryRole, User, Integration, or DeployKey; got "Team"`},
		{name: "organization admin is rejected", src: "actor_type: OrganizationAdmin\nactor_id: 1\nbypass_mode: always\n", wantPath: "A.actor_type", wantReason: `got "OrganizationAdmin"`},
		{name: "lowercase actor_type is rejected", src: strings.Replace(base, "RepositoryRole", "repositoryrole", 1), wantPath: "A.actor_type", wantReason: `got "repositoryrole"`},
		{name: "non-string actor_type is rejected", src: strings.Replace(base, "RepositoryRole", "1", 1), wantPath: "A.actor_type", wantReason: "must be a string"},
		{name: "quoted actor_id is rejected", src: strings.Replace(base, "actor_id: 5", `actor_id: "5"`, 1), wantPath: "A.actor_id", wantReason: `must be an integer; got "5"`},
		{name: "missing bypass_mode is rejected", src: "actor_type: RepositoryRole\nactor_id: 5\n", wantPath: "A", wantReason: `missing key "bypass_mode"`},
		{name: "missing actor_type is rejected", src: "actor_id: 5\nbypass_mode: always\n", wantPath: "A", wantReason: `missing key "actor_type"`},
		{name: "unknown key is rejected", src: base + "name: x\n", wantPath: "A", wantReason: `unknown key "name"; expected actor_type, bypass_mode, actor_id`},
		{name: "unknown bypass_mode is rejected", src: strings.Replace(base, "always", "never", 1), wantPath: "A.bypass_mode", wantReason: `must be always, pull_request, or exempt; got "never"`},
		{name: "non-string bypass_mode is rejected", src: strings.Replace(base, "always", "1", 1), wantPath: "A.bypass_mode", wantReason: "must be a string"},
		{name: "actor_type error is shown before the actor_id combination", src: "actor_type: Team\nbypass_mode: always\n", wantPath: "A.actor_type", wantReason: "must be RepositoryRole"},
		{name: "actor_id combination error is shown before the actor_id value", src: "actor_type: DeployKey\nactor_id: 0\nbypass_mode: always\n", wantPath: "A.actor_id", wantReason: "is not allowed"},
		{name: "actor_id value error is shown before bypass_mode", src: "actor_type: RepositoryRole\nactor_id: 3\nbypass_mode: never\n", wantPath: "A.actor_id", wantReason: "must be 2, 4, or 5"},
		{name: "actor_id combination error is shown before an invalid bypass_mode", src: "actor_type: User\nbypass_mode: never\n", wantPath: "A", wantReason: `actor_type "User" requires actor_id`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.bypassActor(doc.root, "A")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("bypassActor() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("bypassActor() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("bypassActor() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("bypassActor() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("bypassActor() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentRulesetRules は、ルールの並びを宣言の順のまま読むことと、同じ型のルールの重複の拒否を確かめる。
//
//trace:checks FR-001
func TestDocumentRulesetRules(t *testing.T) {
	const deletionItem = "- type: deletion\n"
	const pullRequestItem = "- type: pull_request\n  parameters:\n    required_approving_review_count: 1\n    dismiss_stale_reviews_on_push: false\n    require_code_owner_review: false\n    require_last_push_approval: false\n    required_review_thread_resolution: false\n    allowed_merge_methods: [squash]\n    require_extra_approval_for_unattributed_changes: false\n"
	const statusChecksItem = "- type: required_status_checks\n  parameters:\n    required_status_checks: []\n    strict_required_status_checks_policy: false\n    do_not_enforce_on_create: false\n"
	pullRequest := setting.PullRequestRule(setting.PullRequestParameters{RequiredApprovingReviewCount: 1, AllowedMergeMethods: []setting.AllowedMergeMethod{setting.AllowedMergeMethodSquash}})
	cases := []struct {
		name       string
		src        string
		want       []setting.RulesetRule
		wantReason string
	}{
		{name: "rules are read in declared order", src: pullRequestItem + deletionItem, want: []setting.RulesetRule{pullRequest, setting.DeletionRule()}},
		{name: "empty sequence is read as non-nil empty", src: "[]\n", want: []setting.RulesetRule{}},
		{name: "duplicate deletion is rejected naming the earlier index", src: deletionItem + deletionItem, wantReason: `v[1] (line 2, column 3): duplicate rule type "deletion"; same as v[0]`},
		{name: "duplicate pull_request is rejected even with different parameters", src: pullRequestItem + strings.Replace(pullRequestItem, "count: 1", "count: 2", 1), wantReason: `duplicate rule type "pull_request"; same as v[0]`},
		{name: "duplicate is reported at the later of three rules", src: deletionItem + pullRequestItem + deletionItem, wantReason: `v[2]`},
		{name: "duplicate non_fast_forward is rejected", src: "- type: non_fast_forward\n- type: non_fast_forward\n", wantReason: `duplicate rule type "non_fast_forward"; same as v[0]`},
		{
			name:       "duplicate required_status_checks is rejected even with different parameters",
			src:        statusChecksItem + strings.Replace(statusChecksItem, "policy: false", "policy: true", 1),
			wantReason: `duplicate rule type "required_status_checks"; same as v[0]`,
		},
		{name: "null is rejected", src: "~\n", wantReason: "must not be null"},
		{name: "mapping is rejected as not a sequence", src: "type: deletion\n", wantReason: "must be a sequence"},
		{name: "invalid rule is rejected at its index", src: deletionItem + "- type: creation\n", wantReason: `v[1].type (line 2, column 9): must be pull_request, deletion, required_status_checks, or non_fast_forward; got "creation"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.rulesetRules(doc.root, "v")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("rulesetRules() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("rulesetRules() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rulesetRules() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestDocumentRulesetRule は、ルールの型ごとの parameters の有無と、pull_request の 7 項目のキーと、5 項目の値の判定を確かめる。
// 拒否のケースは、受理される入力 (base) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentRulesetRule(t *testing.T) {
	const base = "type: pull_request\nparameters:\n  required_approving_review_count: 1\n  dismiss_stale_reviews_on_push: false\n  require_code_owner_review: false\n  require_last_push_approval: false\n  required_review_thread_resolution: false\n  allowed_merge_methods: [squash]\n  require_extra_approval_for_unattributed_changes: false\n"
	with := func(old, replacement string) string { return strings.Replace(base, old, replacement, 1) }
	parameters := func(edit func(*setting.PullRequestParameters)) setting.RulesetRule {
		p := setting.PullRequestParameters{RequiredApprovingReviewCount: 1, AllowedMergeMethods: []setting.AllowedMergeMethod{setting.AllowedMergeMethodSquash}}
		edit(&p)
		return setting.PullRequestRule(p)
	}
	cases := []struct {
		name       string
		src        string
		want       setting.RulesetRule
		wantPath   string
		wantReason string
	}{
		{name: "deletion without parameters is read", src: "type: deletion\n", want: setting.DeletionRule()},
		{name: "pull_request with seven parameters is read", src: base, want: parameters(func(*setting.PullRequestParameters) {})},
		{name: "count 0 is read", src: with("count: 1", "count: 0"), want: parameters(func(p *setting.PullRequestParameters) { p.RequiredApprovingReviewCount = 0 })},
		{name: "count 10 is read", src: with("count: 1", "count: 10"), want: parameters(func(p *setting.PullRequestParameters) { p.RequiredApprovingReviewCount = 10 })},
		{name: "dismiss_stale_reviews_on_push true is read into its own field", src: with("dismiss_stale_reviews_on_push: false", "dismiss_stale_reviews_on_push: true"), want: parameters(func(p *setting.PullRequestParameters) { p.DismissStaleReviewsOnPush = true })},
		{name: "require_code_owner_review true is read into its own field", src: with("require_code_owner_review: false", "require_code_owner_review: true"), want: parameters(func(p *setting.PullRequestParameters) { p.RequireCodeOwnerReview = true })},
		{name: "require_last_push_approval true is read into its own field", src: with("require_last_push_approval: false", "require_last_push_approval: true"), want: parameters(func(p *setting.PullRequestParameters) { p.RequireLastPushApproval = true })},
		{name: "required_review_thread_resolution true is read into its own field", src: with("required_review_thread_resolution: false", "required_review_thread_resolution: true"), want: parameters(func(p *setting.PullRequestParameters) { p.RequiredReviewThreadResolution = true })},
		{name: "missing type is rejected", src: "parameters: {}\n", wantPath: "R", wantReason: `missing key "type"`},
		{name: "unknown key is rejected", src: "type: deletion\nname: x\n", wantPath: "R", wantReason: `unknown key "name"; expected type, parameters`},
		{name: "unsupported type is rejected", src: "type: creation\n", wantPath: "R.type", wantReason: `must be pull_request, deletion, required_status_checks, or non_fast_forward; got "creation"`},
		{name: "non-string type is rejected", src: "type: 1\n", wantPath: "R.type", wantReason: "must be a string"},
		{name: "deletion with parameters is rejected", src: "type: deletion\nparameters:\n  a: 1\n", wantPath: "R.parameters", wantReason: `parameters is not allowed when type is "deletion"`},
		{name: "non_fast_forward without parameters is read", src: "type: non_fast_forward\n", want: setting.NonFastForwardRule()},
		{name: "non_fast_forward with parameters is rejected", src: "type: non_fast_forward\nparameters: {}\n", wantPath: "R.parameters", wantReason: `parameters is not allowed when type is "non_fast_forward"`},
		{
			name: "required_status_checks with parameters is read",
			src:  "type: required_status_checks\nparameters:\n  required_status_checks: []\n  strict_required_status_checks_policy: true\n  do_not_enforce_on_create: false\n",
			want: setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{RequiredStatusChecks: []setting.StatusCheck{}, StrictRequiredStatusChecksPolicy: true}),
		},
		{name: "required_status_checks without parameters is rejected", src: "type: required_status_checks\n", wantPath: "R", wantReason: `type "required_status_checks" requires parameters`},
		{name: "invalid required_status_checks parameters are rejected at parameters", src: "type: required_status_checks\nparameters:\n  required_status_checks: []\n", wantPath: "R.parameters", wantReason: `missing key "strict_required_status_checks_policy"`},
		{name: "pull_request without parameters is rejected", src: "type: pull_request\n", wantPath: "R", wantReason: `type "pull_request" requires parameters`},
		{name: "empty parameters is rejected", src: "type: pull_request\nparameters: {}\n", wantPath: "R.parameters", wantReason: "must not be empty"},
		{name: "null parameters is rejected", src: "type: pull_request\nparameters:\n", wantPath: "R.parameters", wantReason: "must not be null or empty"},
		{name: "missing count is rejected", src: with("  required_approving_review_count: 1\n", ""), wantPath: "R.parameters", wantReason: `missing key "required_approving_review_count"`},
		{name: "missing dismiss_stale_reviews_on_push is rejected", src: with("  dismiss_stale_reviews_on_push: false\n", ""), wantPath: "R.parameters", wantReason: `missing key "dismiss_stale_reviews_on_push"`},
		{name: "missing require_code_owner_review is rejected", src: with("  require_code_owner_review: false\n", ""), wantPath: "R.parameters", wantReason: `missing key "require_code_owner_review"`},
		{name: "missing require_last_push_approval is rejected", src: with("  require_last_push_approval: false\n", ""), wantPath: "R.parameters", wantReason: `missing key "require_last_push_approval"`},
		{name: "missing required_review_thread_resolution is rejected", src: with("  required_review_thread_resolution: false\n", ""), wantPath: "R.parameters", wantReason: `missing key "required_review_thread_resolution"`},
		{name: "missing allowed_merge_methods is rejected", src: with("  allowed_merge_methods: [squash]\n", ""), wantPath: "R.parameters", wantReason: `missing key "allowed_merge_methods"`},
		{name: "missing require_extra_approval_for_unattributed_changes is rejected", src: with("  require_extra_approval_for_unattributed_changes: false\n", ""), wantPath: "R.parameters", wantReason: `missing key "require_extra_approval_for_unattributed_changes"`},
		{name: "unknown parameter is rejected", src: base + "  automatic_copilot_code_review_enabled: false\n", wantPath: "R.parameters", wantReason: `unknown key "automatic_copilot_code_review_enabled"; expected required_approving_review_count, dismiss_stale_reviews_on_push, require_code_owner_review, require_last_push_approval, required_review_thread_resolution, allowed_merge_methods, require_extra_approval_for_unattributed_changes`},
		{name: "count 11 is rejected", src: with("count: 1", "count: 11"), wantPath: "R.parameters.required_approving_review_count", wantReason: "must be an integer from 0 to 10; got 11"},
		{name: "quoted count is rejected", src: with("count: 1", `count: "1"`), wantPath: "R.parameters.required_approving_review_count", wantReason: `must be an integer; got "1"`},
		{name: "quoted dismiss_stale_reviews_on_push is rejected", src: with("dismiss_stale_reviews_on_push: false", `dismiss_stale_reviews_on_push: "false"`), wantPath: "R.parameters.dismiss_stale_reviews_on_push", wantReason: "must be true or false"},
		{name: "non-boolean require_code_owner_review is rejected", src: with("require_code_owner_review: false", "require_code_owner_review: yes"), wantPath: "R.parameters.require_code_owner_review", wantReason: "must be true or false"},
		{name: "non-boolean require_last_push_approval is rejected", src: with("require_last_push_approval: false", "require_last_push_approval: 1"), wantPath: "R.parameters.require_last_push_approval", wantReason: "must be true or false"},
		{name: "non-boolean required_review_thread_resolution is rejected", src: with("required_review_thread_resolution: false", "required_review_thread_resolution: 0"), wantPath: "R.parameters.required_review_thread_resolution", wantReason: "must be true or false"},
		{name: "type error is shown before the parameters combination", src: "type: other\nparameters: {}\n", wantPath: "R.type", wantReason: "must be pull_request, deletion, required_status_checks, or non_fast_forward"},
		{name: "count is checked before the booleans", src: strings.Replace(with("count: 1", "count: 11"), "dismiss_stale_reviews_on_push: false", "dismiss_stale_reviews_on_push: maybe", 1), wantPath: "R.parameters.required_approving_review_count", wantReason: "from 0 to 10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.rulesetRule(doc.root, "R")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("rulesetRule() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("rulesetRule() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("rulesetRule() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("rulesetRule() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rulesetRule() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentRequiredStatusChecksParameters は、required_status_checks のパラメータとチェックの判定を確かめる。
// 拒否のケースは、受理される入力 (base) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentRequiredStatusChecksParameters(t *testing.T) {
	const base = "required_status_checks:\n  - context: test\n  - context: lint\n    integration_id: 15368\nstrict_required_status_checks_policy: false\ndo_not_enforce_on_create: false\n"
	with := func(old, replacement string) string { return strings.Replace(base, old, replacement, 1) }
	test, lint := setting.StatusCheck{Context: "test"}, setting.StatusCheck{Context: "lint", IntegrationID: 15368}
	parameters := func(edit func(*setting.RequiredStatusChecksParameters)) setting.RequiredStatusChecksParameters {
		p := setting.RequiredStatusChecksParameters{RequiredStatusChecks: []setting.StatusCheck{test, lint}}
		edit(&p)
		return p
	}
	unchanged := func(*setting.RequiredStatusChecksParameters) {}
	cases := []struct {
		name       string
		src        string
		want       setting.RequiredStatusChecksParameters
		wantPath   string
		wantReason string
	}{
		{name: "checks are read in declared order", src: base, want: parameters(unchanged)},
		{name: "strict policy true is read", src: with("policy: false", "policy: true"), want: parameters(func(p *setting.RequiredStatusChecksParameters) { p.StrictRequiredStatusChecksPolicy = true })},
		{name: "do_not_enforce_on_create true is read", src: with("create: false", "create: true"), want: parameters(func(p *setting.RequiredStatusChecksParameters) { p.DoNotEnforceOnCreate = true })},
		{
			name: "empty checks are read as non-nil empty",
			src:  "required_status_checks: []\nstrict_required_status_checks_policy: false\ndo_not_enforce_on_create: false\n",
			want: setting.RequiredStatusChecksParameters{RequiredStatusChecks: []setting.StatusCheck{}},
		},
		{
			name: "context with surrounding spaces and upper case is read as written",
			src:  with("context: test", `context: " CI/Test "`),
			want: parameters(func(p *setting.RequiredStatusChecksParameters) { p.RequiredStatusChecks[0].Context = " CI/Test " }),
		},
		{
			name: "same context with and without integration_id is read as two checks",
			src:  with("context: lint", "context: test"),
			want: parameters(func(p *setting.RequiredStatusChecksParameters) { p.RequiredStatusChecks[1].Context = "test" }),
		},
		{name: "missing checks are rejected", src: "strict_required_status_checks_policy: false\ndo_not_enforce_on_create: false\n", wantPath: "P", wantReason: `missing key "required_status_checks"`},
		{name: "missing strict policy is rejected", src: with("strict_required_status_checks_policy: false\n", ""), wantPath: "P", wantReason: `missing key "strict_required_status_checks_policy"`},
		{name: "missing do_not_enforce_on_create is rejected", src: with("do_not_enforce_on_create: false\n", ""), wantPath: "P", wantReason: `missing key "do_not_enforce_on_create"`},
		{name: "unknown parameter is rejected", src: base + "other: 1\n", wantPath: "P", wantReason: `unknown key "other"`},
		{name: "null checks are rejected", src: with("required_status_checks:\n  - context: test\n  - context: lint\n    integration_id: 15368\n", "required_status_checks:\n"), wantPath: "P.required_status_checks", wantReason: "must not be null"},
		{name: "missing context is rejected", src: with("  - context: test\n", "  - integration_id: 1\n"), wantPath: "P.required_status_checks[0]", wantReason: `missing key "context"`},
		{name: "unknown check key is rejected", src: with("  - context: test\n", "  - context: test\n    app: x\n"), wantPath: "P.required_status_checks[0]", wantReason: `unknown key "app"`},
		{name: "empty context is rejected", src: with("context: test", `context: ""`), wantPath: "P.required_status_checks[0].context", wantReason: "must not be empty"},
		{name: "non-string context is rejected", src: with("context: test", "context: 1"), wantPath: "P.required_status_checks[0].context", wantReason: "must be a string"},
		{name: "zero integration_id is rejected", src: with("integration_id: 15368", "integration_id: 0"), wantPath: "P.required_status_checks[1].integration_id", wantReason: "must be a positive integer; got 0"},
		{name: "quoted integration_id is rejected", src: with("integration_id: 15368", `integration_id: "15368"`), wantPath: "P.required_status_checks[1].integration_id", wantReason: "must be an integer"},
		{name: "duplicate check without integration_id is rejected", src: with("context: lint\n    integration_id: 15368", "context: test"), wantPath: "P.required_status_checks[1]", wantReason: `duplicate status check "test"; same as P.required_status_checks[0]`},
		{
			name:       "duplicate check with the same integration_id is rejected",
			src:        with("strict_required", "  - context: lint\n    integration_id: 15368\nstrict_required"),
			wantPath:   "P.required_status_checks[2]",
			wantReason: `duplicate status check "lint" with integration_id 15368; same as P.required_status_checks[1]`,
		},
		{name: "non-boolean strict policy is rejected", src: with("policy: false", "policy: yes"), wantPath: "P.strict_required_status_checks_policy", wantReason: "must be true or false"},
		{name: "non-boolean do_not_enforce_on_create is rejected", src: with("create: false", "create: 1"), wantPath: "P.do_not_enforce_on_create", wantReason: "must be true or false"},
		{name: "checks are checked before the strict policy", src: strings.Replace(with("policy: false", "policy: yes"), "context: test", `context: ""`, 1), wantPath: "P.required_status_checks[0].context", wantReason: "must not be empty"},
		{name: "strict policy is checked before do_not_enforce_on_create", src: strings.Replace(with("policy: false", "policy: yes"), "create: false", "create: 1", 1), wantPath: "P.strict_required_status_checks_policy", wantReason: "must be true or false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.requiredStatusChecksParameters(doc.root, "P")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("requiredStatusChecksParameters() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("requiredStatusChecksParameters() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("requiredStatusChecksParameters() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("requiredStatusChecksParameters() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("requiredStatusChecksParameters() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentPullRequestParameters は、pull_request の allowed_merge_methods と require_extra_approval_for_unattributed_changes の値の判定を確かめる。
// キーの判定とほかの5項目の値の判定は TestDocumentRulesetRule が確かめる。拒否のケースは、受理される入力 (declared) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentPullRequestParameters(t *testing.T) {
	const declared = "required_approving_review_count: 1\ndismiss_stale_reviews_on_push: false\nrequire_code_owner_review: false\nrequire_last_push_approval: false\nrequired_review_thread_resolution: false\n" +
		"allowed_merge_methods: [squash, merge]\nrequire_extra_approval_for_unattributed_changes: false\n"
	with := func(old, replacement string) string { return strings.Replace(declared, old, replacement, 1) }
	squash, merge := setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodMerge
	cases := []struct {
		name       string
		src        string
		want       setting.PullRequestParameters
		wantPath   string
		wantReason string
	}{
		{
			name: "allowed_merge_methods are read in the declared order",
			src:  declared,
			want: setting.PullRequestParameters{RequiredApprovingReviewCount: 1, AllowedMergeMethods: []setting.AllowedMergeMethod{squash, merge}},
		},
		{
			name: "require_extra_approval_for_unattributed_changes true is read",
			src:  with("changes: false", "changes: true"),
			want: setting.PullRequestParameters{RequiredApprovingReviewCount: 1, AllowedMergeMethods: []setting.AllowedMergeMethod{squash, merge}, RequireExtraApprovalForUnattributedChanges: true},
		},
		{name: "empty allowed_merge_methods is rejected", src: with("[squash, merge]", "[]"), wantPath: "P.allowed_merge_methods", wantReason: "must not be empty"},
		{name: "non-string merge method is rejected", src: with("[squash, merge]", "[squash, 1]"), wantPath: "P.allowed_merge_methods[1]", wantReason: "must be a string"},
		{name: "undefined merge method is rejected", src: with("[squash, merge]", "[squash, fast_forward]"), wantPath: "P.allowed_merge_methods[1]", wantReason: `must be merge, squash, or rebase; got "fast_forward"`},
		{name: "duplicate merge method is rejected", src: with("[squash, merge]", "[squash, squash]"), wantPath: "P.allowed_merge_methods[1]", wantReason: `duplicate merge method "squash"; same as P.allowed_merge_methods[0]`},
		{name: "non-boolean require_extra_approval_for_unattributed_changes is rejected", src: with("changes: false", "changes: no"), wantPath: "P.require_extra_approval_for_unattributed_changes", wantReason: "must be true or false"},
		{name: "dismissal_restriction is rejected as an unknown key", src: declared + "dismissal_restriction: {}\n", wantPath: "P", wantReason: `unknown key "dismissal_restriction"`},
		{name: "required_reviewers is rejected as an unknown key", src: declared + "required_reviewers: []\n", wantPath: "P", wantReason: `unknown key "required_reviewers"`},
		{name: "booleans are checked before allowed_merge_methods", src: strings.Replace(with("[squash, merge]", "[]"), "resolution: false", "resolution: 0", 1), wantPath: "P.required_review_thread_resolution", wantReason: "must be true or false"},
		{name: "allowed_merge_methods is checked before require_extra_approval_for_unattributed_changes", src: strings.Replace(with("[squash, merge]", "[]"), "changes: false", "changes: no", 1), wantPath: "P.allowed_merge_methods", wantReason: "must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.pullRequestParameters(doc.root, "P")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("pullRequestParameters() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("pullRequestParameters() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("pullRequestParameters() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("pullRequestParameters() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("pullRequestParameters() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDocumentTypedRulesetRule_UndefinedTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("typedRulesetRule() did not panic for an undefined rule type")
		}
	}()
	doc := decodeSnippet(t, "{}\n")
	_, _ = doc.typedRulesetRule(setting.RulesetRuleType("creation"), nil, "R.parameters")
}
