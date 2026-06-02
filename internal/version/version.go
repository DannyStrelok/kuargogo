package version

// Current is the application version.
// It defaults to "dev" for local builds.
// For releases, it is replaced at build time using:
// -ldflags "-X github.com/DannyStrelok/kuargogo/internal/version.Current=vX.Y.Z"
var Current = "dev"

