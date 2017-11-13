package main

type newNameGen func() string

type context interface {
	writeLine(string) error
	commit(newNameGen) (string, error)
	contextName() string
}
