package dashboard

import "embed"

// Assets embeds the dashboard static files.
//go:embed assets/*
var Assets embed.FS
