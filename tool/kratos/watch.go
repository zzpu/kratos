package main

import (
	"bytes"
	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli/v2"
	"github.com/zzpu/kratos/tool/kratos/colors"
	yeeLogger "github.com/zzpu/kratos/tool/kratos/logger"
	"github.com/zzpu/kratos/tool/kratos/utils"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	cmd                 *exec.Cmd
	state               sync.Mutex
	eventTime           = make(map[string]int64)
	scheduleTime        time.Time
	watchExts           = []string{".go"}
	watchExtsStatic     = []string{".html", ".tpl", ".js", ".css"}
	ignoredFilesRegExps = []string{
		`.#(\w+).go`,
		`.(\w+).go.swp`,
		`(\w+).go~`,
		`(\w+).tmp`,
		`commentsRouter_controllers.go`,
	}
)

// NewWatcher starts an fsnotify Watcher on the specified paths
func NewWatcher(c *cli.Context, paths []string, files []string, isgenerate bool) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		yeeLogger.Log.Fatalf("Failed to create watcher: %s", err)
	}

	go func() {
		for {
			select {
			case e := <-watcher.Events:
				isBuild := true

				//if ifStaticFile(e.Name) && config.Conf.EnableReload {
				//	sendReload(e.String())
				//	continue
				//}
				// Skip ignored files
				if shouldIgnoreFile(e.Name) {
					continue
				}
				if !shouldWatchFileWithExtension(e.Name) {
					continue
				}

				mt := utils.GetFileModTime(e.Name)
				if t := eventTime[e.Name]; mt == t {
					yeeLogger.Log.Hintf(colors.Bold("Skipping: ")+"%s", e.String())
					isBuild = false
				}

				eventTime[e.Name] = mt

				if isBuild {
					yeeLogger.Log.Hintf("Event fired: %s", e)
					go func() {
						// Wait 1s before autobuild until there is no file change.
						scheduleTime = time.Now().Add(1 * time.Second)
						time.Sleep(time.Until(scheduleTime))
						AutoBuild(c, files, isgenerate)

						//if config.Conf.EnableReload {
						//	// Wait 100ms more before refreshing the browser
						//	time.Sleep(100 * time.Millisecond)
						//	sendReload(e.String())
						//}						//if config.Conf.EnableReload {
						//	// Wait 100ms more before refreshing the browser
						//	time.Sleep(100 * time.Millisecond)
						//	sendReload(e.String())
						//}
					}()
				}
			case err := <-watcher.Errors:
				yeeLogger.Log.Warnf("Watcher error: %s", err.Error()) // No need to exit here
			}
		}
	}()

	yeeLogger.Log.Info("Initializing watcher...")
	for _, path := range paths {
		yeeLogger.Log.Hintf(colors.Bold("Watching: ")+"%s", path)
		err = watcher.Add(path)
		if err != nil {
			yeeLogger.Log.Fatalf("Failed to watch directory: %s", err)
		}
	}
}

// AutoBuild builds the specified set of files
func AutoBuild(c *cli.Context, files []string, isgenerate bool) {
	state.Lock()
	defer state.Unlock()

	os.Chdir(currpath)

	cmdName := "go"

	var (
		err    error
		stderr bytes.Buffer
		base   string
	)

	base, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	appName := appname

	// For applications use full import path like "github.com/.../.."
	// are able to use "go install" to reduce build time.
	if goInstall {
		dir := buildDir(base, "cmd", 5)
		icmd := exec.Command(cmdName, "install", "-v")
		icmd.Dir = dir
		icmd.Stdout = os.Stdout
		icmd.Stderr = os.Stderr
		icmd.Env = append(os.Environ(), "GOGC=off")
		icmd.Run()
	}

	if isgenerate {
		yeeLogger.Log.Info("Generating the docs...")
		icmd := exec.Command("bee", "generate", "docs")
		icmd.Env = append(os.Environ(), "GOGC=off")
		err = icmd.Run()
		if err != nil {
			utils.Notify("", "Failed to generate the docs.")
			yeeLogger.Log.Errorf("Failed to generate the docs.")
			return
		}
		yeeLogger.Log.Success("Docs generated!")
	}

	if err == nil {

		if runtime.GOOS == "windows" {
			appName += ".exe"
		}

		args := []string{"build"}
		args = append(args, "-o", appName)
		if buildTags != "" {
			args = append(args, "-tags", buildTags)
		}
		dir := buildDir(base, "cmd", 5)
		args = append(args, files...)

		bcmd := exec.Command(cmdName, args...)
		bcmd.Dir = dir
		bcmd.Env = append(os.Environ(), "GOGC=off")
		bcmd.Stderr = &stderr
		err = bcmd.Run()
		if err != nil {
			utils.Notify(stderr.String(), "Build Failed")
			yeeLogger.Log.Errorf("Failed to build the application: %s", stderr.String())
			return
		}
	}

	yeeLogger.Log.Info("Built Successfully!")
	Restart(c, appName)
}

// Kill kills the running command process
func Kill(c *cli.Context) {
	defer func() {
		if e := recover(); e != nil {
			yeeLogger.Log.Errorf("Kill recover: %s", e)
		}
	}()
	if cmd != nil && cmd.Process != nil {
		// Windows does not support Interrupt
		if runtime.GOOS == "windows" {
			cmd.Process.Signal(os.Kill)
		} else {
			cmd.Process.Signal(os.Interrupt)
		}

		ch := make(chan struct{}, 1)
		go func() {
			cmd.Wait()
			ch <- struct{}{}
		}()

		select {
		case <-ch:
			return
		case <-time.After(10 * time.Second):
			yeeLogger.Log.Info("Timeout. Force kill cmd process")
			err := cmd.Process.Kill()
			if err != nil {
				yeeLogger.Log.Errorf("Error while killing cmd process: %s", err)
			}
			return
		}
	}
}

// Restart kills the running command process and starts it again
func Restart(c *cli.Context, appname string) {
	yeeLogger.Log.Debugf("Kill running process", utils.FILE(), utils.LINE())
	Kill(c)
	go Start(c, appname)
}

// Start starts the command process
func Start(c *cli.Context, appname string) {
	yeeLogger.Log.Infof("Restarting '%s'...", appname)
	if !strings.Contains(appname, "./") {
		appname = "./" + appname
	}

	base, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	cmd = exec.Command(appname)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runargs != "" {
		r := regexp.MustCompile("'.+'|\".+\"|\\S+")
		m := r.FindAllString(runargs, -1)
		cmd.Args = append([]string{appname}, m...)
	} else {
		dir := buildDir(base, "cmd", 5)
		conf := path.Join(filepath.Dir(dir), "configs")
		args := append([]string{appname, "-conf", conf}, c.Args().Slice()...)
		cmd.Dir = dir
		cmd.Args = args
	}
	go cmd.Run()
	yeeLogger.Log.Successf("'%s' is running...", appname)
	started <- true
}

func ifStaticFile(filename string) bool {
	for _, s := range watchExtsStatic {
		if strings.HasSuffix(filename, s) {
			return true
		}
	}
	return false
}

// shouldIgnoreFile ignores filenames generated by Emacs, Vim or SublimeText.
// It returns true if the file should be ignored, false otherwise.
func shouldIgnoreFile(filename string) bool {
	for _, regex := range ignoredFilesRegExps {
		r, err := regexp.Compile(regex)
		if err != nil {
			yeeLogger.Log.Fatalf("Could not compile regular expression: %s", err)
		}
		if r.MatchString(filename) {
			return true
		}
		continue
	}
	return false
}

// shouldWatchFileWithExtension returns true if the name of the file
// hash a suffix that should be watched.
func shouldWatchFileWithExtension(name string) bool {
	for _, s := range watchExts {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}
