package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var path string
var dest string
var copy bool
var verbose bool
var justdups bool
var quiet bool

type Flist map[string]struct {
	orig, dedup string
}

func checksum(pathname string) (string, error) {
	infile, inerr := os.Open(pathname)
	if inerr != nil {
		return "", inerr
	}
	md5h := md5.New()
	io.Copy(md5h, infile)
	infile.Close()

	csum := md5h.Sum(nil)
	return fmt.Sprintf("%x", csum), nil // hex
}

func fcopy(src, dst string) (int64, time.Duration, error) {
	start:= time.Now()
        sourceFileStat, err := os.Stat(src)
        if err != nil {
                return 0, time.Since(start),  err
        }

        if !sourceFileStat.Mode().IsRegular() {
                return 0, time.Since(start), fmt.Errorf("%s is not a regular file", src)
        }

        source, err := os.Open(src)
        if err != nil {
                return 0, time.Since(start), err
        }
        defer source.Close()

        destination, err := os.Create(dst)
        if err != nil {
                return 0, time.Since(start), err
        }
        defer destination.Close()

        nBytes, err := io.Copy(destination, source)
        return nBytes, time.Since(start), err
}

func isFile(f os.FileInfo) bool {
	return f.Mode() & os.ModeType == 0 && os.ModeSymlink != 0 // regular file
}

func match(f string) bool {
	regex := []string{
		"*.[jJ][pP][gG]",
		"*.[mM][pP][eE][gG]",
		"*.[mM][pP]4",
		"*.[Gg][Ii][Ff]",
	}

	f = filepath.Base(f)
	for _, r := range regex {
		matched, err := filepath.Match(r, f)
		if err != nil {
			fmt.Println(err)
		}
		if matched {
			return true
		}
	}
	return false
}

func newFlist() *Flist {
	sl := make(Flist)
	return &sl
}

func (s *Flist) dumpdup() {
	for k, v := range (*s) {
		if v.dedup  != "" {
			fmt.Printf(" D: %s: %s -> %s\n", k, v.orig, v.dedup)
		}
	}
}	
func (s *Flist) dump() {
	for k, v := range (*s) {
		if v.dedup != "" {
			fmt.Printf("D - %s: %s -> %s\n", k, v.orig, v.dedup)
		} else {
			fmt.Printf("U - %s: %s\n", k, v.orig)
		}
	}
}

func (s *Flist) add (c, f string) {
	v, dup := (*s)[c]
	if !dup {
		v.orig = f
		(*s)[c] = v
	}
}

func (s *Flist) dedup(dest string) {
	var cnt uint64
	nm := make(map[string]bool)
	
	for k, v := range *s {
		base := filepath.Base(v.orig)
		if _, exists := nm[base]; exists {
			ext := filepath.Ext(base)
			v.dedup = fmt.Sprintf("%s/%s-%d%s", dest, base, cnt, ext)
			(*s)[k] = v
			cnt++
		} 
		nm[base] = true
	}
}

func (s Flist) copy() {
	var files, errors uint64
	var cc int64
	var tt time.Duration
	
	for _, v := range s {
		if v.dedup == "" {
			continue
		}

		fmt.Printf("Copying %s -> %s ... ", v.orig, v.dedup)
		c, t, err := fcopy(v.orig, v.dedup)
		if err != nil {
			errors++
			if !quiet {
				fmt.Printf("\n\tERROR: %s\n", err)
			}
		} else {
			// stats
			fmt.Printf("%d bytes in %f seconds\n", c, t.Seconds())
			files++
			tt += t
			cc += c
		}
	}
	fmt.Printf("Copy complete, %d files, %d bytes in  %s", files, cc, tt)
	if errors > 0 {
		fmt.Printf(", %d errors", errors)
	}
	fmt.Println()
}

// read the directory tree, generate csums and sort.  May be a goroutine 
func (s *Flist) processPath(pathname string) {
	walkfn := func(pathname string, f os.FileInfo, err error) error {
/*		x := len(pathname)
		if x > 78 {
			x = 78
		}
		fmt.Printf("%s\r", pathname[:x])
*/
		n := fmt.Sprintf("%-78v\r", pathname)
		fmt.Printf("%s\r", n[:78])
		if err != nil {
			if !quiet {
				fmt.Printf("Error: %s, path %s\n", err, pathname)
			}
			return err
		}

		pathname = filepath.Clean(pathname)
		if isFile(f) && match(pathname) {
			csum, err := checksum(pathname)
			if err == nil {
				s.add(csum, pathname)
			}
		}
		return nil
	}

	pathname = filepath.Clean(pathname)
	fmt.Printf("Processing %s:\n", pathname)
	start := time.Now()
	filepath.Walk(pathname, walkfn)
	fmt.Printf("                                                                               \r")
	fmt.Printf("Done in %s\n", time.Since(start))
}

func main() {

	
	flag.StringVar(&path, "path", "", "Path to start from")
	flag.StringVar(&dest, "dest", "", "Destination path")
	flag.BoolVar(&copy, "copy", false, "Copy files to dest")
	flag.BoolVar(&verbose, "verbose", false, "Verbose")
	flag.BoolVar(&justdups, "d", false, "Just show dups in Verbose")
	flag.BoolVar(&quiet, "q", false, "Suppress errors")
	flag.Parse()

	if path == "" {
		fmt.Println("ERROR: No starting path")
		os.Exit(1)
	}
	if dest == "" && copy {
		fmt.Println("ERROR: Copy flag set, no destination path")
		os.Exit(1)
	}
	if dest != "" && !copy {
		fmt.Println("ERROR: Destination path set, copy not requested")
		os.Exit(1)
	}
	
	sl := newFlist()
	sl.processPath(path)
	sl.dedup(dest)
	if verbose {
		if justdups {
			sl.dumpdup()
		} else{
			sl.dump()
		}
	}
	if copy {
		sl.copy()
	}
}
