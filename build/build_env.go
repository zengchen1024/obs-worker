package build

import (
	"os"
	"path/filepath"
)

type buildEnv struct {
	srcdir    string
	pkgdir    string
	oldpkgdir string
	meta      string
	packages  string
	mountDir  string
	config    string
	rpmList   string
	logFile   string
}

func (env *buildEnv) init(cfg *Config) error {
	if err := createTmpfs(cfg); err != nil {
		return err
	}

	buildroot := cfg.BuildRoot

	env.meta = filepath.Join(buildroot, ".build.meta")
	env.srcdir = filepath.Join(buildroot, ".build-srcdir")
	env.pkgdir = filepath.Join(buildroot, ".pkgs")
	env.config = filepath.Join(buildroot, ".build.config")
	env.rpmList = filepath.Join(buildroot, ".build.rpmlist")
	env.logFile = filepath.Join(buildroot, ".build.log")
	env.packages = filepath.Join(buildroot, ".build.packages")
	env.mountDir = filepath.Join(buildroot, ".mount")
	env.oldpkgdir = filepath.Join(buildroot, ".build.oldpackages")

	os.Remove(env.meta)
	os.RemoveAll(env.srcdir)
	os.RemoveAll(env.oldpkgdir)
	os.RemoveAll(env.packages)

	cleanDir(env.pkgdir)

	return os.Setenv("BUILD_DIR", filepath.Join(cfg.StateDir, "build"))
}

func createTmpfs(cfg *Config) error {
	/*
		if opt.vmTmpfsMode {
			out, err := utils.RunCmd(
				"mount", "-t", "tmpfs",
				fmt.Sprintf("-osize=%dM", opt.vmDiskRootSize),
				"none", buildroot,
			)
			if err != nil {
				return fmt.Errorf("%v %s", err, out)
			}
		}
	*/
	return nil
}
