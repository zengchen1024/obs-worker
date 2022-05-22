package build

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/zengchen1024/obs-worker/utils"
)

const (
	buildActionCancel = "cancel"
	buildActionBuild  = "build"
)

var errorCancel = fmt.Errorf("cancel")

type buildPkg struct {
	*buildHelper

	needOBSPackage bool

	action string
	lock   sync.Mutex
	wg     sync.WaitGroup
}

func (b *buildPkg) do() (code int, err error) {
	// first check
	b.lock.Lock()
	if b.action == buildActionCancel {
		code = 1
		err = errorCancel

		b.lock.Unlock()

		return
	}

	v := b.genArgs()

	// second check
	b.lock.Lock()
	if b.action == buildActionCancel {
		code = 1
		err = errorCancel

		b.lock.Unlock()

		return
	}

	b.action = buildActionBuild
	b.wg.Add(1)

	go func(args []string) {
		defer b.wg.Done()

		code, err = b.build(args)

		b.lock.Lock()
		b.action = ""
		b.lock.Unlock()

	}(v)

	b.lock.Unlock()

	// now, wait
	b.wg.Wait()

	return
}

func (b *buildPkg) kill() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.action == buildActionBuild {
		args := []string{
			filepath.Join(b.cfg.StateDir, "build", "build"),
			"--root", b.cfg.BuildRoot,
			"--kill",
		}

		out, err, _ := utils.RunCmd(args...)
		if err != nil {
			return fmt.Errorf("%s, %s", out, err.Error())
		}
	}

	b.action = buildActionCancel

	return nil
}

func (b *buildPkg) build(args []string) (int, error) {
	out, err, code := utils.RunCmd(args...)

	utils.LogInfo("build pkd, err: %s, code: %d", err.Error(), code)

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
