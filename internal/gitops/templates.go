package gitops

import (
	"embed"
	"fmt"
)

//go:embed manifests/*.yaml
var manifestsFS embed.FS

var (
	repositorySecretTemplate string
	appProjectTemplate       string
	applicationTemplate      string
	applicationSetTemplate   string
	kargoProjectTemplate     string
	kargoWarehouseTemplate   string
	kargoStageTemplate       string
)

func init() {
	var err error
	repositorySecretTemplate, err = readManifest("secret.yaml")
	if err != nil {
		panic(err)
	}
	appProjectTemplate, err = readManifest("appproject.yaml")
	if err != nil {
		panic(err)
	}
	applicationTemplate, err = readManifest("application.yaml")
	if err != nil {
		panic(err)
	}
	applicationSetTemplate, err = readManifest("applicationset.yaml")
	if err != nil {
		panic(err)
	}
	kargoProjectTemplate, err = readManifest("kargo_project.yaml")
	if err != nil {
		panic(err)
	}
	kargoWarehouseTemplate, err = readManifest("kargo_warehouse.yaml")
	if err != nil {
		panic(err)
	}
	kargoStageTemplate, err = readManifest("kargo_stage.yaml")
	if err != nil {
		panic(err)
	}
}

func readManifest(name string) (string, error) {
	data, err := manifestsFS.ReadFile("manifests/" + name)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded manifest %s: %w", name, err)
	}
	return string(data), nil
}
