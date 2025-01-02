package main

import (
	"fmt"
	"net/http"
  "io"

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

  mediaType := header.Header["Content-Type"][0]

  var vidData []byte
  vidData, err = io.ReadAll(file)
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Unable to read image data into slice", err)
    return
  }

  video, err := cfg.db.GetVideo(videoID)
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Unable to get videos from db", err)
    return
  }

  if video.UserID != userID {
    respondWithError(w, http.StatusUnauthorized, "Current user is not the author of the videos", nil)
    return
  }

  thmb := thumbnail{
    data:      vidData,
    mediaType: mediaType,
  }

  videoThumbnails[video.ID] = thmb

  newThumbnail := fmt.Sprintf("http://localhost:%d/api/thumbnails/%d", cfg.port, video.ID)
  video.ThumbnailURL = &newThumbnail

  err = cfg.db.UpdateVideo(video)
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Cannot update video", err)
    return
  }

	respondWithJSON(w, http.StatusOK, video)
}
