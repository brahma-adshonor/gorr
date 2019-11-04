package util

import (
	"io"
	"os"
	"os/exec"
)

func RunCmd(cmd string) ([]byte, error) {
	sh := "/bin/sh"
	out, err := exec.Command(sh, "-c", cmd).Output()
	if err != nil {
		return out, err
	}

	return out, nil
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}
