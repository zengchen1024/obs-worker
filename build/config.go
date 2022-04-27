package build

import (
	"fmt"
	"strings"

	"github.com/huaweicloud/golangsdk"
	"github.com/zengchen1024/obs-worker/utils"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	VMZVM       = "zvm"
	VMEmulator  = "emulator"
	VMOpenstack = "openstack"
)

type VMCommon struct {
	Kernel        string `json:"kernel"`
	Initrd        string `json:"initrd"`
	CustomOption  string `json:"custom_option"`
	EnableConsole bool   `json:"enable_console"`
}

type VMDisk struct {
	RootSize int `json:"root_size"`
	SwapSize int `json:"swap_size"`
}

type VMInfo struct {
	VMCommon

	VMDisk

	Device string `json:"device"`
	Swap   string `json:"swap"`
	Memory int    `json:"memory"`
}

type Openstack struct {
	VMInfo

	Worker string `json:"worker"`
	Server string `json:"server" required:"true"`
	Flavor string `json:"flavor" required:"true"`
}

type VMDiskExt struct {
	FileSystem   string `json:"file_system"`
	MountOptions string `json:"mount_options"`
	Clean        bool   `json:"clean"`
}

type VMProp struct {
	VMInfo

	VMDiskExt

	Tmpfs string `json:"tmpfs"`

	Hugetlbfs string `json:"hugetlbfs"`
}

type ZVM struct {
	VMCommon

	VMDiskExt

	Worker    string `json:"worker"`
	WorkerNum int    `json:"worker_num" required:"true"`
}

type OtherVM struct {
	VMProp

	VMType string `json:"vm_type" required:"true"`
}

type Emulator struct {
	VMProp

	Script string `json:"script" required:"true"`
}

type ObsBuild struct {
	Jobs    int `json:"jobs"`
	Threads int `json:"threads"`

	JustBuild      bool `json:"just_build"`
	NoWorkerUpdate bool `json:"no_worker_update"`
	NoBuildUpdate  bool `json:"no_build_update"`
}

type VM struct {
	Openstack *Openstack `json:"openstack,omitempty"`

	Emulator *Emulator `json:"emulator,omitempty"`

	OtherVM *OtherVM `json:"vm_other,omitempty"`

	ZVM *ZVM `json:"zvm,omitempty"`
}

func (vm *VM) validate() error {
	if vm == nil {
		return nil
	}

	m := []bool{
		vm.Openstack != nil,
		vm.Emulator != nil,
		vm.OtherVM != nil,
		vm.ZVM != nil,
	}

	n := 0
	for _, b := range m {
		if b {
			n++
		}
	}

	if n > 1 {
		return fmt.Errorf("can't set %d different vm types", n)
	}

	return nil
}

type Config struct {
	Id    string `json:"id" required:"true"`
	Owner string `json:"owner"`

	HostLabel []string `json:"host_label" required:"true"`
	HostArch  string   `json:"host_arch" required:"true"`
	HostCheck string   `json:"host_check"`

	BuildRoot string `json:"build_root" required:"true"`
	StateDir  string `json:"state_dir" required:"true"`

	RepoServers []string `json:"repo_servers" required:"true"`
	SrcServer   string   `json:"src_server" required:"true"`

	CacheDir  string `json:"cache_dir"`
	CacheSize int    `json:"cache_size"`

	BinaryProxy string `json:"binary_proxy"`

	LocalKiwi      string `json:"local_kiwi"`
	HardStatus     bool   `json:"hard_status"`
	CleanupChroot  bool   `json:"cleanup_chroot"`
	WipeAfterBuild bool   `json:"wipe_after_build"`

	ObsBuild

	*VM
}

func (c *Config) SetDefault() error {
	if c.HostArch == "" {
		out, err := utils.RunCmd("uname", "-m")
		if err != nil {
			return fmt.Errorf("get host arch failed, err: %v, %v", string(out), err)
		}
		c.HostArch = strings.TrimSuffix(string(out), "\n")
	}

	if c.Jobs == 0 {
		c.Jobs = 1
	}

	if info := c.GetVMInfo(); info != nil {
		if info.Device == "" {
			info.Device = c.BuildRoot + ".img"
		}

		if info.Swap == "" {
			info.Swap = c.BuildRoot + ".swap"
		}
	}

	return nil
}

func (c *Config) Validate() error {
	v := sets.NewString(getSupportArch()...)
	if a := c.HostArch; !(v.Has(a) || (a == "local" && c.LocalKiwi != "")) {
		return fmt.Errorf("unsupport host arch:%s", a)
	}

	if (c.CacheDir != "" && c.CacheSize == 0) || (c.CacheSize != 0 && c.CacheDir == "") {
		return fmt.Errorf("cache dir and size must be set at same time")
	}

	c.CacheSize = c.CacheSize << 20

	if err := c.VM.validate(); err != nil {
		return err
	}

	_, err := golangsdk.BuildRequestBody(c, "")
	return err
}

func (c *Config) GetVMType() string {
	if c.VM == nil {
		return ""
	}

	if c.OtherVM != nil {
		return c.OtherVM.VMType
	}

	if c.isVMEmulator() {
		return VMEmulator
	}

	if c.IsVMOpenstack() {
		return VMOpenstack
	}

	if c.isVMZVM() {
		return VMZVM
	}

	return ""
}

func (c *Config) isVMEmulator() bool {
	return c.Emulator != nil
}

func (c *Config) IsVMOpenstack() bool {
	return c.Openstack != nil
}

func (c *Config) isVMZVM() bool {
	return c.ZVM != nil
}

func (c *Config) GetVMInfo() *VMInfo {
	if c.VM == nil || c.isVMZVM() {
		return nil
	}

	if c.IsVMOpenstack() {
		return &c.Openstack.VMInfo
	}

	if c.isVMEmulator() {
		return &c.Emulator.VMInfo
	}

	return &c.OtherVM.VMInfo
}

func getSupportArch() []string {
	return []string{
		"aarch64",
		"aarch64_ilp32",
		"armv4l",
		"armv5l",
		"armv6l",
		"armv7l",
		"armv8l",
		"sh4",
		"i586",
		"i686",
		"x86_64",
		"k1om",
		"parisc",
		"parisc64",
		"ppc",
		"ppc64",
		"ppc64p7",
		"ppc64le",
		"ia64",
		"riscv64",
		"s390",
		"s390x",
		"sparc",
		"sparc64",
		"mips",
		"mips64",
		"m68k",
	}
}
