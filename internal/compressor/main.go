package compressor

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func MkdirAll(path string) error {
	// Create the directory and all necessary parents.
	// os.MkdirAll is used to ensure that the entire path is created.
	return os.MkdirAll(path, os.ModePerm)
}

// ZipDir zips the contents of srcDir into destZip (including all subdirectories).
// srcDir should be the path to the directory you want to compress,
// and destZip should be the path where you want to create the zip file.
// example usage:
// err := ZipDir("path/to/source/directory", "path/to/destination/archive.zip")
func ZipDir(srcDir, destZip string) error {
	// Ensure the source directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return os.ErrNotExist
	}
	// Create the destination zip file
	if err := MkdirAll(filepath.Dir(destZip)); err != nil {
		return err
	}
	zipfile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			_, err := archive.Create(relPath + "/")
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		f, err := archive.Create(relPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, file)
		return err
	})
	return err
}

// Unzip extracts a zip archive to the specified destination directory.
// srcZip should be the path to the zip file you want to extract,
// and destDir should be the path where you want to extract the contents.
// example usage:
// err := Unzip("path/to/archive.zip", "path/to/destination/directory")
func Unzip(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func ZipExists(zipLoc string) error {
	// Check if the zip file exists
	if _, err := os.Stat(zipLoc); os.IsNotExist(err) {
		return os.ErrNotExist
	}
	// Check if the file is a valid zip file
	file, err := os.Open(zipLoc)
	if err != nil {
		return err
	}
	defer file.Close()

	// Try to read the first few bytes to check if it's a zip file
	header := make([]byte, 2)
	if _, err := file.Read(header); err != nil {
		return err
	}
	if header[0] != 'P' || header[1] != 'K' {
		return os.ErrInvalid
	}
	return nil
}
