package models

import "time"

type Module struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Language     string          `json:"language"`
	Framework    string          `json:"framework"`
	Description  string          `json:"description"`
	Tags         []string        `json:"tags"`
	Dependencies []string        `json:"dependencies"`
	Files        []ModuleFile    `json:"files"`
	Versions     []ModuleVersion `json:"versions"`
	Owner        string          `json:"owner"`
	Favorite     bool            `json:"favorite"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type ModuleFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
}

type ModuleVersion struct {
	Version   string       `json:"version"`
	Message   string       `json:"message"`
	Files     []ModuleFile `json:"files"`
	CreatedAt time.Time    `json:"created_at"`
}

type GenerateRequest struct {
	ProjectName string   `json:"project_name"`
	Language    string   `json:"language"`
	Framework   string   `json:"framework"`
	ModuleIDs   []string `json:"module_ids"`
}

type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type GenerateResponse struct {
	ProjectName       string          `json:"project_name"`
	Files             []GeneratedFile `json:"files"`
	DependencySummary []string        `json:"dependency_summary"`
	ArchiveName       string          `json:"archive_name"`
}
