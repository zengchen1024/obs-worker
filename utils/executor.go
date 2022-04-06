package utils

// Executor execute commands
type Executor interface {
	Run(args ...string) ([]byte, error)
}
