package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

// Context to store process out in memory buffer
type memContext struct {
	nameGenerator newNameGen
	mux           *sync.Mutex
	file          string
	lines         []string
}

// Create new memory context. NameGen is a function which generates names for files
// where output is stored.
func newMemContext(nameGen newNameGen) *memContext {
	return &memContext{mux: &sync.Mutex{}, file: nameGen(), nameGenerator: nameGen}
}

// Add line to context
func (o *memContext) writeLine(line string) (err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	o.lines = append(o.lines, line)
	return
}

// Commits buffer to file and reset context
func (o *memContext) commit() (fileName string, err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	if len(o.lines) == 0 {
		return
	}
	fileName = o.nameGenerator()
	if ioutil.WriteFile(fileName, []byte(strings.Join(o.lines, "\n")), os.ModePerm); err != nil {
		return "", o.unsavedLinesError(fileName, o.lines)
	}
	log.Printf("%d lines written to file %s\n", len(o.lines), fileName)
	o.lines = nil
	return
}

func (o *memContext) contextName() string {
	return "In memory"
}

// Error contains list of lines which not been stored in file
func (o *memContext) unsavedLinesError(fileName string, lines []string) (err error) {
	msg := fmt.Sprintf("Write to file error (%s)\nUnsaved lines:\n%s\n", fileName, strings.Join(lines, "\n"))
	return errors.New(msg)
}
