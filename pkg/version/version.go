package version

// Base version information.
//
// This is the fallback data used when version information from git is not
// provided via go ldflags.
//
// If you are looking at these fields in the git tree, they look
// strange. They are modified on the fly by the build process.
var (
	GitVersion string = "v0.0.0-main"
	GitCommit  string = "abcd01234" // sha1 from git, output of $(git rev-parse HEAD)
)
