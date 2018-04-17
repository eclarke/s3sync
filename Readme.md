# S3Sync: uploading folders to S3

`s3sync` is a program that will archive and upload a given folder to an S3-compatible service (by default, [Wasabi](https://wasabisys.com)). 

**Features**:
* Parallel upload of large files thanks to AWS Go SDK
* MD5 hashing of local and remote files to prevent duplicate uploads
* Completely dependency-free and cross-platform!

## Installation
First, store your AWS-type credentials in a file with the following structure in `~/.aws/credentials` (or any other valid configuration as shown [here](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials):

```
[default]
aws_access_key_id = XXXX
aws_secret_access_key = XXXX
```

### Installing from binaries
Retrieve the binaries from GitHub (replace `linux64` with `mac64` or `windows64` as appropriate):
```sh
wget https://github.com/eclarke/s3sync/releases/download/v0.1.0/s3sync-linux64 
```

And you’re done!

## Usage
```sh
$ ./s3sync-linux64 -h
Usage of ./s3sync-linux64:
  -bucket string
        bucket name
  -endpoint string
        service endpoint url (default "https://s3.wasabisys.com")
  -folder string
        folder to upload
  -force
        recreate archive if exists
  -makeBucket
        create bucket
  -region string
        s3 region (default "us-east-1")
```

To upload a folder to Wasabi, you’d run
```sh
./s3sync-linux64 -bucket <my_bucket> -folder /path/to/folder
```

S3Sync will then create an archive in the working directory called `folder.tar.gz` (make sure you have enough space on your hard drive for this!) and upload it to S3. 

If you re-run this command, you will find that nothing happens: it doesn’t recreate the archive and it doesn’t re-upload as the local copy and the remote copy have the same fingerprint. If you’ve changed something in the folder, specify `-force` to recreate the archive. This will trigger a re-upload since the fingerprints will no longer match.