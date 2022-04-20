package obsbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/config"
	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/sdk/source"
	"github.com/zengchen1024/obs-worker/sdk/sslcert"
	"github.com/zengchen1024/obs-worker/sdk/statistic"
	"github.com/zengchen1024/obs-worker/sdk/worker"
	"github.com/zengchen1024/obs-worker/utils"
)

type options struct {
	buildroot string
	statedir  string

	vmTmpfsMode bool

	srcServer string
	cacheDir  string
	cacheSize int

	workerId  string
	localKiwi string

	vmType             string
	vmRoot             string
	vmSwap             string
	emulatorScript     string
	vmEnableConsole    bool
	vmWorkerInstance   int
	vmWorkerName       string
	hugetlbfs          string
	vmDiskClean        bool
	vmDiskMountOptions string
	vmDiskFileSystem   string
	vmDiskSwapSize     int
	vmDiskRootSize     int
	vmCustomOption     string
	vmInitrd           string
	vmKernel           string
	vmMemory           int

	openstackServer string
	openstackFlavor string

	jobs    int
	threads int
}

func (o *options) setdefault() {
	if o.vmRoot == "" {
		o.vmRoot = o.buildroot + ".img"
	}

	if o.vmSwap == "" {
		o.vmSwap = o.buildroot + ".swap"
	}
}

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

type buildOnce struct {
	opts *options
	info *BuildInfo
	env  buildEnv

	hc utils.HttpClient

	meta []string

	state worker.Worker
	stats statistic.BuildStatistics

	kiwiOrigins map[string][]string

	cfg  *Config
	port int
}

var instance *buildOnce

func Init(cfg *Config, port int) error {
	b := buildOnce{
		cfg:  cfg,
		port: port,
	}

	if err := b.getWorkerInfo(); err != nil {
		return err
	}

	b.sendIdleState()

	instance = &b

	return nil
}

func Exit() {
	if instance != nil {
		instance.sendExitState()
	}
}

func (b *buildOnce) setKiwiOrigin(k, v string) {
	if items, ok := b.kiwiOrigins[k]; ok {
		b.kiwiOrigins[k] = append(items, v)
	} else {
		b.kiwiOrigins[k] = []string{v}
	}
}

func (b *buildOnce) getKiwiOrigin(k string) []string {
	return b.kiwiOrigins[k]
}

func (b *buildOnce) getWorkerId() string {
	return b.opts.workerId
}
func (b *buildOnce) getCacheDir() string {
	return b.opts.cacheDir
}

func (b *buildOnce) getCacheSize() int {
	return b.opts.cacheSize
}

func (b *buildOnce) getSrcServer() string {
	if b.info.SrcServer != "" {
		return b.info.SrcServer
	}

	return b.opts.srcServer
}

func (b *buildOnce) getBuildRoot() string {
	return b.opts.buildroot
}

func doBuild(opt options, info *BuildInfo) error {
	buildroot := opt.buildroot

	env := buildEnv{
		srcdir:    filepath.Join(buildroot, ".build-srcdir"),
		pkgdir:    filepath.Join(buildroot, ".pkgs"),
		oldpkgdir: filepath.Join(buildroot, ".build.oldpackages"),
		meta:      filepath.Join(buildroot, ".build.meta"),
		packages:  filepath.Join(buildroot, ".build.packages"),
		mountDir:  filepath.Join(buildroot, ".mount"),
		config:    filepath.Join(buildroot, ".build.config"),
		rpmList:   filepath.Join(buildroot, ".build.rpmlist"),
		logFile:   filepath.Join(buildroot, ".build.log"),
	}

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

	cleanBuildEnv(env)

	// download phase

	return nil
}

func cleanBuildEnv(env buildEnv) {
	os.Remove(env.meta)
	os.RemoveAll(env.srcdir)
	os.RemoveAll(env.oldpkgdir)
	os.RemoveAll(env.packages)

	cleanDir(env.pkgdir)
}

func cleanDir(dir string) {
	d, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, item := range d {
		os.RemoveAll(filepath.Join(dir, item.Name()))
	}
}

func (b *buildOnce) download(statedir string) error {
	if err := mkdir(b.env.srcdir); err != nil {
		return err
	}

	if b.info.isDeltaMode() {
		return b.downloadForDelta(statedir)
	}

	b.downloadForKiwiMode("")

	return nil
}

func (b *buildOnce) downloadForDelta(statedir string) error {
	b.meta = append(b.meta, genMetaLine(b.info.getSrcmd5(), b.info.Package))

	filename := "deltagen.spec"

	if b, _ := isEmptyFile(filepath.Join(b.env.srcdir, filename)); b {

	}

	b.info.File = filename

	return nil
}

func (b *buildOnce) downloadForKiwiMode(statedir string) ([]string, error) {
	needsBinaries := false
	needAppxSSLCert := false
	needOBSPackage := false

	mode := b.info.getkiwimode()

	if mode == "" {
		re0 := regexp.MustCompile("^#\\s*needsbinariesforbuild\\s*$")
		re1 := regexp.MustCompile("^#\\s*needssslcertforbuild\\s*$|^Obs:\\s*needssslcertforbuild\\s*$")
		re2 := regexp.MustCompile("^(?:#|Obs:)\\s*needsappxsslcertforbuild\\s*$")

		needSSLCert := false

		filename := filepath.Join(b.env.srcdir, b.info.File)

		readFileLineByLine(filename, func(line string) bool {
			b := []byte(line)
			needsBinaries = re0.Match(b)
			needSSLCert = re1.Match(b)
			needAppxSSLCert = re2.Match(b)
			needOBSPackage = strings.Contains(line, "@OBS_PACKAGE@")

			return needsBinaries && needSSLCert && needAppxSSLCert && needOBSPackage
		})

		if needSSLCert {
			if err := b.downloadSSLCert(); err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}

func (b *buildOnce) getSource() error {
	info := b.info
	v, err := b.downloadPkgSource(b.getSrcServer(), info.Project, info.Package, info.Srcmd5, b.env.srcdir)
	if err != nil {
		return err
	}

	// verify sources
	keys := make([]string, len(v))
	m := make(map[string]string)
	for i := range v {
		item := &v[i]
		m[item.Name] = item.MD5
		keys[i] = item.Name
	}
	sort.Strings(keys)

	md5s := make([]string, 0, len(v))
	for _, k := range keys {
		if f := filepath.Join(b.env.srcdir, k); !isFileExist(f) {
			return fmt.Errorf("%s is not exist", f)
		}

		md5s = append(md5s, genMetaLine(m[k], k))
	}

	if md5 := utils.GenMD5([]byte(strings.Join(md5s, "\n"))); md5 != b.info.VerifyMd5 {
		return fmt.Errorf("source verification fails, %s != %s", md5, b.info.VerifyMd5)
	}

	return nil
}

func (b *buildOnce) getBdeps() ([]string, error) {
	if m := b.info.getkiwimode(); m != "image" && m != "product" {
		return nil, nil
	}

	items := b.info.getSrcBDep()
	meta := make([]string, len(items))
	for i := range items {
		item := &items[i]

		saveTo := filepath.Join(b.env.srcdir, "images", item.Project, item.Package)
		if err := mkdirAll(saveTo); err != nil {
			return nil, err
		}

		_, err := b.downloadPkgSource(b.getSrcServer(), item.Project, item.Package, item.Srcmd5, saveTo)
		if err != nil {
			return nil, err
		}

		meta[i] = genMetaLine(item.Srcmd5, fmt.Sprintf("%s/%s", item.Project, item.Package))
	}

	return meta, nil
}

func (b *buildOnce) downloadPkgSource(srcServer, project, pkg, srcmd5, saveTo string) (
	[]filereceiver.CPIOFileMeta, error,
) {
	opts := source.ListOpts{
		Project: project,
		Package: pkg,
		Srcmd5:  srcmd5,
	}

	check := func(name string, h *filereceiver.CPIOFileHeader) (
		string, string, bool, error,
	) {
		return name, filepath.Join(saveTo, name), true, nil
	}

	return source.List(&b.hc, srcServer, &opts, check)
}

func (b *buildOnce) downloadSSLCert() error {
	v, err := sslcert.List(&b.hc, b.getSrcServer(), b.info.Project, true)
	if err != nil {
		return err
	}

	if v != "" {
		return writeFile(filepath.Join(b.env.srcdir, "_projectcert.crt"), []byte(v))
	}

	return nil
}

func (b *buildOnce) downloadConfig() error {
	return config.Download(
		&b.hc,
		b.getSrcServer(),
		&config.DownloadOpts{
			b.info.Project,
			b.info.Repository,
		},
		b.env.config,
	)
}

func (b *buildOnce) genRpmList(pre *preInstallImage) error {
	pkgdir := b.env.pkgdir
	kiwiMode := b.info.getkiwimode()
	imageBins, _, _ := pre.getImageBins()

	rpmList := []string{}

	bdeps := b.info.BDep
	for i := range bdeps {
		bdep := &bdeps[i]

		if bdep.Package != "" || bdep.RepoArch == "src" {
			continue
		}

		if kiwiMode != "" && bdep.NoInstall {
			continue
		}

		bin := bdep.Name

		if imageBins[bin] != "" {
			rpmList = append(rpmList, fmt.Sprintf("%s preinstallimage", bin))
			continue
		}

		for j, suf := range knownBins {
			if f := filepath.Join(pkgdir, bin+suf); isFileExist(f) {
				rpmList = append(rpmList, fmt.Sprintf("%s %s", bin, f))
				break
			}

			if j == len(knownBins)-1 {
				return fmt.Errorf("missing package: %s", bin)
			}
		}
	}

	if s := pre.getImageName(); s != "" {
		rpmList = append(
			rpmList,
			fmt.Sprintf("preinstallimage: %s", filepath.Join(pkgdir, s)),
		)
	}

	if s := pre.getImageSource(); s != "" {
		rpmList = append(
			rpmList,
			fmt.Sprintf("preinstallimagesource: %s", s),
		)
	}

	if s := b.opts.localKiwi; s != "" {
		if f := filepath.Join(s, s+".rpm"); isFileExist(f) {
			rpmList = append(rpmList, fmt.Sprintf("%s %s", s, f))
		}
	}

	add := func(item string, ok func(*BDep) bool) {
		names := make([]string, 0, len(bdeps))

		for i := range bdeps {
			if bdep := &bdeps[i]; ok(bdep) {
				names = append(names, bdep.Name)
			}
		}

		if len(names) > 0 {
			rpmList = append(
				rpmList,
				fmt.Sprintf("%s: %s", item, strings.Join(names, " ")),
			)
		}
	}

	add("preinstall", func(v *BDep) bool {
		return v.PreInstall
	})

	add("vminstall", func(v *BDep) bool {
		return v.VMInstall
	})

	add("runscripts", func(v *BDep) bool {
		return v.RunScripts
	})

	if kiwiMode != "" {
		add("noinstall", func(v *BDep) bool {
			return v.NoInstall
		})

		add("installonly", func(v *BDep) bool {
			return v.InstallOnly
		})
	}

	writeFile(
		b.env.rpmList,
		[]byte(strings.Join(append(rpmList, "\n"), "\n")),
	)

	return nil
}
