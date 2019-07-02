package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"

	"github.com/loganstone/kpick/conf"
	"github.com/loganstone/kpick/dir"
	"github.com/loganstone/kpick/file"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to `file`.")
	memprofile = flag.String("memprofile", "", "Write memory profile to `file`.")

	dirToSearch   = flag.String("d", conf.DefaultDir, "Directory to search.")
	fileExtToScan = flag.String("f", conf.DefaultFileExt, "File extension to scan.")
	skipPaths     = flag.String("s", conf.DefaultSkipPaths, "Directories to skip from search.(delimiter ',')")
	ignorePattern = flag.String("igg", "", "Pattern for line to ignore when scanning file.")
	verbose       = flag.Bool("v", false, "Make some output more verbose.")
	interactive   = flag.Bool("i", false, "Interactive scanning.")
	errorOnly     = flag.Bool("e", false, "Make output error only.")
)

func report(filesCnt uint64, errorCnt uint64, containKorean []file.Source) {
	if !(*errorOnly) {
		for _, f := range containKorean {
			fmt.Println(f.Path())
			f.PrintFoundLines()
		}
	}
	fmt.Printf("[%d] scanning files\n", filesCnt)
	fmt.Printf("[%d] error \n", errorCnt)
	fmt.Printf("[%d] success \n", filesCnt-errorCnt)
	fmt.Printf("[%d] files containing korean\n", len(containKorean))
}

func shouldScan(foundFilesCnt uint64) bool {
	var response string
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("found files [%d]. do you want to scan it? (y/n): ", foundFilesCnt)
	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	response = strings.Trim(response, " \n")
	if response != "y" && response != "n" {
		return shouldScan(foundFilesCnt)
	}
	if response == "n" {
		return false
	}
	return true
}

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if (*dirToSearch) == "" || (*dirToSearch) == conf.DefaultDir {
		currentDir, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		(*dirToSearch) = currentDir
	}

	err := dir.Check(dirToSearch)
	if err != nil {
		log.Fatal(err)
	}

	if (*skipPaths) != conf.DefaultSkipPaths {
		(*skipPaths) += "," + conf.DefaultSkipPaths
	}

	skipPathRegex, err := dir.MakeSkipPathRegex(skipPaths)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("search for files [*.%s] in [%s] directory\n", (*fileExtToScan), (*dirToSearch))
	foundFiles, err := dir.Search((*dirToSearch), (*fileExtToScan), skipPathRegex)
	if err != nil {
		log.Fatal(err)
	}

	foundFilesCnt := uint64(len(foundFiles))
	if foundFilesCnt == 0 {
		fmt.Printf("[*.%s] file not found in [%s] directory\n", (*fileExtToScan), (*dirToSearch))
		os.Exit(0)
	}

	if *interactive {
		if !shouldScan(foundFilesCnt) {
			os.Exit(0)
		}
	}

	matchRegex, ignoreRegex, err := file.MakeRegexForScan(conf.RegexStrToKorean, *ignorePattern)
	if err != nil {
		log.Fatal(err)
	}

	containKorean := []file.Source{}
	var scanErrorCnt uint64
	for _, paths := range file.Chunks(foundFiles) {
		for source := range file.ScanFiles(paths, *verbose, matchRegex, ignoreRegex) {
			if err := source.Error(); err != nil {
				scanErrorCnt++
				if *verbose || *errorOnly {
					fmt.Printf("[%s] scanning error - %s\n", source.Path(), err)
				}
				continue
			}

			if len(*source.FoundLines()) > 0 {
				containKorean = append(containKorean, (*source))
			}
		}
	}

	sort.Slice(containKorean, func(i, j int) bool {
		return containKorean[i].Path() < containKorean[j].Path()
	})

	report(foundFilesCnt, scanErrorCnt, containKorean)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
