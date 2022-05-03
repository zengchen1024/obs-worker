package build

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type nonModeBuid struct {
	buildHelper

	imageManager  preInstallImageManager
	binaryManager binaryManager
	binaryLoader  nonModeBinary
	report        buildReport
	out           buildInfoOut
	rpmlist       buildRpmlist
	stats         buildStats
	build         buildPkg
	sources       buildSources
	cache         cacheManager
	oldpkg        buildOldPackages

	needOBSPackage bool
}

func newNonModeBuild(cfg *Config, info *buildinfo.BuildInfo) (*nonModeBuid, error) {
	b := nonModeBuid{
		buildHelper: buildHelper{
			cfg:  cfg,
			info: BuildInfo{BuildInfo: *info},
		},
	}

	h := &b.buildHelper

	b.sources = buildSources{h}

	b.rpmlist = buildRpmlist{
		buildHelper: h,
	}

	b.build = buildPkg{
		buildHelper: h,
	}

	b.cache = cacheManager{
		buildHelper: h,
	}
	if err := b.cache.init(); err != nil {
		return nil, err
	}

	b.oldpkg = buildOldPackages{
		buildHelper:           h,
		handleDownloadDetails: b.stats.setBinaryDownloadDetail,
	}

	b.binaryManager = binaryManager{
		buildHelper:           h,
		cache:                 &b.cache,
		handleCacheHits:       b.stats.setCacheHit,
		handleDownloadDetails: b.stats.setBinaryDownloadDetail,
	}
	b.binaryManager.init()

	b.imageManager = preInstallImageManager{
		buildHelper:    h,
		cache:          &b.cache,
		handleRepoBins: b.binaryManager.setKnownBins,
	}

	b.binaryLoader = nonModeBinary{
		buildHelper:   h,
		imageManager:  &b.imageManager,
		binaryManager: &b.binaryManager,

		handleOutBDep:    b.out.setBdep,
		handleKiwiOrigin: b.report.setKiwiOrigin,

		handleImage: func(img *preInstallImage) {
			b.rpmlist.setPreInstallImage(img)
			b.stats.setPreInstallImage(img)
		},
	}

	return &b, nil
}

func (b *nonModeBuid) Do() error {
	if err := b.env.init(b.cfg); err != nil {
		return err
	}

	b.setBuildInfoOut()

	b.stats.recordDownloadStartTime()

	if err := b.fetchSources(); err != nil {
		return err
	}

	info := b.getBuildInfo()
	if !info.isNoUnchanged() && info.File != "preinstallimage" {
		if err := b.oldpkg.download(); err != nil {
			return err
		}
	}

	b.stats.recordDownloadTime()

	if err := b.downloadProjectConfig(); err != nil {
		return err
	}

	if err := b.rpmlist.generate(); err != nil {
		return err
	}

	if err := b.build.do(); err != nil {
		return err
	}

	dir := b.env.otherDir

	b.stats.do(dir)

	b.out.writeBuildEnv(dir)

	b.report.do(dir)

	return nil
}

func (b *nonModeBuid) setBuildInfoOut() {
	/*
	  if (!$kiwimode && !$followupmode && !$deltamode) {
	    $buildinfo->{'outbuildinfo'} = {
	      'project' => $projid,
	      'package' => $packid,
	      'repository' => $repoid,
	      'arch' => $arch,
	      'srcmd5' => $buildinfo->{'srcmd5'},
	      'verifymd5' => $buildinfo->{'verifymd5'} || $buildinfo->{'srcmd5'},
	      'bdep' => [],
	    };
	    for ('versrel', 'bcnt', 'release', 'module') {
	      $buildinfo->{'outbuildinfo'}->{$_} = $buildinfo->{$_} if defined $buildinfo->{$_};
	    }
	  }
	*/
}

func (b *nonModeBuid) fetchSources() error {
	s, err := b.sources.getSource()
	if err != nil {
		return err
	}

	metas := []string{s}

	needSSLCert, ignoreImage, _ := b.parseBuildFile()

	if needSSLCert {
		if err := b.sources.downloadSSLCert(); err != nil {
			return err
		}
	}

	if ignoreImage {
		b.report.needCollectOrigins = true
	}

	v, err := b.binaryLoader.getBinaries(!ignoreImage)
	if err != nil {
		return err
	}
	metas = append(metas, v...)

	return writeFile(b.env.meta, []byte(strings.Join(metas, "\n")+"\n"))
}

func (b *nonModeBuid) parseBuildFile() (
	needSSLCert bool,
	ignoreImage bool,
	needAppxSSLCert bool,
) {
	needOBSPackage := false

	re0 := regexp.MustCompile("^#\\s*needsbinariesforbuild\\s*$")
	re1 := regexp.MustCompile("^#\\s*needssslcertforbuild\\s*$|^Obs:\\s*needssslcertforbuild\\s*$")
	re2 := regexp.MustCompile("^(?:#|Obs:)\\s*needsappxsslcertforbuild\\s*$")

	filename := filepath.Join(b.getSrcdir(), b.getBuildInfo().File)

	readFileLineByLine(filename, func(line string) bool {
		bs := []byte(line)
		if re0.Match(bs) {
			ignoreImage = true
		}

		if re1.Match(bs) {
			needSSLCert = true
		}

		if re2.Match(bs) {
			needAppxSSLCert = true
		}

		if strings.Contains(line, "@OBS_PACKAGE@") {
			needOBSPackage = true
		}

		return ignoreImage && needSSLCert && needAppxSSLCert && needOBSPackage
	})

	if needOBSPackage {
		b.build.needOBSPackage = true
	}
	return
}
