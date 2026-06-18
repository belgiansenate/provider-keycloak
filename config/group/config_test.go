package group

import (
	"testing"

	"github.com/keycloak/terraform-provider-keycloak/keycloak"
)

func TestFindGroupByPathDFS(t *testing.T) {
	tree := []*keycloak.Group{
		{Id: "top1", Name: "synergy", Path: "/synergy", SubGroups: []*keycloak.Group{
			{Id: "admins", Name: "admins", Path: "/synergy/admins"},
			{Id: "editors", Name: "editors", Path: "/synergy/editors", SubGroups: []*keycloak.Group{
				{Id: "leads", Name: "leads", Path: "/synergy/editors/leads"},
			}},
		}},
		{Id: "top2", Name: "other", Path: "/other"},
	}

	cases := []struct {
		path   string
		wantID string
	}{
		{"/synergy", "top1"},
		{"/synergy/admins", "admins"},
		{"/synergy/editors", "editors"},
		{"/synergy/editors/leads", "leads"},
		{"/other", "top2"},
		{"/synergy/missing", ""},
		{"/nope", ""},
	}

	for _, c := range cases {
		got := findGroupByPathDFS(c.path, tree)
		gotID := ""
		if got != nil {
			gotID = got.Id
		}
		if gotID != c.wantID {
			t.Errorf("findGroupByPathDFS(%q) = %q, want %q", c.path, gotID, c.wantID)
		}
	}
}
