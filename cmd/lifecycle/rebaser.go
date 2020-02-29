package main

import (
	"fmt"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/local"
	"github.com/buildpacks/imgutil/remote"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"

	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/lifecycle/auth"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/docker"
	"github.com/buildpacks/lifecycle/image"
)

type rebaseCmd struct {
	//flags
	imageNames  []string
	runImageRef string
	useDaemon   bool
	uid, gid    int

	//set if necessary before dropping privileges
	docker client.CommonAPIClient
}

func (r *rebaseCmd) Init() {
	cmd.DeprecatedFlagRunImage(&r.runImageRef)
	cmd.FlagUseDaemon(&r.useDaemon)
	cmd.FlagUID(&r.uid)
	cmd.FlagGID(&r.gid)
}

func (r *rebaseCmd) Args(nargs int, args []string) error {
	if nargs == 0 {
		return cmd.FailErrCode(errors.New("at least one image argument is required"), cmd.CodeInvalidArgs, "parse arguments")
	}
	r.imageNames = args
	return nil
}

func (r *rebaseCmd) Privileges() error {
	if r.useDaemon {
		var err error
		r.docker, err = docker.Client()
		if err != nil {
			return cmd.FailErr(err, "initialize docker client")
		}
	}
	if err := cmd.RunAs(r.uid, r.gid); err != nil {
		cmd.FailErr(err, fmt.Sprintf("exec as user %d:%d", r.uid, r.gid))
	}
	return nil
}

func (r *rebaseCmd) Exec() error {
	rebaser := &lifecycle.Rebaser{
		Logger: cmd.Logger,
	}

	registry, err := image.EnsureSingleRegistry(r.imageNames...)
	if err != nil {
		return cmd.FailErrCode(err, cmd.CodeInvalidArgs, "parse arguments")
	}

	var appImage imgutil.Image
	if r.useDaemon {
		appImage, err = local.NewImage(
			r.imageNames[0],
			r.docker,
			local.FromBaseImage(r.imageNames[0]),
		)
	} else {
		appImage, err = remote.NewImage(
			r.imageNames[0],
			auth.EnvKeychain(cmd.EnvRegistryAuth),
			remote.FromBaseImage(r.imageNames[0]),
		)
	}
	if err != nil || !appImage.Found() {
		return cmd.FailErr(err, "access image to rebase")
	}

	var md lifecycle.LayersMetadata
	if err := lifecycle.DecodeLabel(appImage, lifecycle.LayerMetadataLabel, &md); err != nil {
		return err
	}

	if r.runImageRef == "" {
		if md.Stack.RunImage.Image == "" {
			return cmd.FailErrCode(errors.New("-image is required when there is no stack metadata available"), cmd.CodeInvalidArgs, "parse arguments")
		}
		r.runImageRef, err = md.Stack.BestRunImageMirror(registry)
		if err != nil {
			return err
		}
	}

	var newBaseImage imgutil.Image
	if r.useDaemon {
		newBaseImage, err = local.NewImage(
			r.imageNames[0],
			r.docker,
			local.FromBaseImage(r.runImageRef),
		)
	} else {
		newBaseImage, err = remote.NewImage(
			r.imageNames[0],
			auth.EnvKeychain(cmd.EnvRegistryAuth),
			remote.FromBaseImage(r.runImageRef),
		)
	}
	if err != nil || !newBaseImage.Found() {
		return cmd.FailErr(err, "access run image")
	}

	if err := rebaser.Rebase(appImage, newBaseImage, r.imageNames[1:]); err != nil {
		if _, ok := err.(*imgutil.SaveError); ok {
			return cmd.FailErrCode(err, cmd.CodeFailedSave, "rebase")
		}
		return cmd.FailErr(err, "rebase")
	}
	return nil
}
