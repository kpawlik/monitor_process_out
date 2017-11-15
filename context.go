package main

type newNameGen func() string

type context interface {
	// write line from process out to context buffer
	writeLine(string) error
	// commit buffer to file
	commit() (string, error)
	// name of the context
	contextName() string
}
