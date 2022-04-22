package build

type buildPkg struct {
	*buildHelper

	needOBSPackage bool
}

func (b *buildPkg) do() error {
	return nil
}
