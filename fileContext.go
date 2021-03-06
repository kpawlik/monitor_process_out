package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// Context to store process out in temporary file
type fileContext struct {
	nameGenerator newNameGen
	f             *os.File
	mux           *sync.Mutex
	counter       int
	lines         []string
	buffSize      int
}

// Create new file context. NameGen is a function which generates names for files
// where output is stored.
func newFileContext(nameGen newNameGen) *fileContext {
	var (
		err  error
		file *os.File
	)
	fileName := nameGen()
	fileName = fmt.Sprintf("%s.tmp", fileName)
	if file, err = os.Create(fileName); err != nil {
		panic(err)
	}
	return &fileContext{mux: &sync.Mutex{}, f: file, nameGenerator: nameGen, buffSize: 1000}
}

// Reset lines buffer, counter and create new temporary file
func (o *fileContext) reset(fileName string) (err error) {
	o.lines = nil
	o.counter = 0
	fileName = fmt.Sprintf("%s.tmp", fileName)
	o.f, err = os.Create(fileName)
	return
}

// Add line to context
func (o *fileContext) writeLine(line string) (err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	o.lines = append(o.lines, line)
	o.counter++
	if len(o.lines) == o.buffSize {
		_, err = o.f.WriteString(fmt.Sprintf("%s", strings.Join(o.lines, "\n")))
		o.f.Sync()
		o.lines = nil
	}
	return
}

// Commits buffer to file and reset context
func (o *fileContext) commit() (fileName string, err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	if len(o.lines) > 0 {
		_, err = o.f.WriteString(fmt.Sprintf("%s", strings.Join(o.lines, "\n")))
		o.f.Sync()
		o.lines = nil
	}
	o.f.Close()
	tmpName := o.f.Name()
	if o.counter == 0 {
		err = os.Remove(tmpName)
	} else {
		fileName = o.nameGenerator()
		if err = os.Rename(tmpName, fileName); err != nil {
			return
		}
		log.Printf("%d lines written to file %s\n", o.counter, fileName)
	}
	err = o.reset(o.nameGenerator())
	return
}

func (o *fileContext) contextName() string {
	return "Disc file"
}
