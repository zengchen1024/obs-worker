package build

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zengchen1024/obs-worker/utils"
)

type buildPkg struct {
	*buildHelper

	needOBSPackage bool
}

func (b *buildPkg) do() (int, error) {
	v := b.genArgs()

	out, err, code := utils.RunCmd(v...)
	utils.LogInfo("build pkd, err:%v, code=%d", err, code)

	if err != nil {
		err = fmt.Errorf("%s, %v", out, err)

		switch code {
		case 512:
			if b.getBuildInfo().Reason != "rebuild counter sync" {
				return 2, err
			}

			code = 0
			err = nil

		case 768:
			return 3, err

		default:
			return 1, err
		}
	}

	if v, _ := isEmptyFile(b.env.logFile); v {
		return 1, fmt.Errorf("build succeeded, but no logfile?")
	}

	return code, err
}

func (b *buildPkg) genArgs() []string {
	args := []string{
		filepath.Join(b.cfg.StateDir, "build", "build"),
	}

	add := func(v ...string) {
		args = append(args, v...)
	}

	add("--root", b.cfg.BuildRoot)

	b.genArgsForOthers(add)

	return args
}

func (b *buildPkg) genArgsForOthers(add func(...string)) {
	info := b.getBuildInfo()

	add("--clean")
	add("--changelog")

	if oldPkgDir := b.env.oldpkgdir; isFileExist(oldPkgDir) {
		add("--oldpackages", oldPkgDir)
	}

	add("--norootforbuild")

	/*
	  add("--norootforbuild" unless $buildinfo->{"rootforbuild"} || ($BSConfig::norootexceptions && grep {"$projid/$packid" =~ /^$_$/} keys %$BSConfig::norootexceptions))
	*/

	add("--baselibs-internal")

	env := &b.env

	add("--dist", env.config)
	add("--rpmlist", env.rpmList)
	add("--logfile", env.logFile)

	if info.Release != "" {
		add("--release", info.Release)
	}

	if info.DebugInfo != "" {
		add("--debug")
	}

	add("--arch", info.Arch)

	cfg := b.cfg
	if cfg.Jobs > 0 {
		add("--jobs", strconv.Itoa(cfg.Jobs))
	}
	if cfg.Threads > 0 {
		add("--threads", strconv.Itoa(cfg.Threads))
	}

	s := fmt.Sprintf(
		"\"Building %s for project %s, repository %s, arch %s, srcmd5 %s\"",
		info.Package, info.Project, info.Repository, info.Arch, info.SrcMd5,
	)
	add("--reason", s)

	disturl := ""
	if info.DistURL != "" {
		disturl = info.DistURL
	} else {
		disturl = fmt.Sprintf(
			"obs://%s/%s/%s-%s",
			info.Project,
			info.Repository,
			info.SrcMd5,
			info.Package,
		)
	}
	add("--disturl", disturl)

	if cfg.LocalKiwi != "" {
		add("--linksources")
	}

	s = filepath.Join(cfg.StateDir, "build", "signdummy")
	if info.getkiwimode() == "product" && isFileExist(s) {
		add("--signdummy")
	}

	packid := info.Package
	i := strings.LastIndex(packid, ":")
	v := i >= 0 && i+1 < len(packid) &&
		!strings.HasPrefix(packid, "_product:") &&
		!strings.HasSuffix(packid, "_patchinfo:")
	if v {
		if b.needOBSPackage {
			add(fmt.Sprintf("--obspackage=%s", packid[:i]))
		}
		add(fmt.Sprintf("--buildflavor=%s", packid[i+1:]))
	} else {
		if b.needOBSPackage {
			add(fmt.Sprintf("--obspackage=%s", packid))
		}
	}

	add(filepath.Join(env.srcdir, info.File))
}
