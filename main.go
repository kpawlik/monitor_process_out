package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const logFileName = "process_out_monitor.log"

var (
	configFile    string
	err           error
	linesChan     chan string
	fileNamesChan chan string
	cfg           *config
	nameTemplate  *template.Template
	logFilePath   string
	wg            *sync.WaitGroup
)

type config struct {
	WriteInterval   int64    `json:"write_interval"`
	Commad          string   `json:"command"`
	CommadArgs      []string `json:"command_args"`
	OutDir          string   `json:"out_dir"`
	LogDir          string   `json:"log_dir"`
	OutName         string   `json:"out_filename_pattern"`
	OutScript       string   `json:"out_process_script"`
	OutScriptParams []string `json:"out_process_script_params"`
	Context         string   `json:"context"`
}

// Initialize/parse flags and config
func init() {
	var (
		help bool
	)
	flag.StringVar(&configFile, "cfg", "config.json", "Configuration file")
	flag.BoolVar(&help, "h", false, "Print help")
	flag.Parse()
	if help {
		flag.PrintDefaults()
		os.Exit(0)
	}
	cfg = readConfig()

}

// Read configuration from file
func readConfig() *config {
	var (
		err     error
		cfgBuff []byte
	)
	if cfgBuff, err = ioutil.ReadFile(configFile); err != nil {
		fmt.Printf("Error opening config file. %s, %s\n", configFile, err)
		os.Exit(1)
	}
	cfg := &config{}
	if err = json.Unmarshal(cfgBuff, cfg); err != nil {
		fmt.Printf("Error reading configuration from JSON. %s, %s\n", configFile, err)
		os.Exit(1)
	}
	return cfg
}

// Run convert script and check process status
func runConvertScript(fileNamesChan chan string, cfg *config, wg *sync.WaitGroup) {
	for {
		fileName := <-fileNamesChan
		if len(fileName) == 0 {
			continue
		}
		params := append(cfg.OutScriptParams, fileName)
		cmd := exec.Command(cfg.OutScript, params...)
		if err = cmd.Run(); err != nil {
			log.Printf("ERROR: Start convert script failed: %s\nProcess: %s\n",
				err, strings.Join(cmd.Args, " "))
		}
		wg.Done()
	}
}

func newFileName(cfg *config) string {
	buff := &bytes.Buffer{}
	timestamp := fmt.Sprintf("%s%d", time.Now().Format("20060102150405"), time.Now().Nanosecond())
	nameTemplate.Execute(buff, struct{ Timestamp string }{Timestamp: timestamp})
	return path.Join(cfg.OutDir, buff.String())
}

func newContext(cfg *config) (ctx context) {
	fileName := newFileName(cfg)
	switch cfg.Context {
	case "file":
		ctx = newFileContext(fileName)
	default:
		ctx = newMemContext(fileName)
	}
	return
}

func nameGenerator(cfg *config) newNameGen {
	f := func() string {
		return newFileName(cfg)
	}
	return f
}

func main() {
	var (
		err      error
		fileName string
	)
	wg = &sync.WaitGroup{}
	linesChan = make(chan string)
	fileNamesChan = make(chan string, 10)
	cfg := readConfig()
	logFilePath = path.Join(cfg.LogDir, logFileName)
	log.SetOutput(&lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    20, // megabytes
		MaxBackups: 20,
	})
	nameTemplate = template.Must(template.New("fileName").Parse(cfg.OutName))
	ctx := newContext(cfg)

	nameGen := nameGenerator(cfg)

	log.Printf("Run and monitor command: %s %s\n", cfg.Commad, strings.Join(cfg.CommadArgs, " "))
	log.Printf("Write interval: %d seconds\n", cfg.WriteInterval)
	log.Printf("Output dir: %s\n", cfg.OutDir)
	log.Printf("Log file: %s\n", logFilePath)
	log.Printf("Out process script: %s %s\n", cfg.OutScript, strings.Join(cfg.OutScriptParams, " "))
	log.Printf("Context: %s\n", ctx.contextName())

	cmd := exec.Command(cfg.Commad, cfg.CommadArgs...)
	ticker := time.NewTicker(time.Second * time.Duration(cfg.WriteInterval))
	stdout, _ := cmd.StdoutPipe()
	scanner := bufio.NewScanner(stdout)
	cmd.Start()
	// append lines from process out to lines
	go func() {
		for {
			select {
			case line := <-linesChan:
				ctx.writeLine(line)
			}
		}
	}()
	// scan process out
	go func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			m := scanner.Text()
			linesChan <- m
		}
	}(scanner)
	go runConvertScript(fileNamesChan, cfg, wg)
	// write process out every tick
	go func(ticker *time.Ticker, cfg *config, ctx context) {
		var (
			fileName string
			err      error
		)
		for _ = range ticker.C {
			if fileName, err = ctx.commit(nameGen); err != nil {
				log.Printf("ERROR: %s\n", err)
				return
			}
			wg.Add(1)
			fileNamesChan <- fileName
		}
	}(ticker, cfg, ctx)
	// wait until process end
	if err = cmd.Wait(); err != nil {
		log.Printf("Error for commad %s. %s\n", cfg.Commad, err)
	}
	if fileName, err = ctx.commit(nameGen); err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}
	wg.Add(1)
	fileNamesChan <- fileName
	wg.Wait()
}
