package recipe

import (
    "fmt"
    "strings"
)

// Validate inspects recipe and returns issues.
func Validate(r Recipe) []Issue {
    var issues []Issue
    requiredModes := map[string][]string{
        "replace": {"fixed", "regex"},
        "delete_lines": {"fixed", "regex"},
        "rpm_install": {"upgrade", "install"},
    }

    for _, step := range r.Steps {
        stepType := step.Type
        cfg := step.Config

        add := func(level, msg string) {
            issues = append(issues, Issue{Level: level, StepID: step.ID, Message: msg})
        }

        // generic required config presence
        require := func(key string) {
            if value, ok := cfg[key]; !ok || fmt.Sprintf("%v", value) == "" {
                add("error", fmt.Sprintf("%s is required", key))
            }
        }

        switch stepType {
        case "mkdir":
            require("path")
        case "copy":
            require("src")
            require("dest")
        case "chmod":
            require("path")
            require("mode")
        case "chown":
            require("path")
            require("owner")
        case "extract_tar_gz", "extract_zip":
            require("src")
            require("dest")
            if _, ok := cfg["creates"]; !ok {
                add("warn", "creates is not set; idempotency may be improved")
            }
        case "rpm_install":
            require("rpms")
            require("mode")
            modeVal := fmt.Sprintf("%v", cfg["mode"])
            if !contains(requiredModes["rpm_install"], strings.ToLower(modeVal)) {
                add("error", "mode must be upgrade or install")
            }
        case "append_lines":
            require("file")
            require("lines")
            if backup, ok := cfg["backup"]; ok {
                if b, _ := backup.(bool); !b {
                    add("warn", "backup is disabled; risk of data loss")
                }
            }
        case "delete_lines":
            require("file")
            require("match")
            require("mode")
            modeVal := fmt.Sprintf("%v", cfg["mode"])
            if !contains(requiredModes["delete_lines"], strings.ToLower(modeVal)) {
                add("error", "mode must be fixed or regex")
            }
            if backup, ok := cfg["backup"]; ok {
                if b, _ := backup.(bool); !b {
                    add("warn", "backup is disabled; risk of data loss")
                }
            }
        case "replace":
            require("file")
            require("pattern")
            require("replacement")
            require("mode")
            modeVal := fmt.Sprintf("%v", cfg["mode"])
            if !contains(requiredModes["replace"], strings.ToLower(modeVal)) {
                add("error", "mode must be fixed or regex")
            }
            if backup, ok := cfg["backup"]; ok {
                if b, _ := backup.(bool); !b {
                    add("warn", "backup is disabled; risk of data loss")
                }
            }
        case "run_cmd":
            require("cmd")
            if _, ok := cfg["cwd"]; !ok {
                add("warn", "cwd is not set; command will run from script directory")
            }
        case "service_sysv":
            require("src")
            require("name")
        case "service_systemd":
            require("src")
            require("name")
        case "auto_service":
            require("name")
            require("sysv_src")
            require("systemd_src")
        default:
            add("warn", fmt.Sprintf("unknown step type %s", stepType))
        }
    }
    return issues
}

func contains(slice []string, value string) bool {
    for _, v := range slice {
        if v == value {
            return true
        }
    }
    return false
}
