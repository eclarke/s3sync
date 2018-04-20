package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/dustin/go-humanize"

	"github.com/fatih/color"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {

	if runtime.GOOS == "windows" {
		color.NoColor = true
	}

	uploadCmd := flag.NewFlagSet("ul", flag.ExitOnError)
	downloadCmd := flag.NewFlagSet("dl", flag.ExitOnError)
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)

	bucketPtr := listCmd.String("bucket", "", "bucket name")
	endpointPtr := listCmd.String("endpoint", "https://s3.wasabisys.com", "service endpoint url")
	regionPtr := listCmd.String("region", "us-east-1", "s3 region")

	folderPtr := uploadCmd.String("folder", "", "folder to upload")
	remakeArchive := uploadCmd.Bool("force", false, "recreate archive if exists")
	clean := uploadCmd.Bool("clean", false, "delete local archive upon successful upload")
	uploadCmd.StringVar(bucketPtr, "bucket", "", "bucket name")
	uploadCmd.StringVar(endpointPtr, "endpoint", "https://s3.wasabisys.com", "service endpoint url")
	uploadCmd.StringVar(regionPtr, "region", "us-east-1", "s3 region")

	archivePtr := downloadCmd.String("archive", "", "archive to retrieve")
	downloadCmd.StringVar(bucketPtr, "bucket", "", "bucket name")
	downloadCmd.StringVar(endpointPtr, "endpoint", "https://s3.wasabisys.com", "service endpoint url")
	downloadCmd.StringVar(regionPtr, "region", "us-east-1", "s3 region")

	flag.Parse()

	if len(os.Args) < 2 {
		flag.PrintDefaults()
		fatal("Must specify upload ('ul'), download ('dl'), or 'list'")
	}

	initS3 := func() (bucket string, sess *session.Session, svc *s3.S3) {
		if *bucketPtr == "" {
			fatal("Must specify bucket name")
		}

		bucket = *bucketPtr

		sess, err := session.NewSession(&aws.Config{
			Region:   aws.String(*regionPtr),
			Endpoint: aws.String(*endpointPtr)},
		)
		if err != nil {
			fatal("Could not connect to S3. (%v)", err)
		}

		svc = s3.New(sess)
		return
	}

	switch os.Args[1] {
	case "ul":
		uploadCmd.Parse(os.Args[2:])
		bucket, sess, svc := initS3()
		archive, err := NewArchive(*folderPtr, *remakeArchive)
		if err != nil {
			fatal("Could not create archive %q. (%v)", *folderPtr, err)
		}

		if err = archive.Upload(bucket, svc, sess); err != nil {
			fatal("Could not upload archive %q. (%v)", archive.name, err)
		}

		if *clean {
			if err = archive.Delete(); err != nil {
				fatal("Could not delete archive; try removing it manually. (%v)", err)
			}
		}
	case "dl":
		downloadCmd.Parse(os.Args[2:])
		bucket, sess, svc := initS3()
		remoteArchive, err := NewRemoteArchive(*archivePtr, bucket, svc)
		if err != nil {
			fatal("Unable to retrieve remote archive %q. (%v)", *archivePtr, err)
		}
		if err = remoteArchive.Download(sess); err != nil {
			fatal("Unable to download remote archive. (%v)", err)
		}
	case "list":
		listCmd.Parse(os.Args[2:])
		bucket, _, svc := initS3()
		resp, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: aws.String(bucket)})
		if err != nil {
			fatal("Could not list items in bucket %q. (%v)", bucket, err)
		}
		blue := color.New(color.FgBlue).SprintfFunc()
		cyan := color.New(color.FgCyan).SprintfFunc()
		for _, item := range resp.Contents {
			modTime := (*item.LastModified).Format("Jan 2")
			size := cyan(humanize.Bytes(uint64(*item.Size)))
			fmt.Printf("%s\t%v\t%s\n", size, modTime, blue(*item.Key))
		}

	}
}

func info(format string, v ...interface{}) {
	green := color.New(color.FgGreen).SprintFunc()
	log.Printf("["+green("INFO")+"] "+format+"\n", v...)
}

func fatal(format string, v ...interface{}) {
	red := color.New(color.FgRed).SprintFunc()
	log.Fatalf("["+red("ERROR")+"] "+format+"\n", v...)
}
