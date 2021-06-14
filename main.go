package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alephao/bitrise-step-s3-cache-pull/parser"
	"github.com/mholt/archiver"
)

const (
	BITRISE_GIT_BRANCH       = "BITRISE_GIT_BRANCH"
	BITRISE_OSX_STACK_REV_ID = "BITRISE_OSX_STACK_REV_ID"
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
	functionExecuter := parser.NewCacheKeyFunctionExecuter(branch, stackrev)
	keyParser := parser.NewKeyParser(&functionExecuter)

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
	awsAccessKeyId := GetEnvOrExit("aws_access_key_id")
	awsSecretAccessKey := GetEnvOrExit("aws_secret_access_key")
	awsRegion := GetEnvOrExit("aws_region")
	bucketName := GetEnvOrExit("bucket_name")
	restoreKeys := GetEnvOrExit("restore_keys")
	cachePath := GetEnvOrExit("path")

	CreateTempFolder(func(tempFolderPath string) {
		s3 := NewAwsS3(
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
				downloadedFilePath := fmt.Sprintf("%s/%s.zip", tempFolderPath, cacheKey)
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
