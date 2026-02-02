package compose

import "sort"

// ServiceGroupEntry describes a single service's compose context,
// used as input to GroupByCompose.
type ServiceGroupEntry struct {
	// ComposeFile is the path to the service's compose file.
	ComposeFile string
	// Profile is the optional compose profile.
	Profile string
}

// ServiceGroup aggregates services that share the same compose file and
// profile so they can be managed together in a single compose command.
type ServiceGroup struct {
	// Name is a derived name for the group (compose file + profile).
	Name string
	// ComposeFile is the shared compose file path.
	ComposeFile string
	// Profile is the shared compose profile.
	Profile string
	// Services lists the service names in this group.
	Services []string
}

// GroupByCompose takes a map of service-name to ServiceGroupEntry and
// returns a slice of ServiceGroups, one per unique compose-file+profile
// combination. Groups and services within groups are sorted for
// deterministic ordering.
func GroupByCompose(
	endpoints map[string]ServiceGroupEntry,
) []ServiceGroup {
	type groupKey struct {
		composeFile string
		profile     string
	}

	grouped := make(map[groupKey][]string)
	for svcName, entry := range endpoints {
		key := groupKey{
			composeFile: entry.ComposeFile,
			profile:     entry.Profile,
		}
		grouped[key] = append(grouped[key], svcName)
	}

	groups := make([]ServiceGroup, 0, len(grouped))
	for key, services := range grouped {
		sort.Strings(services)

		name := key.composeFile
		if key.profile != "" {
			name += ":" + key.profile
		}

		groups = append(groups, ServiceGroup{
			Name:        name,
			ComposeFile: key.composeFile,
			Profile:     key.profile,
			Services:    services,
		})
	}

	// Sort groups by name for deterministic output.
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}
