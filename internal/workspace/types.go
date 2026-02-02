// workspace types/constants

package workspace

import "time"

 
const (
    TempDirPrefix = ".rr-temp"
    RunIDPrefix   = "rr"
)
 
type Workspace struct {
    RunID string
    Path  string
}
 
type WorkspaceConfig struct {
    BaseDir string
    Keep    bool
}