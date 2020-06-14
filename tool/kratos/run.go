package main

import (
	yeeLogger "github.com/zzpu/kratos/tool/kratos/logger"
	"github.com/zzpu/kratos/tool/kratos/utils"
	"io/ioutil"
	"os"
	"runtime"

	//"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

var (
	// Channel to signal an Exit
	exit      chan bool
	mainFiles utils.ListOpts
	// The flags list of the paths excluded from watching
	excludedPaths utils.StrFlags
	// Pass through to -tags arg of "go build"
	buildTags string
	// Application path
	currpath string
	// Application name
	appname string

	started = make(chan bool)
	runargs string

	goInstall = true

	vendorWatch = false
)

func runAction(c *cli.Context) error {
	appPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	var paths []string
	readAppDirectories(appPath, &paths)

	files := []string{}
	for _, arg := range mainFiles {
		if len(arg) > 0 {
			files = append(files, arg)
		}
	}

	appname = path.Base(appPath)

	NewWatcher(c, paths, files, false)
	AutoBuild(c, files, false)

	//dir := buildDir(appPath, "cmd", 5)
	//conf := path.Join(filepath.Dir(dir), "configs")
	//args := append([]string{"run", "main.go", "-conf", conf}, c.Args().Slice()...)
	//cmd := exec.Command("go", args...)
	//cmd.Dir = dir
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	//if err := cmd.Run(); err != nil {
	//	panic(err)
	//}

	for {
		<-exit
		runtime.Goexit()
	}
	return nil
}

func readAppDirectories(directory string, paths *[]string) {
	fileInfos, err := ioutil.ReadDir(directory)
	if err != nil {
		return
	}

	useDirectory := false
	for _, fileInfo := range fileInfos {
		if strings.HasSuffix(fileInfo.Name(), "docs") {
			continue
		}
		if strings.HasSuffix(fileInfo.Name(), "swagger") {
			continue
		}

		if !vendorWatch && strings.HasSuffix(fileInfo.Name(), "vendor") {
			continue
		}

		if isExcluded(path.Join(directory, fileInfo.Name())) {
			continue
		}

		if fileInfo.IsDir() && fileInfo.Name()[0] != '.' {
			readAppDirectories(directory+"/"+fileInfo.Name(), paths)
			continue
		}

		if useDirectory {
			continue
		}

		if path.Ext(fileInfo.Name()) == ".go" {
			*paths = append(*paths, directory)
			useDirectory = true
		}
	}
}

// If a file is excluded
func isExcluded(filePath string) bool {
	for _, p := range excludedPaths {
		absP, err := filepath.Abs(p)
		if err != nil {
			yeeLogger.Log.Errorf("Cannot get absolute path of '%s'", p)
			continue
		}
		absFilePath, err := filepath.Abs(filePath)
		if err != nil {
			yeeLogger.Log.Errorf("Cannot get absolute path of '%s'", filePath)
			break
		}
		if strings.HasPrefix(absFilePath, absP) {
			yeeLogger.Log.Errorf("'%s' is not being watched", filePath)
			return true
		}
	}
	return false
}
