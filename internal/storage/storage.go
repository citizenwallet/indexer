package storage

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

// GetUserHomeDir returns the user's home directory on linux, windows or macOS
// taken from: https://gist.github.com/miguelmota/f30a04a6d64bd52d7ab59ea8d95e54da
func GetUserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	} else if runtime.GOOS == "linux" {
		home := os.Getenv("XDG_CONFIG_HOME")
		if home != "" {
			return home
		}
	}
	return os.Getenv("HOME")
}

// Exists checks if the given file or folder for a path exists
func Exists(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)
	if err != nil || os.IsNotExist(err) {
		return false
	}

	return true
}

// Save saves data to a file
func Save(name string, data []byte) error {
	exists := Exists(name)
	if exists {
		err := EraseFile(name)
		if err != nil {
			return err
		}
	}
	return ioutil.WriteFile(name, data, os.FileMode(0644))
}

// Update updates data in a file
func Update(name string, data []byte) error {
	return ioutil.WriteFile(name, data, os.FileMode(0644))
}

// Read reads data from a file
func Read(name string) ([]byte, error) {
	return ioutil.ReadFile(name)
}

// CreateDir creates a directory
func CreateDir(dir string) error {
	return os.MkdirAll(dir, os.FileMode(0755))
}

// EraseDir erases the contents of a directory
// taken from: https://stackoverflow.com/questions/33450980/how-to-remove-all-contents-of-a-directory-using-golang
func EraseDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

// EraseFile erases the file
func EraseFile(file string) error {
	return os.Remove(file)
}
