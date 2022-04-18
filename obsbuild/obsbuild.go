package obsbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/statistic"
)

const knownVMs = "xen kvm zvm emulator pvm"

func (b *buildOnce) getObsBuildArgs() []string {
	h := obsbuildHelper{b}
	return h.genArgs()
}

type obsbuildHelper struct {
	b *buildOnce
}

func (h *obsbuildHelper) getVMType() string {
	return h.b.opts.vmType
}

func (h *obsbuildHelper) genArgs() []string {
	args := []string{
		filepath.Join(h.b.opts.statedir, "build", "build"),
	}

	add := func(v ...string) {
		args = append(args, v...)
	}

	vm := h.getVMType()

	if strings.Contains(knownVMs, vm) {
		h.genArgsForVM(vm, add)
	} else if vm == "openstack" {
		h.genArgsForOpenstack(add)
	} else {
		add("--root", h.b.opts.buildroot)
		if vm != "" {
			add("--vm-type", vm)
		}
	}

	h.genArgsForOthers("", "", false, add)

	h.genArgsForKiwiImage(add)

	return args
}

func (h *obsbuildHelper) genArgsForVM(vm string, add func(...string)) {
	dir := h.b.env.mountDir
	if !isFileExist(dir) {
		mkdir(dir)
	}

	opts := h.b.opts

	add("--root", dir)
	add("--vm-type", vm)
	add("--vm-disk", opts.vmRoot)
	add("--vm-swap", opts.vmSwap)

	if vm == "emulator" && opts.emulatorScript != "" {
		add("--emulator-script", opts.emulatorScript)
	}

	add("--statistics")
	add("--vm-watchdog")

	mem := opts.vmMemory
	if mem == 0 {
		s := filepath.Join(opts.buildroot, "memory")
		if v, err := os.ReadFile(s); err == nil {
			mem, _ = strconv.Atoi(string(v))
		}
	}
	if mem > 0 {
		add("--memory", strconv.Itoa(mem))
	}

	if opts.vmKernel != "" {
		add("--vm-kernel", opts.vmKernel)
	}
	if opts.vmInitrd != "" {
		add("--vm-initrd", opts.vmInitrd)
	}
	if opts.vmCustomOption != "" {
		add("--vm-custom-opt=" + opts.vmCustomOption)
	}
	if opts.vmDiskRootSize > 0 {
		add("--vmdisk-rootsize", strconv.Itoa(opts.vmDiskRootSize))
	}
	if opts.vmDiskSwapSize > 0 {
		add("--vmdisk-swapsize", strconv.Itoa(opts.vmDiskSwapSize))
	}
	if opts.vmDiskFileSystem != "" {
		add("--vmdisk-filesystem", opts.vmDiskFileSystem)
	}
	if opts.vmDiskMountOptions != "" {
		add("--vmdisk-mount-options=" + opts.vmDiskMountOptions)
	}
	if opts.vmDiskClean {
		add("--vmdisk-clean")
	}
	if opts.hugetlbfs != "" {
		add("--hugetlbfs", opts.hugetlbfs)
	}
	if opts.vmWorkerName != "" {
		add("--vm-worker", opts.vmWorkerName)
	}
	if opts.vmWorkerInstance > 0 {
		add("--vm-worker-nr", strconv.Itoa(opts.vmWorkerInstance))
	}
	if opts.vmEnableConsole {
		add("--vm-enable-console")
	}
}

func (h *obsbuildHelper) genArgsForOpenstack(add func(...string)) {
	dir := h.b.env.mountDir
	if !isFileExist(dir) {
		mkdir(dir)
	}

	opts := h.b.opts

	add("--root", dir)
	add("--vm-type", "openstack")
	add("--vm-disk", opts.vmRoot)
	add("--vm-swap", opts.vmSwap)
	add("--vm-server", opts.openstackServer)
	add("--vm-worker", opts.vmWorkerName)
	add("--vm-kernel", opts.vmKernel)
	add("--openstack-flavor", opts.openstackFlavor)
}

func (h *obsbuildHelper) genArgsForOthers(
	oldPkgDir, disturl string, needObsPackage bool,
	add func(...string),
) {
	add("--clean")
	add("--changelog")
	if oldPkgDir != "" && isFileExist(oldPkgDir) {
		add("--oldpackages", oldPkgDir)
	}
	/*
	  add("--norootforbuild" unless $buildinfo->{"rootforbuild"} || ($BSConfig::norootexceptions && grep {"$projid/$packid" =~ /^$_$/} keys %$BSConfig::norootexceptions))
	*/
	env := &h.b.env
	info := h.b.info
	add("--baselibs-internal")
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

	opts := h.b.opts
	if opts.jobs > 0 {
		add("--jobs", strconv.Itoa(opts.jobs))
	}
	if opts.threads > 0 {
		add("--threads", strconv.Itoa(opts.threads))
	}

	s := fmt.Sprintf(
		"Building %s for project %s, repository %s, arch %s, srcmd5 %s",
		info.Package, info.Project, info.Repository, info.Arch, info.Srcmd5,
	)
	add("--reason", s)

	add("--disturl", disturl)

	if opts.localKiwi != "" {
		add("--linksources")
	}

	s = filepath.Join(opts.statedir, "build", "signdummy")
	if info.getkiwimode() == "product" && isFileExist(s) {
		add("--signdummy")
	}

	packid := info.Package
	i := strings.LastIndex(packid, ":")
	v := i >= 0 && i+1 < len(packid) &&
		!strings.HasPrefix(packid, "_product:") &&
		!strings.HasSuffix(packid, "_patchinfo:")
	if v {
		if needObsPackage {
			add(fmt.Sprintf("--obspackage=%s", packid[:i]))
		}
		add(fmt.Sprintf("--buildflavor=%s", packid[i+1:]))
	} else {
		if needObsPackage {
			add(fmt.Sprintf("--obspackage=%s", packid))
		}
	}

	add(filepath.Join(env.srcdir, info.File))
}

func (h *obsbuildHelper) genArgsForKiwiImage(add func(...string)) {
	info := h.b.info

	if info.getkiwimode() != "image" {
		return
	}
}

func (h *obsbuildHelper) postAction() {
	h.postActionForVMWorker()

	other := filepath.Join(h.b.env.packages, "OTHER")
	if !isFileExist(other) {
		mkdir(other)
	}

	if b, err := statistic.Mashal(&h.b.stats); err == nil {
		tmp := filepath.Join(other, "_statistics.new")
		if nil == writeFile(tmp, b) {
			os.Rename(tmp, strings.TrimSuffix(tmp, ".new"))
		}
	}

	// write buildinfo->{'outbuildinfo'} to xml

}

func (h *obsbuildHelper) postActionForVMWorker() {
	vm := h.getVMType()
	if !strings.Contains(knownVMs, vm) && vm != "openstack" {
		return
	}

	env := h.b.env

	pkgDir := env.packages
	os.RemoveAll(pkgDir)

	// move directory with extracted build results
	s := filepath.Join(env.mountDir, ".build.packages")
	if err := os.Rename(s, pkgDir); err != nil {
		return
	}

	s = "."
	os.Symlink(s, filepath.Join(pkgDir, "SRPMS"))
	os.Symlink(s, filepath.Join(pkgDir, "DEBS"))
	os.Symlink(s, filepath.Join(pkgDir, "KIWI"))

	h.convertStatToXml()
}

// convert build statistics into xml
func (h *obsbuildHelper) convertStatToXml() {
	s := filepath.Join(h.b.env.pkgdir, "OTHER", "_statistics")
	if !isFileExist(s) {
		return
	}

	stats := &h.b.stats

	setTimes := func(key string, v int) {
		t := statistic.Time{Unit: "s", Value: v}
		switch key {
		case "TIME_preinstall":
			stats.Times.Preinstall = t
		case "TIME_install":
			stats.Times.Install = t
		case "TIME_main_build":
			stats.Times.Main = t
		case "TIME_postchecks":
			stats.Times.Postchecks = t
		case "TIME_rpmlint":
			stats.Times.Rpmlint = t
		case "TIME_buildcmp":
			stats.Times.Buildcmp = t
		case "TIME_deltarpms":
			stats.Times.Deltarpms = t
		}
	}

	readFileLineByLine(s, func(l string) bool {
		items := strings.Split(l, ": ")
		v, _ := strconv.Atoi(items[1])

		switch items[0] {
		case "MAX_mb_used_on_disk":
			stats.Disk.Usage.Size = statistic.Size{Unit: "M", Value: v}

		case "MAX_mb_used_memory":
			stats.Memory.Usage = statistic.Size{Unit: "M", Value: v}

		case "IO_requests_read", "IO_requests_write":
			stats.Disk.Usage.IORequests += v

		case "IO_sectors_read", "IO_sectors_write":
			stats.Disk.Usage.IOSectors += v

		default:
			setTimes(items[0], v)
		}

		return false
	})
}
