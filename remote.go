package main

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type RemoteArchive struct {
	key, md5, bucket string
	svc              *s3.S3
	exists           bool
}

func NewRemoteArchive(key string, bucket string, svc *s3.S3) (*RemoteArchive, error) {
	ra := &RemoteArchive{
		key:    key,
		md5:    "",
		bucket: bucket,
		svc:    svc,
		exists: false,
	}

	out, err := svc.HeadObject(&s3.HeadObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(bucket),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				info("No remote archive found with that name")
			default:
				return nil, err
			}
		}
	} else {
		ra.exists = true
		if md5, hasMd5 := out.Metadata["Md5chksum"]; hasMd5 {
			ra.md5 = *md5
		}
	}

	return ra, nil
}

func (ra RemoteArchive) Download(sess *session.Session) error {
	if !ra.exists {
		return errors.New("Remote archive does not exist")
	}
	file, err := os.Create(ra.key)
	if err != nil {
		return err
	}
	defer file.Close()

	downloader := s3manager.NewDownloader(sess)
	numBytes, err := downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(ra.bucket),
		Key:    aws.String(ra.key),
	})
	if err != nil {
		return err
	}
	info("Downloaded %q (%d bytes)", ra.key, numBytes)
	return nil
}

func (ra RemoteArchive) Md5Hex() (hexstr string, err error) {
	hash, err := base64.StdEncoding.DecodeString(ra.md5)
	if err != nil {
		return "", err
	}
	hexstr = hex.EncodeToString(hash)
	return
}

func (ra RemoteArchive) Fingerprint() (string, error) {
	hash, err := ra.Md5Hex()
	if err != nil {
		return "", err
	}
	rhash := []rune(hash)
	fingerprintLen := 7
	if len(rhash) < fingerprintLen {
		fingerprintLen = len(rhash)
	}
	return string(rhash[0:fingerprintLen]), nil
}
