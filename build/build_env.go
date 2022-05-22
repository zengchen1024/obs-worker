package build

import (
	"os"
	"path/filepath"

	"github.com/zengchen1024/obs-worker/utils"
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
	otherDir  string
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
	env.otherDir = filepath.Join(buildroot, ".build.packages", "OTHER")

	if !isFileExist(buildroot) {
		if err := mkdir(buildroot); err != nil {
			return err
		}
	}

	os.Remove(env.meta)
	os.Remove(env.packages)

	os.Remove(env.logFile)
	utils.WriteFile(env.logFile, nil)

	if err := cleanDir(env.srcdir); err != nil {
		return err
	}

	if err := cleanDir(env.pkgdir); err != nil {
		return err
	}

	os.RemoveAll(env.oldpkgdir)

	// obs-build/build need this env
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
