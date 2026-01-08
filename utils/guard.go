package utils

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func GuardStart(proc_handler func(), time_out time.Duration, err_handle func(err error)) {

	var args []string
	fork := false
	ret := false

	for i := 1; i < len(os.Args); i++ {
		if strings.HasPrefix(os.Args[i], "--fork") {
			ret = true
			continue
		}
		args = append(args, os.Args[i])
	}

	if !ret {
		args = append(args, "--fork")
	} else {
		fork = true
	}

	time.Sleep(time_out)

	if !fork {
		log.Println("父进程开始")
		for {
			args = append(args, "--local_config")
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = os.Environ()
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			var err error

			if err = cmd.Start(); err != nil {
				log.Printf("failed to run command: %v\n", err)
			} else if err = cmd.Wait(); err != nil {
				log.Printf("failed to wait command: %v\n", err)
			}

			err_handle(err)
			time.Sleep(time_out)
		}
	}

	log.Println("子进程开始")
	proc_handler()
}
