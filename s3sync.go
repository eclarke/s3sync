package main

import (
	"archive/tar"
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/klauspost/pgzip"

	"github.com/fatih/color"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func addFile(tw *tar.Writer, path string, info os.FileInfo) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	// Alter the name in the FileInfo to take the full path so that
	// the tar file maintains the directory structure
	header, err := tar.FileInfoHeader(info, path)
	header.Name = path
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := io.Copy(tw, file); err != nil {
		return err
	}
	return nil
}

func addFilesToArchive(tw *tar.Writer, path string) error {
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if err := addFile(tw, p, info); err != nil {
				return err
			}
		}
		return nil
	})
	return nil
}

func createArchive(name string, path string) error {

	info("Creating archive %q", name)
	archive, err := os.Create(fmt.Sprintf(name))
	if err != nil {
		return err
	}
	defer archive.Close()
	gw := pgzip.NewWriter(archive)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	if err := addFilesToArchive(tw, path); err != nil {
		return err
	}
	return nil
}

func uploadArchive(name string, bucket string, md5 string, sess *session.Session) error {
	info("Uploading archive %q to %q...", name, bucket)
	uploader := s3manager.NewUploader(sess)
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	metadata := make(map[string]string)
	metadata["md5chksum"] = md5
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(name),
		ContentMD5: aws.String(md5),
		Metadata:   aws.StringMap(metadata),
		Body:       file,
	})
	if err != nil {
		return err
	}
	return nil
}

func calculateFileMD5(path string) (string, error) {
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

func objectExists(key string, hash string, bucket string, svc *s3.S3) bool {
	// First see if it exists, and if so whether we can grab the md5 checksum
	out, err := svc.HeadObject(&s3.HeadObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				info("Remote: object not found")
			default:
				fatal("Error querying remote endpoint:\n\t %v", err)
			}
		}

		return false
	}
	md5sum, hasMd5 := out.Metadata["Md5chksum"]

	if hasMd5 && (*md5sum == hash) {
		info("Remote: file exists")
		return true
	} else if hasMd5 {
		info("Remote: found, but mismatched MD5 (local: %s, remote: %s)", hash, *md5sum)
		return false
	} else {
		info("Remote: found, but no MD5 stored")
		return false
	}

}

func resolveFolder(path string) (abspath string, name string, err error) {
	abspath, err = filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abspath)
	if err != nil {
		return "", "", err
	}
	name = info.Name()
	return
}

func main() {

	if runtime.GOOS == "windows" {
		color.NoColor = true
	}

	bucketPtr := flag.String("bucket", "", "bucket name")
	folderPtr := flag.String("folder", "", "folder to upload")
	endpointPtr := flag.String("endpoint", "https://s3.wasabisys.com", "service endpoint url")
	regionPtr := flag.String("region", "us-east-1", "s3 region")
	remakeArchive := flag.Bool("force", false, "recreate archive if exists")
	clean := flag.Bool("clean", false, "delete local archive upon successful upload")

	flag.Parse()

	if *folderPtr == "" {
		fatal("Must specify folder to upload")
	}

	if *bucketPtr == "" {
		fatal("Must specify bucket name")
	}

	bucket := *bucketPtr

	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(*regionPtr),
		Endpoint: aws.String(*endpointPtr)},
	)
	if err != nil {
		fatal("Could not connect to S3. (%v)", err)
	}

	svc := s3.New(sess)

	// Create archive
	folder, name, err := resolveFolder(*folderPtr)
	if err != nil {
		fatal("Could not resolve target folder %q. (%v)", err)
	}

	info("Target folder: %s", folder)

	archiveName := fmt.Sprintf("%s.tar.gz", name)
	if _, err := os.Stat(archiveName); (err == nil) && !*remakeArchive {
		info("Archive %q already exists; not recreating", archiveName)
	} else {
		if err = createArchive(archiveName, folder); err != nil {
			fatal("Could not create archive. (%v)", err)
		}
	}

	// Create MD5 of archive
	info("Checking if %q exists on remote...", archiveName)

	archiveHash, err := calculateFileMD5(archiveName)
	if err != nil {
		fatal("Could not calculate MD5sum of archive. (%v)", err)
	}

	// Upload archive if doesn't exist/mismatched
	if !objectExists(archiveName, archiveHash, bucket, svc) {
		if err = uploadArchive(archiveName, bucket, archiveHash, sess); err != nil {
			fatal("Unable to upload %q. (%v)", archiveName, bucket, err)
		}
	} else {
		info("Nothing to be done.")
	}

	if *clean {
		if err = os.Remove(archiveName); err != nil {
			fatal("Unable to delete %q; maybe try manually? (%v)", archiveName, err)
		}
		info("Deleted local copy of %q", archiveName)
	}

	info("Finished.")

}

func info(format string, v ...interface{}) {
	green := color.New(color.FgGreen).SprintFunc()
	log.Printf("["+green("INFO")+"] "+format+"\n", v...)
}

func fatal(format string, v ...interface{}) {
	red := color.New(color.FgRed).SprintFunc()
	log.Fatalf("["+red("ERROR")+"] "+format+"\n", v...)
}
