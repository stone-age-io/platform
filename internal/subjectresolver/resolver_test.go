package subjectresolver

import "testing"

func TestJoin(t *testing.T) {
	cases := []struct {
		prefix, suffix, want string
	}{
		{"{location}.camera.{thing}", "motion", "{location}.camera.{thing}.motion"},
		{"{location}.camera.{thing}", "", "{location}.camera.{thing}"},
		{"", "heartbeat", DefaultPrefix + ".heartbeat"},
		{"", "", DefaultPrefix},
	}
	for _, c := range cases {
		if got := Join(c.prefix, c.suffix); got != c.want {
			t.Errorf("Join(%q,%q) = %q, want %q", c.prefix, c.suffix, got, c.want)
		}
	}
}

func TestResolveThing(t *testing.T) {
	ctx := ThingContext{
		Org:           "acme",
		Location:      "warehouse-a",
		Thing:         "cam-042",
		ThingTypeCode: "ip_camera",
	}
	got := ResolveThing("{location}.camera.{thing}.motion", ctx)
	if got != "warehouse-a.camera.cam-042.motion" {
		t.Errorf("unexpected resolve: %q", got)
	}
}

func TestResolveThingDefaultPrefix(t *testing.T) {
	ctx := ThingContext{Location: "warehouse-a", Thing: "cam-042", ThingTypeCode: "ip_camera"}
	got := ResolveThing(Join("", "heartbeat"), ctx)
	if got != "warehouse-a.ip_camera.cam-042.heartbeat" {
		t.Errorf("unexpected default-prefix resolve: %q", got)
	}
}

func TestResolveThingLeavesUnsetLiteral(t *testing.T) {
	ctx := ThingContext{Thing: "cam-042"}
	got := ResolveThing("{location}.camera.{thing}.motion", ctx)
	if got != "{location}.camera.cam-042.motion" {
		t.Errorf("unset vars should stay literal, got %q", got)
	}
}

func TestResolveRolePatternWildcards(t *testing.T) {
	got := ResolveRolePattern("{location}.camera.{thing}.heartbeat", RolePatternContext{})
	if got != "*.camera.*.heartbeat" {
		t.Errorf("unexpected role pattern: %q", got)
	}
}

func TestResolveRolePatternScopedLocation(t *testing.T) {
	ctx := RolePatternContext{Location: "warehouse-a"}
	got := ResolveRolePattern("{location}.camera.{thing}.heartbeat", ctx)
	if got != "warehouse-a.camera.*.heartbeat" {
		t.Errorf("unexpected location-scoped role pattern: %q", got)
	}
}
