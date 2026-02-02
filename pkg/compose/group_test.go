package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupByCompose_SingleGroup(t *testing.T) {
	endpoints := map[string]ServiceGroupEntry{
		"web":   {ComposeFile: "docker-compose.yml", Profile: ""},
		"api":   {ComposeFile: "docker-compose.yml", Profile: ""},
		"redis": {ComposeFile: "docker-compose.yml", Profile: ""},
	}

	groups := GroupByCompose(endpoints)
	require.Len(t, groups, 1)
	assert.Equal(t, "docker-compose.yml", groups[0].Name)
	assert.Equal(t, "docker-compose.yml", groups[0].ComposeFile)
	assert.Empty(t, groups[0].Profile)
	// Services should be sorted alphabetically.
	assert.Equal(t,
		[]string{"api", "redis", "web"}, groups[0].Services,
	)
}

func TestGroupByCompose_MultipleGroups(t *testing.T) {
	endpoints := map[string]ServiceGroupEntry{
		"web":      {ComposeFile: "compose-app.yml", Profile: ""},
		"db":       {ComposeFile: "compose-infra.yml", Profile: ""},
		"redis":    {ComposeFile: "compose-infra.yml", Profile: ""},
		"worker":   {ComposeFile: "compose-app.yml", Profile: ""},
		"chromadb": {ComposeFile: "compose-infra.yml", Profile: "rag"},
	}

	groups := GroupByCompose(endpoints)
	require.Len(t, groups, 3)

	// Groups sorted by name.
	assert.Equal(t, "compose-app.yml", groups[0].Name)
	assert.Equal(t,
		[]string{"web", "worker"}, groups[0].Services,
	)

	assert.Equal(t, "compose-infra.yml", groups[1].Name)
	assert.Equal(t,
		[]string{"db", "redis"}, groups[1].Services,
	)
	assert.Empty(t, groups[1].Profile)

	assert.Equal(t, "compose-infra.yml:rag", groups[2].Name)
	assert.Equal(t, []string{"chromadb"}, groups[2].Services)
	assert.Equal(t, "rag", groups[2].Profile)
}

func TestGroupByCompose_WithProfiles(t *testing.T) {
	endpoints := map[string]ServiceGroupEntry{
		"svc-a": {ComposeFile: "compose.yml", Profile: "dev"},
		"svc-b": {ComposeFile: "compose.yml", Profile: "prod"},
		"svc-c": {ComposeFile: "compose.yml", Profile: "dev"},
	}

	groups := GroupByCompose(endpoints)
	require.Len(t, groups, 2)

	assert.Equal(t, "compose.yml:dev", groups[0].Name)
	assert.Equal(t,
		[]string{"svc-a", "svc-c"}, groups[0].Services,
	)
	assert.Equal(t, "dev", groups[0].Profile)

	assert.Equal(t, "compose.yml:prod", groups[1].Name)
	assert.Equal(t, []string{"svc-b"}, groups[1].Services)
	assert.Equal(t, "prod", groups[1].Profile)
}

func TestGroupByCompose_EmptyInput(t *testing.T) {
	groups := GroupByCompose(map[string]ServiceGroupEntry{})
	assert.Empty(t, groups)
}

func TestGroupByCompose_SingleService(t *testing.T) {
	endpoints := map[string]ServiceGroupEntry{
		"lonely": {ComposeFile: "solo.yml", Profile: ""},
	}

	groups := GroupByCompose(endpoints)
	require.Len(t, groups, 1)
	assert.Equal(t, "solo.yml", groups[0].Name)
	assert.Equal(t, []string{"lonely"}, groups[0].Services)
}

func TestGroupByCompose_DeterministicOrdering(t *testing.T) {
	// Run multiple times to verify sort stability.
	for i := 0; i < 10; i++ {
		endpoints := map[string]ServiceGroupEntry{
			"z-svc":  {ComposeFile: "b.yml", Profile: ""},
			"a-svc":  {ComposeFile: "a.yml", Profile: ""},
			"m-svc":  {ComposeFile: "b.yml", Profile: ""},
			"b-svc2": {ComposeFile: "a.yml", Profile: "x"},
		}

		groups := GroupByCompose(endpoints)
		require.Len(t, groups, 3)
		assert.Equal(t, "a.yml", groups[0].Name)
		assert.Equal(t, "a.yml:x", groups[1].Name)
		assert.Equal(t, "b.yml", groups[2].Name)

		assert.Equal(t, []string{"a-svc"}, groups[0].Services)
		assert.Equal(t, []string{"b-svc2"}, groups[1].Services)
		assert.Equal(t,
			[]string{"m-svc", "z-svc"}, groups[2].Services,
		)
	}
}
