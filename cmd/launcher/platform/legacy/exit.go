package legacy

import (
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/cmd/launcher/platform/common"
)

var exitCodes = map[common.LifecycleExitError]int{
	// launch phase errors: 700-799
	common.LaunchError: 702, // LaunchError indicates generic launch error
}

func (p *legacyPlatform) CodeFor(errType common.LifecycleExitError) int {
	return CodeFor(errType)
}

func CodeFor(errType common.LifecycleExitError) int {
	if code, ok := exitCodes[errType]; ok {
		return code
	}
	return cmd.CodeFailed
}
