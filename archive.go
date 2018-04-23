package main

import (
	"archive/tar"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/klauspost/pgzip"
)

// Archive is a tar.gz file (or will be) built from a path to a folder
type Archive struct {
	name, path, info, md5 string
}

// NewArchive builds an archive from a folder
func NewArchive(path string, force bool) (*Archive, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	folderInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s.tar.gz", folderInfo.Name())

	a := &Archive{
		name: name,
		path: path,
		md5:  "",
	}

	// If the archive file exists, do not recreate unless force == true
	if _, err := os.Stat(a.name); (err != nil) || force {
		if err := createArchive(a.name, a.path); err != nil {
			return nil, err
		}
	} else {
		info("Local archive %q already exists", a.name)
	}

	// Calculate the MD5 of the file
	a.md5, err = calculateMD5(a.name)
	if err != nil {
		// TODO: better error message
		return nil, err
	}

	return a, nil
}

func calculateMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return hash, nil
}

func addFiles(tw *tar.Writer, path string) error {
	baseDir := filepath.Base(path)
	walkFn := func(p string, finfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		var link string
		if finfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(finfo, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(baseDir, strings.TrimPrefix(p, path)))
		if err = tw.WriteHeader(header); err != nil {
			return err
		}
		if !finfo.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(p)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err = io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	}
	if err := filepath.Walk(path, walkFn); err != nil {
		return err
	}
	return nil
}

func createArchive(name string, path string) error {

	info("Creating archive %q...", name)
	archive, err := os.Create(fmt.Sprintf(name))
	if err != nil {
		return err
	}
	defer archive.Close()
	gw := pgzip.NewWriter(archive)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	if err := addFiles(tw, path); err != nil {
		return err
	}
	info("Created archive.")
	return nil
}

// Upload uploads an archive to the S3 bucket provided if it doesn't already exist
func (a Archive) Upload(bucket string, svc *s3.S3, sess *session.Session) error {

	info("Checking remote archive...")
	ra, err := NewRemoteArchive(a.name, bucket, svc)
	if err != nil {
		return err
	}

	if ra.exists {
		if ra.md5 == a.md5 {
			info("Remote archive %q already exists; not uploading", ra.key)
			return nil
		}
		info("Remote archive has mismatched MD5 (local: %s, remote: %s)", a.md5, ra.md5)
	}

	if err = a.upload(bucket, sess); err != nil {
		return err
	}

	return nil
}

func (a Archive) upload(bucket string, sess *session.Session) error {
	info("Uploading archive %q to %q...", a.name, bucket)
	uploader := s3manager.NewUploader(sess)
	file, err := os.Open(a.name)
	if err != nil {
		return err
	}
	defer file.Close()
	metadata := make(map[string]string)
	metadata["md5chksum"] = a.md5
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(a.name),
		ContentMD5: aws.String(a.md5),
		Metadata:   aws.StringMap(metadata),
		Body:       file,
	})
	if err != nil {
		return err
	}
	info("Upload finished.")
	return nil
}

// Delete removes the local copy of the archive
func (a Archive) Delete() error {
	if err := os.Remove(a.name); err != nil {
		return err
	}
	info("Deleted local copy of %q", a.name)
	return nil
}
