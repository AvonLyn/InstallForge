package recipe

import "time"

// Recipe represents a project recipe.
type Recipe struct {
    SchemaVersion string        `json:"schema_version"`
    Project       ProjectMeta   `json:"project"`
    Vars          map[string]string `json:"vars"`
    Steps         []Step        `json:"steps"`
    UpdatedAt     time.Time     `json:"updatedAt"`
}

// ProjectMeta describes project information.
type ProjectMeta struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Target      []string `json:"target"`
}

// Step defines a single action.
type Step struct {
    ID     string                 `json:"id"`
    Name   string                 `json:"name"`
    Type   string                 `json:"type"`
    Config map[string]interface{} `json:"config"`
}

// Issue represents validation issue.
type Issue struct {
    Level string `json:"level"`
    StepID string `json:"stepId"`
    Message string `json:"message"`
}

// NewEmptyRecipe returns starter recipe.
func NewEmptyRecipe(projectID, name string) Recipe {
    return Recipe{
        SchemaVersion: "1.0",
        Project: ProjectMeta{
            ID: projectID,
            Name: name,
            Description: "",
            Target: []string{"oracle_linux_6_9", "kylinsec_3_4"},
        },
        Vars: map[string]string{
            "INSTALL_ROOT": "/opt/demo",
            "LOG_DIR": "/var/log/asg",
        },
        Steps:     []Step{},
        UpdatedAt: time.Now(),
    }
}
