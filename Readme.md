# S3Sync: uploading folders to S3

`s3sync` is a program that will archive and upload a given folder to an S3-compatible service (by default, [Wasabi](https://wasabisys.com)). 

**Features**:
* Parallel upload of large files thanks to AWS Go SDK
* MD5 hashing of local and remote files to prevent duplicate uploads
* Completely dependency-free and cross-platform!

## Installation
First, store your AWS-type credentials in a file with the following structure in `~/.aws/credentials` (or any other valid configuration as shown [here](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)):

```
[default]
aws_access_key_id = XXXX
aws_secret_access_key = XXXX
```

Next, download a binary from our release page. The most up-to-date binaries are always available in the [latest](https://github.com/eclarke/s3sync/releases/tag/latest) release. Alternatively, download them from the command line, for example: 

```sh
wget https://github.com/eclarke/s3sync/releases/download/latest/s3sync_linux_amd64 
```

## Usage

### Uploading a folder:

To upload a folder, you’d run
```sh
./s3sync ul -bucket my_bucket -folder /path/to/folder
```

S3Sync will then create an archive in the working directory called `folder.tar.gz` (make sure you have enough space on your hard drive for this!) and upload it to S3. 

If you re-run this command, you will find that nothing happens: it doesn’t recreate the archive and it doesn’t re-upload as the local copy and the remote copy have the same fingerprint. If you’ve changed something in the folder, specify `-force` to recreate the archive. This will trigger a re-upload since the fingerprints will no longer match. If you specify `-clean`, the local archive will be deleted upon successful upload.

### Downloading an archive

To download an existing archive, run:
```sh
./s3sync dl -bucket my_bucket -archive folder.tar.gz
```

This downloads the file to your working directory.

### Listing archives on remote

To see all the files on your remote bucket, run:
```sh
./s3sync list -bucket my_bucket
``` 
