package main

import (
	"crypto/md5"
	"encoding/base64"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func main() {
	if len(os.Args) != 2 {
		fatal("filename required to upload")
	}

	filename := os.Args[1]
	bucket := "ares-minion-testbucket"

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// calculate the MD5sum of the file, then seek back to the beginning
	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		log.Fatal(err)
	}
	file.Seek(0, 0)

	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))

	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: aws.String("https://s3.wasabisys.com")},
	)

	svc := s3.New(sess)

	if !objectExists(filename, hash, bucket, svc) {
		info("Uploading object %q...", filename)
		uploader := s3manager.NewUploader(sess)

		metadata := make(map[string]string)
		metadata["md5chksum"] = hash

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(filename),
			ContentMD5: aws.String(hash),
			Metadata:   aws.StringMap(metadata),
			Body:       file,
		})
		if err != nil {
			fatal("Unable to upload %q to %q, %v", filename, bucket, err)
		}

		info("Successfully uploaded %q to %q", filename, bucket)

	}

}

func objectExists(key string, hash string, bucket string, svc *s3.S3) bool {
	// First see if it exists, and if so whether we can grab the md5 checksum
	out, err := svc.HeadObject(&s3.HeadObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(bucket),
	})
	if err != nil {
		info("No corresponding object found")
		return false
	}
	md5sum, hasMd5 := out.Metadata["Md5chksum"]

	if hasMd5 && (*md5sum == hash) {
		info("Found object and MD5 matched")
		return true
	} else if hasMd5 {
		info("Found object, but MD5 mismatched")
		return false
	} else {
		info("Found object, but no MD5 stored")
		return false
	}

}

func info(format string, v ...interface{}) {
	log.Printf("[ INFO] "+format+"\n", v...)
}

func fatal(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format+"\n", v...)
}
