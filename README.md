# monitor_process_out

Reads process output in real time and store results into files every N secons. Allows to run additionals process on saved files.
Originally created to monitor output from `kafka-console-consumer.sh`

## Start
```
$ monitor_process_out -cfg CONFIG_FILE
```

CONFIG_FILE - JSON config file (defualt 'config.json')

Structure:

```
{
    "write_interval": 10,
    "command": "ls",
    "command_args": ["-la"],
    "out_dir": "/out",
    "log_dir": "/log",
    "out_filename_pattern": "out_{{.Timestamp}}.txt",
    "out_process_script": "python",
    "out_process_script_params": ["process_out.py", "-additinal_flag"]
}

```

`write_interval` - interval in seconds, how often data will be save to file 

`command` - process which output will be monitored

`command_args` - process arguments

`out_dir` - directory path where files will be stored

`log_dir` - directory path where logs will be stored

`out_filename_pattern` - name of the out file {{.Timestamp}} will be replaced by timestamp in format YYYYMMMDDDhhmmss[nanoseconds]`

`out_process_script` - optional script, which will called after each file save. As a parameter will be passed path to the saved file.

`out_process_script_params` - additionals parameters for out_process_script
