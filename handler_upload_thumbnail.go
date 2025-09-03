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
	"slices"

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

	r.Body = http.MaxBytesReader(w, r.Body, MaxImageMemory)
	if err := r.ParseMultipartForm(MaxImageMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")

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

	if !ok || !slices.Contains(ValidImageTypes, fileExtension) {
		respondWithError(w, http.StatusUnsupportedMediaType, "unsupported Content-Type: "+fileType, nil)
		return
	}

	randomBytes := make([]byte, FilenameByteSize)
	if _, err = rand.Read(randomBytes); err != nil {
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

	if _, err = io.Copy(f, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to write to file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, filename, fileExtension)
	videoData.ThumbnailURL = &thumbnailURL

	if err = cfg.db.UpdateVideo(videoData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "database error", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
