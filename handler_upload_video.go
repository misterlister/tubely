package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	r.Body = http.MaxBytesReader(w, r.Body, MaxVideoMemory)
	if err := r.ParseMultipartForm(MaxVideoMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	videoData, err := cfg.db.GetVideo(videoID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "video not found", err)
			return
		}
		respondWithError(w, http.StatusInternalServerError, "database error", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unable to edit other user's videos", nil)
		return
	}

	fileType, _, err := mime.ParseMediaType(mediaType)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid Content-Type", err)
		return
	}

	fileExtension, ok := typeMap[fileType]

	if !ok || !slices.Contains(ValidVideoTypes, fileExtension) {
		respondWithError(w, http.StatusUnsupportedMediaType, "unsupported Content-Type: "+fileType, nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")

	defer os.Remove(tempFile.Name())

	defer tempFile.Close()

	if _, err = io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to write to file", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "aspect ratio error", err)
		return
	}

	ratioPrefix := getAspectRatioPrefix(aspectRatio)

	processedFilepath, err := processVideoForFastStart(tempFile.Name())

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to preprocess file", err)
		return
	}

	processedFile, err := os.Open(processedFilepath)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to open preprocessed file", err)
		return
	}

	defer os.Remove(processedFile.Name())

	defer processedFile.Close()

	randomBytes := make([]byte, FilenameByteSize)
	if _, err = rand.Read(randomBytes); err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to generate filename", err)
		return
	}

	hexKey := hex.EncodeToString(randomBytes)

	key := fmt.Sprintf("%s%s.%s", ratioPrefix, hexKey, fileExtension)

	input := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        processedFile,
		ContentType: &fileType,
	}

	if _, err := cfg.s3Client.PutObject(r.Context(), &input); err != nil {
		respondWithError(w, http.StatusInternalServerError, "s3 upload failed", err)
		return
	}

	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	videoData.VideoURL = &videoURL

	if err = cfg.db.UpdateVideo(videoData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update video", err)
		return
	}

	signedVideo, err := cfg.dbVideoToSignedVideo(videoData)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "presign failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedVideo)
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	sections := strings.Split(*video.VideoURL, ",")
	if len(sections) < 2 {
		return video, errors.New("improperly formatted video url: " + *video.VideoURL)
	}
	bucket := sections[0]
	key := sections[1]

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Minute*2)

	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedURL
	return video, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	params := s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	req, err := presignClient.PresignGetObject(context.Background(), &params, s3.WithPresignExpires(expireTime))

	if err != nil {
		return "", err
	}

	return req.URL, nil
}
