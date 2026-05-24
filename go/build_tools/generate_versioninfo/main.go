package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"text/template"

	"github.com/BurntSushi/toml"
)

const (
	resourcesDir          = "resources"
	baseResourcesDir      = resourcesDir + string(filepath.Separator) + "base"
	generatedResourcesDir = resourcesDir + string(filepath.Separator) + "generated"
)

type Project struct {
	Version string `toml:"version"`
}

type PyProject struct {
	Project Project `toml:"project"`
}

func main() {
	projectDir := os.Args[1]
	projectFile := filepath.Join(projectDir, "pyproject.toml")

	var pyproject PyProject
	if _, err := toml.DecodeFile(projectFile, &pyproject); err != nil {
		panic(err)
	}
	rawVersion := pyproject.Project.Version
	v := parsePythonProjectVersion(rawVersion)

	if err := os.MkdirAll(generatedResourcesDir, 0700); err != nil {
		panic(err)
	}
	generateAppManifestFile(v)
	generateVersionInfoFile(v)
}

// generate app manifest
func generateAppManifestFile(v PythonProjectVersion) {
	t, err := template.ParseFiles(filepath.Join(baseResourcesDir, "app.manifest.tmpl"))
	if err != nil {
		panic(err)
	}
	f, err := os.Create(filepath.Join(generatedResourcesDir, "app.manifest"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	t.Execute(f, struct{ ProductVersion string }{v.FileVersion().String()})
	if err := f.Close(); err != nil {
		panic(err)
	}
}

// generate versioninfo.json
func generateVersionInfoFile(v PythonProjectVersion) {
	data, err := os.ReadFile(filepath.Join(baseResourcesDir, "versioninfo.json"))
	if err != nil {
		panic(err)
	}
	versioninfo := map[string]any{}
	json.Unmarshal(data, &versioninfo)

	fixedFileInfo := versioninfo["FixedFileInfo"].(map[string]any)
	fixedFileInfo["FileVersion"] = v.FileVersion()
	fixedFileInfo["ProductVersion"] = v.FileVersion()
	stringFileInfo := versioninfo["StringFileInfo"].(map[string]any)
	stringFileInfo["FileVersion"] = v.String()
	stringFileInfo["ProductVersion"] = v.String()

	data, err = json.Marshal(versioninfo)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(filepath.Join(generatedResourcesDir, "versioninfo.json"), data, 0600)
	if err != nil {
		panic(err)
	}
}
