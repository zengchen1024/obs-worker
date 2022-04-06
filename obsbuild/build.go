package obsbuild

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opensourceways/obs-worker/utils"
)

type options struct {
	buildroot string
	statedir  string

	vmTmpfsMode    bool
	vmDiskRootSize int
}

type buildEnv struct {
	srcdir    string
	pkgdir    string
	oldpkgdir string
	meta      string
	packages  string
}

type buildOnce struct {
	info *BuildInfo
	env  buildEnv

	meta []string
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
	filename := filepath.Join(b.env.srcdir, b.info.File)

	needsBinaries := false
	needSSLCert := false
	needAppxSSLCert := false
	needOBSPackage := false

	mode := b.info.getkiwimode()

	if mode == "" {
		re0 := regexp.MustCompile("^#\\s*needsbinariesforbuild\\s*$")
		re1 := regexp.MustCompile("^#\\s*needssslcertforbuild\\s*$|^Obs:\\s*needssslcertforbuild\\s*$")
		re2 := regexp.MustCompile("^(?:#|Obs:)\\s*needsappxsslcertforbuild\\s*$")

		readFileLineByLine(filename, func(line string) bool {
			b := []byte(line)
			needsBinaries = re0.Match(b)
			needSSLCert = re1.Match(b)
			needAppxSSLCert = re2.Match(b)
			needOBSPackage = strings.Contains(line, "@OBS_PACKAGE@")

			return needsBinaries && needSSLCert && needAppxSSLCert && needOBSPackage
		})
	}

	return nil, nil
}

func genMetaLine(md5, pkg string) string {
	return fmt.Sprintf("%s  %s", md5, pkg)
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
