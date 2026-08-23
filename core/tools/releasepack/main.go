// Command releasepack creates deterministic DavDeck release archives.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", "", "directory to archive")
	output := flag.String("output", "", "archive output path")
	format := flag.String("format", "", "tar.gz or zip")
	epoch := flag.Int64("epoch", 0, "normalized Unix timestamp")
	flag.Parse()
	if *root == "" || *output == "" || (*format != "tar.gz" && *format != "zip") || *epoch < 0 {
		flag.Usage()
		os.Exit(2)
	}
	stamp := time.Unix(*epoch, 0).UTC()
	var err error
	if *format == "tar.gz" {
		err = writeTarGzip(*root, *output, stamp)
	} else {
		err = writeZip(*root, *output, stamp)
	}
	if err == nil {
		err = writeChecksum(*output)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "releasepack:", err)
		os.Exit(1)
	}
}

func entries(root string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root {
			result = append(result, path)
		}
		return nil
	})
	sort.Slice(result, func(i, j int) bool {
		left, _ := filepath.Rel(filepath.Dir(root), result[i])
		right, _ := filepath.Rel(filepath.Dir(root), result[j])
		return filepath.ToSlash(left) < filepath.ToSlash(right)
	})
	return result, err
}

func normalizedHeader(root, path string, stamp time.Time) (*tar.Header, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return nil, err
		}
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(filepath.Dir(root), path)
	if err != nil {
		return nil, err
	}
	header.Name = filepath.ToSlash(relative)
	if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
		header.Name += "/"
	}
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	header.ModTime, header.AccessTime, header.ChangeTime = stamp, time.Time{}, time.Time{}
	return header, nil
}

func writeTarGzip(root, output string, stamp time.Time) (returnErr error) {
	paths, err := entries(root)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); returnErr == nil {
			returnErr = err
		}
	}()
	gzipWriter, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return err
	}
	gzipWriter.Header.ModTime, gzipWriter.Header.OS = stamp, 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, path := range paths {
		header, err := normalizedHeader(root, path, stamp)
		if err != nil {
			return err
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg {
			if err := copyFile(tarWriter, path); err != nil {
				return err
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func writeZip(root, output string, stamp time.Time) (returnErr error) {
	paths, err := entries(root)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); returnErr == nil {
			returnErr = err
		}
	}()
	writer := zip.NewWriter(file)
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(root), path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name, header.Modified, header.Method = filepath.ToSlash(relative), stamp, zip.Deflate
		if info.IsDir() {
			if !strings.HasSuffix(header.Name, "/") {
				header.Name += "/"
			}
			header.Method = zip.Store
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(entry, link); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(entry, path); err != nil {
				return err
			}
		}
	}
	return writer.Close()
}

func copyFile(writer io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func writeChecksum(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	body := hex.EncodeToString(hash.Sum(nil)) + "  " + filepath.Base(path) + "\n"
	return os.WriteFile(path+".sha256", []byte(body), 0o644)
}
