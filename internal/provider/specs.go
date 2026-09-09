package provider

type ResourceSpec struct {
	TypeName            string
	Version             string
	SchemaRef           string
	Description         string
	Collection          string
	IDField             string
	CreateMethod        string
	ReadMethod          string
	UpdateMethod        string
	DeleteMethod        string
	CreateResponseField string
	RequiredFields      []string
	ComputedFields      []string
	AdoptExisting       bool
	BuiltInVariable     bool
}

type DataSourceSpec struct {
	TypeName  string
	Version   string
	Method    string
	ResultRef string
}

type ActionSpec struct {
	TypeName string
	Version  string
	Method   string
}

func resourceSpecs() []ResourceSpec {
	return []ResourceSpec{
		{TypeName: "gtm_account_settings", Version: "v2", SchemaRef: "Account", Description: "Manages settings for an existing Google Tag Manager Account through API v2. The Tag Manager API does not support creating Accounts, and removing this resource does not delete the Account.", ReadMethod: "accounts.get", UpdateMethod: "accounts.update", AdoptExisting: true},
		{TypeName: "gtm_user_permission", Version: "v2", SchemaRef: "UserPermission", Collection: "user_permissions", CreateMethod: "accounts.user_permissions.create", ReadMethod: "accounts.user_permissions.get", UpdateMethod: "accounts.user_permissions.update", DeleteMethod: "accounts.user_permissions.delete", RequiredFields: []string{"emailAddress"}},
		{TypeName: "gtm_container", Version: "v2", SchemaRef: "Container", Description: "Creates and manages a Google Tag Manager Container under an existing Account through API v2.", Collection: "containers", IDField: "containerId", CreateMethod: "accounts.containers.create", ReadMethod: "accounts.containers.get", UpdateMethod: "accounts.containers.update", DeleteMethod: "accounts.containers.delete", RequiredFields: []string{"name", "usageContext"}, ComputedFields: []string{"publicId", "tagIds", "features"}},
		{TypeName: "gtm_environment", Version: "v2", SchemaRef: "Environment", Collection: "environments", IDField: "environmentId", CreateMethod: "accounts.containers.environments.create", ReadMethod: "accounts.containers.environments.get", UpdateMethod: "accounts.containers.environments.update", DeleteMethod: "accounts.containers.environments.delete", RequiredFields: []string{"name", "type"}, ComputedFields: []string{"authorizationCode", "authorizationTimestamp"}},
		{TypeName: "gtm_workspace", Version: "v2", SchemaRef: "Workspace", Collection: "workspaces", IDField: "workspaceId", CreateMethod: "accounts.containers.workspaces.create", ReadMethod: "accounts.containers.workspaces.get", UpdateMethod: "accounts.containers.workspaces.update", DeleteMethod: "accounts.containers.workspaces.delete", RequiredFields: []string{"name"}},
		{TypeName: "gtm_built_in_variable", Version: "v2", SchemaRef: "BuiltInVariable", Collection: "built_in_variables", CreateMethod: "accounts.containers.workspaces.built_in_variables.create", ReadMethod: "accounts.containers.workspaces.built_in_variables.list", DeleteMethod: "accounts.containers.workspaces.built_in_variables.delete", RequiredFields: []string{"type"}, BuiltInVariable: true},
		{TypeName: "gtm_client", Version: "v2", SchemaRef: "Client", Collection: "clients", IDField: "clientId", CreateMethod: "accounts.containers.workspaces.clients.create", ReadMethod: "accounts.containers.workspaces.clients.get", UpdateMethod: "accounts.containers.workspaces.clients.update", DeleteMethod: "accounts.containers.workspaces.clients.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_folder", Version: "v2", SchemaRef: "Folder", Collection: "folders", IDField: "folderId", CreateMethod: "accounts.containers.workspaces.folders.create", ReadMethod: "accounts.containers.workspaces.folders.get", UpdateMethod: "accounts.containers.workspaces.folders.update", DeleteMethod: "accounts.containers.workspaces.folders.delete", RequiredFields: []string{"name"}},
		{TypeName: "gtm_google_tag_config", Version: "v2", SchemaRef: "GtagConfig", Collection: "gtag_config", IDField: "gtagConfigId", CreateMethod: "accounts.containers.workspaces.gtag_config.create", ReadMethod: "accounts.containers.workspaces.gtag_config.get", UpdateMethod: "accounts.containers.workspaces.gtag_config.update", DeleteMethod: "accounts.containers.workspaces.gtag_config.delete", RequiredFields: []string{"type"}},
		{TypeName: "gtm_tag", Version: "v2", SchemaRef: "Tag", Collection: "tags", IDField: "tagId", CreateMethod: "accounts.containers.workspaces.tags.create", ReadMethod: "accounts.containers.workspaces.tags.get", UpdateMethod: "accounts.containers.workspaces.tags.update", DeleteMethod: "accounts.containers.workspaces.tags.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_custom_template", Version: "v2", SchemaRef: "CustomTemplate", Collection: "templates", IDField: "templateId", CreateMethod: "accounts.containers.workspaces.templates.create", ReadMethod: "accounts.containers.workspaces.templates.get", UpdateMethod: "accounts.containers.workspaces.templates.update", DeleteMethod: "accounts.containers.workspaces.templates.delete", RequiredFields: []string{"name", "templateData"}},
		{TypeName: "gtm_transformation", Version: "v2", SchemaRef: "Transformation", Collection: "transformations", IDField: "transformationId", CreateMethod: "accounts.containers.workspaces.transformations.create", ReadMethod: "accounts.containers.workspaces.transformations.get", UpdateMethod: "accounts.containers.workspaces.transformations.update", DeleteMethod: "accounts.containers.workspaces.transformations.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_trigger", Version: "v2", SchemaRef: "Trigger", Collection: "triggers", IDField: "triggerId", CreateMethod: "accounts.containers.workspaces.triggers.create", ReadMethod: "accounts.containers.workspaces.triggers.get", UpdateMethod: "accounts.containers.workspaces.triggers.update", DeleteMethod: "accounts.containers.workspaces.triggers.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_variable", Version: "v2", SchemaRef: "Variable", Collection: "variables", IDField: "variableId", CreateMethod: "accounts.containers.workspaces.variables.create", ReadMethod: "accounts.containers.workspaces.variables.get", UpdateMethod: "accounts.containers.workspaces.variables.update", DeleteMethod: "accounts.containers.workspaces.variables.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_zone", Version: "v2", SchemaRef: "Zone", Collection: "zones", IDField: "zoneId", CreateMethod: "accounts.containers.workspaces.zones.create", ReadMethod: "accounts.containers.workspaces.zones.get", UpdateMethod: "accounts.containers.workspaces.zones.update", DeleteMethod: "accounts.containers.workspaces.zones.delete", RequiredFields: []string{"name"}},

		{TypeName: "gtm_v1_account_settings", Version: "v1", SchemaRef: "Account", Description: "Manages settings for an existing Google Tag Manager Account through API v1. The Tag Manager API does not support creating Accounts, and removing this resource does not delete the Account.", ReadMethod: "accounts.get", UpdateMethod: "accounts.update", AdoptExisting: true},
		{TypeName: "gtm_v1_user_permission", Version: "v1", SchemaRef: "UserAccess", Collection: "permissions", IDField: "permissionId", CreateMethod: "accounts.permissions.create", ReadMethod: "accounts.permissions.get", UpdateMethod: "accounts.permissions.update", DeleteMethod: "accounts.permissions.delete", RequiredFields: []string{"emailAddress"}},
		{TypeName: "gtm_v1_container", Version: "v1", SchemaRef: "Container", Description: "Creates and manages a Google Tag Manager Container under an existing Account through API v1.", Collection: "containers", IDField: "containerId", CreateMethod: "accounts.containers.create", ReadMethod: "accounts.containers.get", UpdateMethod: "accounts.containers.update", DeleteMethod: "accounts.containers.delete", RequiredFields: []string{"name", "usageContext"}, ComputedFields: []string{"publicId", "tagIds"}},
		{TypeName: "gtm_v1_environment", Version: "v1", SchemaRef: "Environment", Collection: "environments", IDField: "environmentId", CreateMethod: "accounts.containers.environments.create", ReadMethod: "accounts.containers.environments.get", UpdateMethod: "accounts.containers.environments.update", DeleteMethod: "accounts.containers.environments.delete", RequiredFields: []string{"name", "type"}, ComputedFields: []string{"authorizationCode", "authorizationTimestamp"}},
		{TypeName: "gtm_v1_folder", Version: "v1", SchemaRef: "Folder", Collection: "folders", IDField: "folderId", CreateMethod: "accounts.containers.folders.create", ReadMethod: "accounts.containers.folders.get", UpdateMethod: "accounts.containers.folders.update", DeleteMethod: "accounts.containers.folders.delete", RequiredFields: []string{"name"}},
		{TypeName: "gtm_v1_tag", Version: "v1", SchemaRef: "Tag", Collection: "tags", IDField: "tagId", CreateMethod: "accounts.containers.tags.create", ReadMethod: "accounts.containers.tags.get", UpdateMethod: "accounts.containers.tags.update", DeleteMethod: "accounts.containers.tags.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_v1_trigger", Version: "v1", SchemaRef: "Trigger", Collection: "triggers", IDField: "triggerId", CreateMethod: "accounts.containers.triggers.create", ReadMethod: "accounts.containers.triggers.get", UpdateMethod: "accounts.containers.triggers.update", DeleteMethod: "accounts.containers.triggers.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_v1_variable", Version: "v1", SchemaRef: "Variable", Collection: "variables", IDField: "variableId", CreateMethod: "accounts.containers.variables.create", ReadMethod: "accounts.containers.variables.get", UpdateMethod: "accounts.containers.variables.update", DeleteMethod: "accounts.containers.variables.delete", RequiredFields: []string{"name", "type"}},
		{TypeName: "gtm_v1_container_version", Version: "v1", SchemaRef: "ContainerVersion", Collection: "versions", IDField: "containerVersionId", CreateMethod: "accounts.containers.versions.create", ReadMethod: "accounts.containers.versions.get", UpdateMethod: "accounts.containers.versions.update", DeleteMethod: "accounts.containers.versions.delete", CreateResponseField: "containerVersion"},
	}
}

func dataSourceSpecs() []DataSourceSpec {
	return []DataSourceSpec{
		{TypeName: "gtm_account", Version: "v2", Method: "accounts.get"},
		{TypeName: "gtm_accounts", Version: "v2", Method: "accounts.list"},
		{TypeName: "gtm_user_permission", Version: "v2", Method: "accounts.user_permissions.get"},
		{TypeName: "gtm_user_permissions", Version: "v2", Method: "accounts.user_permissions.list"},
		{TypeName: "gtm_container", Version: "v2", Method: "accounts.containers.get"},
		{TypeName: "gtm_containers", Version: "v2", Method: "accounts.containers.list"},
		{TypeName: "gtm_container_lookup", Version: "v2", Method: "accounts.containers.lookup"},
		{TypeName: "gtm_container_snippet", Version: "v2", Method: "accounts.containers.snippet"},
		{TypeName: "gtm_destination", Version: "v2", Method: "accounts.containers.destinations.get"},
		{TypeName: "gtm_destinations", Version: "v2", Method: "accounts.containers.destinations.list"},
		{TypeName: "gtm_environment", Version: "v2", Method: "accounts.containers.environments.get"},
		{TypeName: "gtm_environments", Version: "v2", Method: "accounts.containers.environments.list"},
		{TypeName: "gtm_container_version", Version: "v2", Method: "accounts.containers.versions.get"},
		{TypeName: "gtm_live_container_version", Version: "v2", Method: "accounts.containers.versions.live"},
		{TypeName: "gtm_container_version_headers", Version: "v2", Method: "accounts.containers.version_headers.list"},
		{TypeName: "gtm_latest_container_version_header", Version: "v2", Method: "accounts.containers.version_headers.latest"},
		{TypeName: "gtm_workspace", Version: "v2", Method: "accounts.containers.workspaces.get"},
		{TypeName: "gtm_workspaces", Version: "v2", Method: "accounts.containers.workspaces.list"},
		{TypeName: "gtm_workspace_status", Version: "v2", Method: "accounts.containers.workspaces.getStatus"},
		{TypeName: "gtm_built_in_variables", Version: "v2", Method: "accounts.containers.workspaces.built_in_variables.list"},
		{TypeName: "gtm_client", Version: "v2", Method: "accounts.containers.workspaces.clients.get"},
		{TypeName: "gtm_clients", Version: "v2", Method: "accounts.containers.workspaces.clients.list"},
		{TypeName: "gtm_folder", Version: "v2", Method: "accounts.containers.workspaces.folders.get"},
		{TypeName: "gtm_folders", Version: "v2", Method: "accounts.containers.workspaces.folders.list"},
		{TypeName: "gtm_folder_entities", Version: "v2", Method: "accounts.containers.workspaces.folders.entities"},
		{TypeName: "gtm_google_tag_config", Version: "v2", Method: "accounts.containers.workspaces.gtag_config.get"},
		{TypeName: "gtm_google_tag_configs", Version: "v2", Method: "accounts.containers.workspaces.gtag_config.list"},
		{TypeName: "gtm_tag", Version: "v2", Method: "accounts.containers.workspaces.tags.get"},
		{TypeName: "gtm_tags", Version: "v2", Method: "accounts.containers.workspaces.tags.list"},
		{TypeName: "gtm_custom_template", Version: "v2", Method: "accounts.containers.workspaces.templates.get"},
		{TypeName: "gtm_custom_templates", Version: "v2", Method: "accounts.containers.workspaces.templates.list"},
		{TypeName: "gtm_transformation", Version: "v2", Method: "accounts.containers.workspaces.transformations.get"},
		{TypeName: "gtm_transformations", Version: "v2", Method: "accounts.containers.workspaces.transformations.list"},
		{TypeName: "gtm_trigger", Version: "v2", Method: "accounts.containers.workspaces.triggers.get"},
		{TypeName: "gtm_triggers", Version: "v2", Method: "accounts.containers.workspaces.triggers.list"},
		{TypeName: "gtm_variable", Version: "v2", Method: "accounts.containers.workspaces.variables.get"},
		{TypeName: "gtm_variables", Version: "v2", Method: "accounts.containers.workspaces.variables.list"},
		{TypeName: "gtm_zone", Version: "v2", Method: "accounts.containers.workspaces.zones.get"},
		{TypeName: "gtm_zones", Version: "v2", Method: "accounts.containers.workspaces.zones.list"},

		{TypeName: "gtm_v1_account", Version: "v1", Method: "accounts.get"},
		{TypeName: "gtm_v1_accounts", Version: "v1", Method: "accounts.list"},
		{TypeName: "gtm_v1_user_permission", Version: "v1", Method: "accounts.permissions.get"},
		{TypeName: "gtm_v1_user_permissions", Version: "v1", Method: "accounts.permissions.list"},
		{TypeName: "gtm_v1_container", Version: "v1", Method: "accounts.containers.get"},
		{TypeName: "gtm_v1_containers", Version: "v1", Method: "accounts.containers.list"},
		{TypeName: "gtm_v1_environment", Version: "v1", Method: "accounts.containers.environments.get"},
		{TypeName: "gtm_v1_environments", Version: "v1", Method: "accounts.containers.environments.list"},
		{TypeName: "gtm_v1_folder", Version: "v1", Method: "accounts.containers.folders.get"},
		{TypeName: "gtm_v1_folders", Version: "v1", Method: "accounts.containers.folders.list"},
		{TypeName: "gtm_v1_folder_entities", Version: "v1", Method: "accounts.containers.folders.entities.list"},
		{TypeName: "gtm_v1_tag", Version: "v1", Method: "accounts.containers.tags.get"},
		{TypeName: "gtm_v1_tags", Version: "v1", Method: "accounts.containers.tags.list"},
		{TypeName: "gtm_v1_trigger", Version: "v1", Method: "accounts.containers.triggers.get"},
		{TypeName: "gtm_v1_triggers", Version: "v1", Method: "accounts.containers.triggers.list"},
		{TypeName: "gtm_v1_variable", Version: "v1", Method: "accounts.containers.variables.get"},
		{TypeName: "gtm_v1_variables", Version: "v1", Method: "accounts.containers.variables.list"},
		{TypeName: "gtm_v1_container_version", Version: "v1", Method: "accounts.containers.versions.get"},
		{TypeName: "gtm_v1_container_versions", Version: "v1", Method: "accounts.containers.versions.list"},
	}
}

func actionSpecs() []ActionSpec {
	return []ActionSpec{
		{TypeName: "gtm_combine_containers", Version: "v2", Method: "accounts.containers.combine"},
		{TypeName: "gtm_move_tag_id", Version: "v2", Method: "accounts.containers.move_tag_id"},
		{TypeName: "gtm_link_destination", Version: "v2", Method: "accounts.containers.destinations.link"},
		{TypeName: "gtm_reauthorize_environment", Version: "v2", Method: "accounts.containers.environments.reauthorize"},
		{TypeName: "gtm_update_container_version", Version: "v2", Method: "accounts.containers.versions.update"},
		{TypeName: "gtm_delete_container_version", Version: "v2", Method: "accounts.containers.versions.delete"},
		{TypeName: "gtm_publish_container_version", Version: "v2", Method: "accounts.containers.versions.publish"},
		{TypeName: "gtm_set_latest_container_version", Version: "v2", Method: "accounts.containers.versions.set_latest"},
		{TypeName: "gtm_undelete_container_version", Version: "v2", Method: "accounts.containers.versions.undelete"},
		{TypeName: "gtm_create_container_version", Version: "v2", Method: "accounts.containers.workspaces.create_version"},
		{TypeName: "gtm_bulk_update_workspace", Version: "v2", Method: "accounts.containers.workspaces.bulk_update"},
		{TypeName: "gtm_quick_preview_workspace", Version: "v2", Method: "accounts.containers.workspaces.quick_preview"},
		{TypeName: "gtm_resolve_workspace_conflict", Version: "v2", Method: "accounts.containers.workspaces.resolve_conflict"},
		{TypeName: "gtm_sync_workspace", Version: "v2", Method: "accounts.containers.workspaces.sync"},
		{TypeName: "gtm_revert_built_in_variable", Version: "v2", Method: "accounts.containers.workspaces.built_in_variables.revert"},
		{TypeName: "gtm_revert_client", Version: "v2", Method: "accounts.containers.workspaces.clients.revert"},
		{TypeName: "gtm_revert_folder", Version: "v2", Method: "accounts.containers.workspaces.folders.revert"},
		{TypeName: "gtm_move_entities_to_folder", Version: "v2", Method: "accounts.containers.workspaces.folders.move_entities_to_folder"},
		{TypeName: "gtm_revert_tag", Version: "v2", Method: "accounts.containers.workspaces.tags.revert"},
		{TypeName: "gtm_import_custom_template_from_gallery", Version: "v2", Method: "accounts.containers.workspaces.templates.import_from_gallery"},
		{TypeName: "gtm_revert_custom_template", Version: "v2", Method: "accounts.containers.workspaces.templates.revert"},
		{TypeName: "gtm_revert_transformation", Version: "v2", Method: "accounts.containers.workspaces.transformations.revert"},
		{TypeName: "gtm_revert_trigger", Version: "v2", Method: "accounts.containers.workspaces.triggers.revert"},
		{TypeName: "gtm_revert_variable", Version: "v2", Method: "accounts.containers.workspaces.variables.revert"},
		{TypeName: "gtm_revert_zone", Version: "v2", Method: "accounts.containers.workspaces.zones.revert"},

		{TypeName: "gtm_v1_reauthorize_environment", Version: "v1", Method: "accounts.containers.reauthorize_environments.update"},
		{TypeName: "gtm_v1_move_entities_to_folder", Version: "v1", Method: "accounts.containers.move_folders.update"},
		{TypeName: "gtm_v1_restore_container_version", Version: "v1", Method: "accounts.containers.versions.restore"},
		{TypeName: "gtm_v1_undelete_container_version", Version: "v1", Method: "accounts.containers.versions.undelete"},
		{TypeName: "gtm_v1_publish_container_version", Version: "v1", Method: "accounts.containers.versions.publish"},
	}
}

func supportedMethodIDs() map[string]map[string]struct{} {
	covered := map[string]map[string]struct{}{"v1": {}, "v2": {}}
	add := func(version, method string) {
		covered[version]["tagmanager."+method] = struct{}{}
	}
	for _, spec := range resourceSpecs() {
		for _, method := range []string{spec.CreateMethod, spec.ReadMethod, spec.UpdateMethod, spec.DeleteMethod} {
			if method != "" {
				add(spec.Version, method)
			}
		}
	}
	for _, spec := range dataSourceSpecs() {
		add(spec.Version, spec.Method)
	}
	for _, spec := range actionSpecs() {
		add(spec.Version, spec.Method)
	}
	return covered
}
