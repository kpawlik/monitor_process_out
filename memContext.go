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

type memContext struct {
	nameGenerator newNameGen
	mux           *sync.Mutex
	file          string
	lines         []string
}

func newMemContext(nameGen newNameGen) *memContext {
	return &memContext{mux: &sync.Mutex{}, file: nameGen(), nameGenerator: nameGen}
}

func (o *memContext) writeLine(line string) (err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	o.lines = append(o.lines, line)
	return
}

func (o *memContext) commit() (fileName string, err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	outLines := make([]string, len(o.lines))
	copy(outLines, o.lines)
	fileName = o.file
	if len(outLines) == 0 {
		fileName = ""
		return
	}
	if ioutil.WriteFile(fileName, []byte(strings.Join(outLines, "\n")), os.ModePerm); err != nil {
		return "", o.unsavedLinesError(fileName, outLines)
	}
	log.Printf("%d lines written to file %s\n", len(outLines), fileName)
	o.file = o.nameGenerator()
	o.lines = nil
	return
}

func (o *memContext) contextName() string {
	return "In memory"
}

func (o *memContext) unsavedLinesError(fileName string, lines []string) (err error) {
	msg := fmt.Sprintf("Write to file error (%s)\nUnsaved lines:\n%s\n", fileName, strings.Join(lines, "\n"))
	return errors.New(msg)
}
