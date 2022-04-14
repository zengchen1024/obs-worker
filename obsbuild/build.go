package obsbuild

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/opensourceways/obs-worker/sdk/config"
	"github.com/opensourceways/obs-worker/sdk/filereceiver"
	"github.com/opensourceways/obs-worker/sdk/source"
	"github.com/opensourceways/obs-worker/sdk/sslcert"
	"github.com/opensourceways/obs-worker/utils"
)

type options struct {
	buildroot string
	statedir  string

	vmTmpfsMode    bool
	vmDiskRootSize int

	srcServer string
	cacheDir  string
	cacheSize int

	workerId string
}

type buildEnv struct {
	srcdir    string
	pkgdir    string
	oldpkgdir string
	meta      string
	packages  string
}

type buildOnce struct {
	opts *options
	info *BuildInfo
	env  buildEnv

	hc utils.HttpClient

	meta []string
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
	}

	var exe utils.Executor

	if opt.vmTmpfsMode {
		out, err := exe.Run(
			"mount", "-t", "tmpfs",
			fmt.Sprintf("-osize=%dM", opt.vmDiskRootSize),
			"none", buildroot,
		)
		if err != nil {
			return fmt.Errorf("%v %v", err, string(out))
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
	if err := os.Mkdir(b.env.srcdir, os.FileMode(0777)); err != nil {
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
		if err := os.MkdirAll(saveTo, os.FileMode(0777)); err != nil {
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
		return os.WriteFile(filepath.Join(b.env.srcdir, "_projectcert.crt"), []byte(v), 0644)
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
		filepath.Join(b.getBuildRoot(), ".build.config"),
	)
}

func genMetaLine(md5, pkg string) string {
	return fmt.Sprintf("%s  %s", md5, pkg)
}

func splitMetaLine(line string) (string, string) {
	v := strings.Split(line, "  ")
	if len(v) == 2 {
		return v[0], v[1]
	}

	return line, ""
}

func isFileExist(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

func isEmptyFile(f string) (bool, error) {
	v, err := os.Stat(f)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}

		return false, err
	}

	if v.IsDir() {
		return false, fmt.Errorf("%s is a directory", f)
	}

	return v.Size() == 0, nil
}

func readFileLineByLine(filename string, handle func(string) bool) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if b := handle(scanner.Text()); b {
			break
		}
	}
}
