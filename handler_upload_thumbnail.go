package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read file data", err)
		return
	}

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

	if !ok {
		respondWithError(w, http.StatusUnsupportedMediaType, "unsupported Content-Type: "+fileType, nil)
		return
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to generate filename", err)
		return
	}

	filename := base64.RawURLEncoding.EncodeToString(randomBytes)

	pathname := filepath.Join(cfg.assetsRoot, filename+"."+fileExtension)

	f, err := os.Create(pathname)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to create file", err)
		return
	}

	defer f.Close()

	_, err = io.Copy(f, file)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to write to file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, filename, fileExtension)
	videoData.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(videoData)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "database error", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
