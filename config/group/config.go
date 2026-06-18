package group

import (
	"context"
	"github.com/crossplane-contrib/provider-keycloak/config/lookup"
	"github.com/crossplane/upjet/v2/pkg/config"
	"github.com/keycloak/terraform-provider-keycloak/keycloak"
	"strings"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("keycloak_group", func(r *config.Resource) {
		// We need to override the default group that upjet generated for
		r.ShortGroup = "group"

		r.References["parent_id"] = config.Reference{
			TerraformName: "keycloak_group",
		}
	})
	p.AddResourceConfigurator("keycloak_group_memberships", func(r *config.Resource) {
		// We need to override the default group that upjet generated for
		r.ShortGroup = "group"
		r.References["group_id"] = config.Reference{
			TerraformName: "keycloak_group",
		}

	})
	p.AddResourceConfigurator("keycloak_group_roles", func(r *config.Resource) {
		// We need to override the default group that upjet generated for
		r.ShortGroup = "group"
		r.References["group_id"] = config.Reference{
			TerraformName: "keycloak_group",
		}
	})
	p.AddResourceConfigurator("keycloak_group_permissions", func(r *config.Resource) {
		// We need to override the default group that upjet generated for
		r.ShortGroup = "group"
		r.References["group_id"] = config.Reference{
			TerraformName: "keycloak_group",
		}
	})
}

var groupIdentifyingPropertiesLookup = lookup.IdentifyingPropertiesLookupConfig{
	RequiredParameters:           []string{"realm_id", "name"},
	GetIDByExternalName:          getGroupIDByExternalName,
	GetIDByIdentifyingProperties: getGroupIDByIdentifyingProperties,
}

// GroupIdentifierFromIdentifyingProperties is used to find the existing resource by it´s identifying properties
var GroupIdentifierFromIdentifyingProperties = lookup.BuildIdentifyingPropertiesLookup(groupIdentifyingPropertiesLookup)

func getGroupIDByExternalName(ctx context.Context, id string, parameters map[string]any, kcClient *keycloak.KeycloakClient) (string, error) {
	found, err := kcClient.GetGroup(ctx, parameters["realm_id"].(string), id)
	if err != nil {
		return "", err
	}
	return found.Id, nil
}

func getGroupIDByIdentifyingProperties(ctx context.Context, parameters map[string]any, kcClient *keycloak.KeycloakClient) (string, error) {
	realmId := parameters["realm_id"].(string)
	name := parameters["name"].(string)

	// A "name" containing "/" (e.g. "/synergy/admins") is treated as a full group
	// path and resolved by walking the hierarchy. This disambiguates nested groups
	// that GetGroupByName (name-only) cannot. A plain name keeps the old behaviour.
	if strings.Contains(name, "/") {
		return getGroupIDByPath(ctx, realmId, name, kcClient)
	}

	found, err := kcClient.GetGroupByName(ctx, realmId, name)
	if err != nil {
		if strings.Contains(err.Error(), "no group with name") {
			return "", nil
		}

		return "", err
	}

	return found.Id, nil
}

// getGroupIDByPath resolves a group's ID from its full path (e.g. "/synergy/admins").
// It searches on the leaf segment (the only thing the Keycloak group search accepts)
// and then matches the exact Path in the returned hierarchy. Returns "" if not found.
func getGroupIDByPath(ctx context.Context, realmId, path string, kcClient *keycloak.KeycloakClient) (string, error) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", nil
	}
	wantPath := "/" + trimmed
	segments := strings.Split(trimmed, "/")
	leaf := segments[len(segments)-1]

	groups, err := kcClient.ListGroupsWithName(ctx, realmId, leaf)
	if err != nil {
		return "", err
	}
	if found := findGroupByPathDFS(wantPath, groups); found != nil {
		return found.Id, nil
	}
	return "", nil
}

// findGroupByPathDFS walks the group tree (depth-first) looking for an exact Path match.
func findGroupByPathDFS(path string, groups []*keycloak.Group) *keycloak.Group {
	for _, group := range groups {
		if group.Path == path {
			return group
		}
		if found := findGroupByPathDFS(path, group.SubGroups); found != nil {
			return found
		}
	}
	return nil
}
