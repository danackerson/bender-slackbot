package commands

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
	"github.com/otium/ytdl"
)

var dropboxAccessToken = os.Getenv("CTX_DROPBOX_ACCESS_TOKEN")
var chunkSize = int64(1 << 27) // ~138 MB

func uploadInternetFileToDropbox(downloadFromURL string, uploadToPath string,
	config dropbox.Config) (tempPublicURL string, err error) {
	dbx := files.New(config)

	// https://www.dropbox.com/developers/documentation/http/documentation
	// https://github.com/dropbox/dropbox-sdk-go-unofficial/blob/master/dropbox/files/types.go

	res, err := http.Head(downloadFromURL)
	if err != nil {
		return downloadFromURL, err
	} else if res.ContentLength <= 0 {
		return downloadFromURL, errors.New("<= cowardly refusing to transfer empty file")
	}

	log.Printf("File size: %s bytes\n", strconv.FormatInt(res.ContentLength, 10))

	resp, err := http.Get(downloadFromURL)
	if err != nil {
		log.Printf("ERR: %s\n", err.Error())
	}
	defer resp.Body.Close()

	err = nil
	// if video is > 1<<27 ()
	if res.ContentLength > chunkSize {
		commitInfo := files.NewCommitInfo(uploadToPath)
		err = uploadChunked(dbx, resp.Body, commitInfo, res.ContentLength)
	} else if res.ContentLength > 0 {
		commitInfo := files.NewCommitInfo(uploadToPath)
		_, err = dbx.Upload(commitInfo, resp.Body)
	}

	if err == nil {
		filesMetaData, err := dbx.GetTemporaryLink(
			files.NewGetTemporaryLinkArg(uploadToPath))
		return filesMetaData.Link, err
	}

	return downloadFromURL, err
}

// https://github.com/mschneider82/sharecmd/blob/master/provider/dropbox/dropbox.go
func uploadChunked(dbx files.Client, r io.Reader, commitInfo *files.CommitInfo, sizeTotal int64) (err error) {
	res, err := dbx.UploadSessionStart(files.NewUploadSessionStartArg(),
		&io.LimitedReader{R: r, N: chunkSize})
	if err != nil {
		return err
	}

	written := chunkSize

	for (sizeTotal - written) > chunkSize {
		cursor := files.NewUploadSessionCursor(res.SessionId, uint64(written))
		args := files.NewUploadSessionAppendArg(cursor)

		err = dbx.UploadSessionAppendV2(args, &io.LimitedReader{R: r, N: chunkSize})
		if err != nil {
			return err
		}
		written += chunkSize
	}

	cursor := files.NewUploadSessionCursor(res.SessionId, uint64(written))
	args := files.NewUploadSessionFinishArg(cursor, commitInfo)

	if _, err = dbx.UploadSessionFinish(args, r); err != nil {
		return err
	}

	return nil
}

func downloadYoutubeVideo(origURL string) bool {
	downloaded := false

	config := dropbox.Config{
		Token:    dropboxAccessToken,
		LogLevel: dropbox.LogInfo, // if needed, set the desired logging level. Default is off
	}

	vid, err := ytdl.GetVideoInfo(origURL)
	if err == nil {
		URI, err := vid.GetDownloadURL(vid.Formats[0])
		if err == nil {
			log.Printf("preparing to download: %s\n", URI.String())

			uploadToPath := "/youtube/" + vid.Title + "." + vid.Formats[0].Extension
			tempPublicURL, err := uploadInternetFileToDropbox(URI.String(), uploadToPath, config)
			if err != nil {
				log.Printf("%s %s\n", tempPublicURL, err.Error())
			} else {
				log.Printf("Uploaded %s\n", tempPublicURL)
				tempPublicURL = strings.Replace(tempPublicURL, "dl=0", "dl=1", 1)
				icon := "https://emoji.slack-edge.com/T092UA8PR/youtube/a9a89483b7536f8a.png"
				smallIcon := "http://icons.iconarchive.com/icons/iconsmind/outline/16/Youtube-icon.png"

				sendPayloadToJoinAPI(tempPublicURL, vid.Title, icon, smallIcon)
				downloaded = true
			}
		} else {
			log.Printf("ERR: %s\n", err.Error())
		}
	} else {
		log.Printf("ERR: %s\n", err.Error())
	}

	return downloaded
}