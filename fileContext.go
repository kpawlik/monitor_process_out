package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type fileContext struct {
	f       *os.File
	mux     *sync.Mutex
	counter int
}

func newFileContext(fileName string) *fileContext {
	var (
		err  error
		file *os.File
	)
	fileName = fmt.Sprintf("%s.tmp", fileName)
	if file, err = os.Create(fileName); err != nil {
		panic(err)
	}
	return &fileContext{mux: &sync.Mutex{}, f: file}
}

func (o *fileContext) reset(fileName string) (err error) {
	o.counter = 0
	fileName = fmt.Sprintf("%s.tmp", fileName)
	o.f, err = os.Create(fileName)
	return
}
func (o *fileContext) writeLine(line string) (err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	_, err = o.f.WriteString(fmt.Sprintf("%s\n", line))
	o.f.Sync()
	o.counter++
	return
}

func (o *fileContext) commit(nameGen newNameGen) (fileName string, err error) {
	defer o.mux.Unlock()
	o.mux.Lock()
	o.f.Close()
	tmpName := o.f.Name()
	fileName = strings.TrimRight(tmpName, ".tmp")
	if err = os.Rename(tmpName, fileName); err != nil {
		return
	}
	log.Printf("%d lines written to file %s\n", o.counter, fileName)
	err = o.reset(nameGen())
	return
}

func (o *fileContext) contextName() string {
	return "Disc file"
}
