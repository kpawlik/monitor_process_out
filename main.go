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
	"runtime"
	"strings"
	"sync"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const logFileName = "process_out_monitor.log"

var (
	configFile   string
	line         string
	err          error
	linesChan    chan string
	lines        = []string{}
	mux          = &sync.Mutex{}
	cfg          *config
	nameTemplate *template.Template
	logFilePath  string
	wg           *sync.WaitGroup
	mem          runtime.MemStats
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
func runConvertScript(fileName string, cfg *config) {
	params := append(cfg.OutScriptParams, fileName)
	cmd := exec.Command(cfg.OutScript, params...)
	if err = cmd.Run(); err != nil {
		log.Printf("ERROR: Start convert script failed: %s\nProcess: %s\n",
			err, strings.Join(cmd.Args, " "))
	}
}

// Write process out to file
func writeRawOut(lines []string, fileName string, cfg *config) (err error) {
	if err = ioutil.WriteFile(fileName, []byte(strings.Join(lines, "\n")), os.ModePerm); err != nil {
		return
	}
	log.Printf("%d records stored in %s", len(lines), fileName)
	return
}

// Write chunk of process out
func writeChunk(lines []string, cfg *config) (fileName string, err error) {
	if len(lines) == 0 {
		return
	}
	buff := &bytes.Buffer{}
	timestamp := fmt.Sprintf("%s%d", time.Now().Format("20060102150405"), time.Now().Nanosecond())
	nameTemplate.Execute(buff, struct{ Timestamp string }{Timestamp: timestamp})
	fileName = path.Join(cfg.OutDir, buff.String())
	if err = writeRawOut(lines, fileName, cfg); err != nil {
		log.Printf("ERROR: Write file error: %s (%s)\n", err, fileName)
		log.Printf("Lines not saved:\n %s\n", strings.Join(lines, "\n"))
	}
	return
}

// Clean up. Write rest of process out to.
func cleanup(cfg *config, cmd *exec.Cmd) {
	var (
		err      error
		fileName string
	)
	wg.Wait()
	//make sure that all was written
	mux.Lock()
	defer mux.Unlock()
	if fileName, err = writeChunk(lines, cfg); err != nil {
		log.Printf("Error write file: %s\n", err)
		return
	}
	runConvertScript(fileName, cfg)
}

func main() {
	wg = &sync.WaitGroup{}
	linesChan = make(chan string)
	cfg := readConfig()
	logFilePath = path.Join(cfg.LogDir, logFileName)
	log.SetOutput(&lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    20, // megabytes
		MaxBackups: 20,
	})

	nameTemplate = template.Must(template.New("fileName").Parse(cfg.OutName))

	log.Printf("Run and monitor command: %s %s\n", cfg.Commad, strings.Join(cfg.CommadArgs, " "))
	log.Printf("Write interval: %d seconds\n", cfg.WriteInterval)
	log.Printf("Output dir: %s\n", cfg.OutDir)
	log.Printf("Log file: %s\n", logFilePath)
	log.Printf("Out process script: %s %s\n", cfg.OutScript, strings.Join(cfg.OutScriptParams, " "))
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
				mux.Lock()
				lines = append(lines, line)
				mux.Unlock()
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
	// write process out every tick
	go func(ticker *time.Ticker, cfg *config) {
		for _ = range ticker.C {
			mux.Lock()
			outLines := make([]string, len(lines))
			copy(outLines, lines)
			wg.Add(1)
			go func(lines []string, cfg *config) {
				defer wg.Done()
				if fileName, err := writeChunk(lines, cfg); err == nil && cfg.OutScript != "" {
					runConvertScript(fileName, cfg)
				}
			}(outLines, cfg)
			lines = nil
			mux.Unlock()
		}
	}(ticker, cfg)
	// wait until process end
	if err = cmd.Wait(); err != nil {
		log.Printf("Error for commad %s. %s\n", cfg.Commad, err)
	}
	cleanup(cfg, cmd)
}
