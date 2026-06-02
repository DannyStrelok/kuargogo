package infrastructure

import "embed"

//go:embed playbooks
//go:embed playbooks/*
var PlaybooksFS embed.FS
