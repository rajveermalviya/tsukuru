package main

import (
	"io"
	"os"
)

func contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func cp(src, dst string) error {
	srcf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcf.Close()

	srcInfo, err := srcf.Stat()
	if err != nil {
		return err
	}

	dstf, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstf.Close()

	_, err = io.Copy(dstf, srcf)
	if err != nil {
		return err
	}

	return nil
}
