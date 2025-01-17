package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alephao/cacheutil"
	"github.com/mholt/archiver"
)

const (
	BITRISE_GIT_BRANCH       = "BITRISE_GIT_BRANCH"
	BITRISE_OSX_STACK_REV_ID = "BITRISE_OSX_STACK_REV_ID"

	CACHE_AWS_ACCESS_KEY_ID     = "cache_aws_access_key_id"
	CACHE_AWS_SECRET_ACCESS_KEY = "cache_aws_secret_access_key"
	CACHE_AWS_ENDPOINT          = "cache_aws_endpoint"
	CACHE_AWS_REGION            = "cache_aws_region"
	CACHE_BUCKET_NAME           = "cache_bucket_name"
	CACHE_RESTORE_KEYS          = "cache_restore_keys"
	CACHE_PATH                  = "cache_path"
	CACHE_ARCHIVE_EXTENSION     = "cache_archive_extension"
)

func parseRestoreKeysInput(keysString string) []string {
	var keys []string
	for _, keyString := range strings.Split(
		strings.TrimSpace(
			keysString,
		),
		"\n",
	) {
		keys = append(keys, strings.TrimSpace(keyString))
	}
	return keys
}

func parseRestoreKeys(restoreKeys string) ([]string, error) {
	branch := os.Getenv(BITRISE_GIT_BRANCH)
	stackrev := os.Getenv(BITRISE_OSX_STACK_REV_ID)
	functionExecuter := cacheutil.NewCacheKeyFunctionExecuter(branch, stackrev)
	keyParser := cacheutil.NewKeyParser(&functionExecuter)

	var keys []string
	for _, keyTemplate := range parseRestoreKeysInput(restoreKeys) {
		key, err := keyParser.Parse(keyTemplate)

		if err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	return keys, nil
}

func main() {
	awsAccessKeyId := GetEnvOrExit(CACHE_AWS_ACCESS_KEY_ID)
	awsSecretAccessKey := GetEnvOrExit(CACHE_AWS_SECRET_ACCESS_KEY)
	awsEndpoint := os.Getenv(CACHE_AWS_ENDPOINT)
	awsRegion := GetEnvOrExit(CACHE_AWS_REGION)
	bucketName := GetEnvOrExit(CACHE_BUCKET_NAME)
	restoreKeys := GetEnvOrExit(CACHE_RESTORE_KEYS)
	cachePath := GetEnvOrExit(CACHE_PATH)
	archiveExtension := GetEnvOrExit(CACHE_ARCHIVE_EXTENSION)

	CreateTempFolder(func(tempFolderPath string) {
		s3 := NewAwsS3(
			awsEndpoint,
			awsRegion,
			awsAccessKeyId,
			awsSecretAccessKey,
			bucketName,
		)

		keys, err := parseRestoreKeys(restoreKeys)

		if err != nil {
			log.Fatalf("failed to parse keys\nerror: %s", err.Error())
		}

		for _, key := range keys {
			log.Printf("Checking if cache exists for key '%s'\n", key)
			cacheExists, cacheKey := s3.CacheExists(key)
			if cacheExists {
				log.Println("Cache found! Downloading...")
				downloadedFilePath := fmt.Sprintf("%s/%s.%s", tempFolderPath, cacheKey, archiveExtension)
				size, err := s3.Download(cacheKey, downloadedFilePath)

				if err != nil {
					log.Printf("Download failed with error: %s. Cancelling cache restore.\n", err.Error())
					return
				}

				log.Printf("Download was successful, file size: %d. Uncompressing...\n", size)

				err = archiver.Unarchive(downloadedFilePath, cachePath)

				if err != nil {
					log.Printf("Failed to uncompress: %s. Cancelling cache restore.\n", err.Error())
					return
				}
				return
			}
		}
		log.Println("Cache not found.")
	})

	os.Exit(0)
}
