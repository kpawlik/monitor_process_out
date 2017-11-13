package main

type newNameGen func() string

type context interface {
	writeLine(string) error
	commit() (string, error)
	contextName() string
}
